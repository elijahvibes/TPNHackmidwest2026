// Package agents holds the small, single-purpose AI agents the planner runs
// on the Akamai inference service before and after the itinerary is written:
//
//	Curator    — filters bad or irrelevant places out of the real candidate lists
//	Researcher — gathers destination knowledge (timing, neighborhoods, pitfalls)
//	Reviewer   — audits the written itinerary against the real data
//
// Every agent degrades gracefully: with no model, or a failed call, the caller
// gets the untouched input and a Report that says so. No agent may invent a
// place or a price; they only choose, describe and check.
package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"trailmix/internal/travel"
)

// Chat is the slice of ai.Client the agents need.
type Chat interface {
	Enabled() bool
	ChatJSON(ctx context.Context, system, user string, out any) error
}

// Trip is the shared context every agent sees.
type Trip struct {
	Destination    string   `json:"destination"`
	StartDate      string   `json:"start_date"`
	EndDate        string   `json:"end_date"`
	Travelers      int      `json:"travelers"`
	Interests      []string `json:"interests"`
	LocalTransport string   `json:"local_transport"`
	RadiusMiles    float64  `json:"activity_radius_miles"`
	Weather        string   `json:"weather"` // one-line summary
	SafetyLevel    string   `json:"safety_level"`
	Center         travel.LatLng
}

// Report is what each agent did, for the UI's "how this plan was made" panel.
type Report struct {
	Agent   string   `json:"agent"`
	Status  string   `json:"status"` // ok | skipped | failed
	Summary string   `json:"summary"`
	Notes   []string `json:"notes,omitempty"`
}

// callTimeout fits the workshop endpoint's ~17 tokens/s with a few hundred
// tokens of output plus queueing.
const callTimeout = 90 * time.Second

// ---------------------------------------------------------------- Curator

// Curate asks the model to pick the best `want` places from real candidates
// and drop the junk: gas stations listed as attractions, duplicate chains,
// places that don't fit the interests, anything beyond reach with the
// traveler's local transport. It can only return places that were given to it.
func Curate(ctx context.Context, c Chat, trip Trip, kind string, cands []travel.Place, want int) ([]travel.Place, Report) {
	rep := Report{Agent: "curator:" + kind}
	cands = dedupe(cands)
	if len(cands) <= want || c == nil || !c.Enabled() {
		rep.Status, rep.Summary = "skipped", fmt.Sprintf("%d %s candidates, nothing to filter", len(cands), kind)
		if len(cands) > want {
			cands = cands[:want]
		}
		return cands, rep
	}
	if len(cands) > 40 {
		cands = cands[:40]
	}
	var list strings.Builder
	for i, p := range cands {
		fmt.Fprintf(&list, "%d. %s", i, p.Name)
		if len(p.Categories) > 0 {
			fmt.Fprintf(&list, " [%s]", strings.Join(p.Categories[:min(3, len(p.Categories))], ", "))
		}
		if p.Rating > 0 {
			fmt.Fprintf(&list, " ★%.1f", p.Rating)
		}
		if d := milesBetween(trip.Center, p.Location); d > 0 {
			fmt.Fprintf(&list, " %.1f mi", d)
		}
		if p.Note != "" {
			fmt.Fprintf(&list, " (%s)", p.Note)
		}
		list.WriteByte('\n')
	}
	system := `You are the Curator agent for a trip planner. You receive a numbered list of REAL places from a map provider and must choose the ones worth a traveler's time. Reply with JSON only:
{"keep":[{"index":int,"note":string,"price_usd":number}],"dropped":[{"index":int,"reason":string}]}
Rules: keep at most the requested number, best first. DROP anything that is not a genuine destination of the requested kind (gas stations, parking lots, offices, schools, cemeteries, generic chain stores, unnamed or administrative entries), near-duplicates and multiple branches of one chain, places that clash with the traveler's interests or group, and places unrealistic to reach with their local transport. Each note is at most 12 words and uses only facts shown in the list; if there are no facts, use an empty string; notes never mention prices. price_usd is your best estimate of the typical per-person cost (adult admission, or an average meal with drink), 0 for free places like parks; when the place name tells you nothing, estimate from its category. Never invent a place or an index that is not listed.`
	tripJSON, _ := json.Marshal(trip)
	user := fmt.Sprintf("Trip: %s\nKind: %s\nKeep the best %d.\n\n%s", tripJSON, kind, want, list.String())

	var out struct {
		Keep []struct {
			Index    int     `json:"index"`
			Note     string  `json:"note"`
			PriceUSD float64 `json:"price_usd"`
		} `json:"keep"`
		Dropped []struct {
			Index  int    `json:"index"`
			Reason string `json:"reason"`
		} `json:"dropped"`
	}
	cctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	if err := c.ChatJSON(cctx, system, user, &out); err != nil {
		log.Printf("agents: curator(%s): %v", kind, err)
		rep.Status, rep.Summary = "failed", "curator unavailable; using provider order"
		return cands[:want], rep
	}
	used := map[int]bool{}
	kept := make([]travel.Place, 0, want)
	for _, k := range out.Keep {
		if k.Index < 0 || k.Index >= len(cands) || used[k.Index] || len(kept) == want {
			continue
		}
		used[k.Index] = true
		p := cands[k.Index]
		if n := strings.TrimSpace(k.Note); n != "" && len(n) <= 90 {
			p.Note = n
		}
		// A provider price always wins; the model's estimate only fills gaps
		// (and refines the category guess) and stays marked as an estimate.
		if (p.PriceUnknown || p.PriceEstimated) && k.PriceUSD >= 0 && k.PriceUSD <= 400 {
			p.PriceUSD, p.PriceUnknown, p.PriceEstimated = k.PriceUSD, false, true
		}
		kept = append(kept, p)
	}
	if len(kept) == 0 {
		rep.Status, rep.Summary = "failed", "curator kept nothing; using provider order"
		return cands[:want], rep
	}
	for _, d := range out.Dropped {
		if d.Index >= 0 && d.Index < len(cands) && !used[d.Index] && len(rep.Notes) < 6 {
			rep.Notes = append(rep.Notes, fmt.Sprintf("dropped %s: %s", cands[d.Index].Name, strings.TrimSpace(d.Reason)))
		}
	}
	rep.Status = "ok"
	rep.Summary = fmt.Sprintf("kept %d of %d real %s places", len(kept), len(cands), kind)
	return kept, rep
}

// CurateAll runs one Curator per list concurrently (the workshop endpoint
// allows 60 requests a minute, so three in flight is fine).
func CurateAll(ctx context.Context, c Chat, trip Trip, lists map[string][]travel.Place, want map[string]int) (map[string][]travel.Place, []Report) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	out := make(map[string][]travel.Place, len(lists))
	var reps []Report
	for kind, cands := range lists {
		wg.Add(1)
		go func(kind string, cands []travel.Place) {
			defer wg.Done()
			kept, rep := Curate(ctx, c, trip, kind, cands, want[kind])
			mu.Lock()
			out[kind] = kept
			reps = append(reps, rep)
			mu.Unlock()
		}(kind, cands)
	}
	wg.Wait()
	sort.Slice(reps, func(i, j int) bool { return reps[i].Agent < reps[j].Agent })
	return out, reps
}

// ------------------------------------------------------------- Researcher

// Research is what the Researcher agent knows about the destination for
// these dates. It is general model knowledge, so the UI labels it that way;
// it never carries prices or opening hours.
type Research struct {
	Overview      string   `json:"overview"`
	Neighborhoods []string `json:"neighborhoods,omitempty"`
	Tips          []string `json:"tips,omitempty"`
	Avoid         []string `json:"avoid,omitempty"`
	Seasonal      string   `json:"seasonal,omitempty"`
	PlaceNotes    []struct {
		Name string `json:"name"`
		Note string `json:"note"`
	} `json:"place_notes,omitempty"`
	Source string `json:"source"`
}

// DoResearch asks the model what a well-travelled friend would say about the
// destination on these dates, grounded in the real places already found.
func DoResearch(ctx context.Context, c Chat, trip Trip, places []travel.Place) (*Research, Report) {
	rep := Report{Agent: "researcher"}
	if c == nil || !c.Enabled() {
		rep.Status, rep.Summary = "skipped", "no model configured"
		return nil, rep
	}
	names := make([]string, 0, len(places))
	for _, p := range places {
		names = append(names, p.Name)
	}
	system := `You are the Researcher agent for a trip planner. Write what a well-travelled local friend would tell this traveler about the destination for these exact dates. Reply with JSON only:
{"overview":string (2-3 sentences), "neighborhoods":[string] (up to 4, each "Name — why go"), "tips":[string] (3-6 practical tips: best times of day, parking/transit habits, reservations, what to bring), "avoid":[string] (up to 3 pitfalls: tourist traps, closures typical for the season, crowded times), "seasonal":string (what these dates mean: weather pattern, events, daylight), "place_notes":[{"name":string,"note":string}] (only for places in the given list you actually know; at most 6; one useful sentence each)}
Rules: never state prices, admission fees or opening hours. Never invent a place. Only use names from the given list in place_notes. Be specific and concrete; skip filler. If you are unsure about something, leave it out.`
	tripJSON, _ := json.Marshal(trip)
	user := fmt.Sprintf("Trip: %s\nReal places found: %s", tripJSON, strings.Join(names, "; "))
	var out Research
	cctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	if err := c.ChatJSON(cctx, system, user, &out); err != nil {
		log.Printf("agents: researcher: %v", err)
		rep.Status, rep.Summary = "failed", "research unavailable"
		return nil, rep
	}
	out.Overview = strings.TrimSpace(out.Overview)
	if out.Overview == "" && len(out.Tips) == 0 {
		rep.Status, rep.Summary = "failed", "research came back empty"
		return nil, rep
	}
	// Only keep notes about places we were actually given.
	known := map[string]bool{}
	for _, n := range names {
		known[strings.ToLower(strings.TrimSpace(n))] = true
	}
	kept := out.PlaceNotes[:0]
	for _, pn := range out.PlaceNotes {
		if known[strings.ToLower(strings.TrimSpace(pn.Name))] && strings.TrimSpace(pn.Note) != "" {
			kept = append(kept, pn)
		}
	}
	out.PlaceNotes = kept
	out.Tips = clip(out.Tips, 6)
	out.Avoid = clip(out.Avoid, 3)
	out.Neighborhoods = clip(out.Neighborhoods, 4)
	out.Source = "akamai-ai (model knowledge; verify locally)"
	rep.Status = "ok"
	rep.Summary = fmt.Sprintf("%d tips, %d place notes for %s", len(out.Tips), len(out.PlaceNotes), trip.Destination)
	return &out, rep
}

// --------------------------------------------------------------- Reviewer

// Item is the slice of an itinerary line the Reviewer needs.
type Item struct {
	Day      int
	Title    string
	Category string
	CostUSD  float64
}

// Known is the ground truth an itinerary must stay inside.
type Known struct {
	Places    []travel.Place // activities, food, stops the plan was built from
	Travelers int
}

// Audit checks every activity/food/stop line against the real place list and
// its estimated price. It returns the indices of items that name a place that
// does not exist in the data, plus the indices whose cost should be clamped
// (and to what). It is deterministic: the model never gets to overrule it.
func Audit(items []Item, known Known) (drop map[int]string, clamp map[int]float64) {
	drop, clamp = map[int]string{}, map[int]float64{}
	trav := float64(max(1, known.Travelers))
	for i, it := range items {
		if it.Category != "activity" && it.Category != "food" && it.Category != "stop" {
			continue
		}
		p, ok := matchPlace(it.Title, known.Places)
		if !ok {
			drop[i] = fmt.Sprintf("day %d: %q is not in the real place data", it.Day, it.Title)
			continue
		}
		// A place with an estimated price must not be billed above it for the group.
		if p.PriceUSD > 0 && it.CostUSD > p.PriceUSD*trav*1.25+1 {
			clamp[i] = round2(p.PriceUSD * trav)
		}
		if p.PriceUSD == 0 && it.CostUSD > 0 && !p.PriceUnknown {
			clamp[i] = 0 // free place billed as paid
		}
	}
	return drop, clamp
}

// Critique asks the model to read the finished itinerary once more as a
// skeptical traveler and list concrete problems (an outdoor day in the rain,
// dinner scheduled after the flight home, two museums back to back). The notes
// are shown to the user; nothing is silently changed.
func Critique(ctx context.Context, c Chat, trip Trip, itinerary any) Report {
	rep := Report{Agent: "reviewer"}
	if c == nil || !c.Enabled() {
		rep.Status, rep.Summary = "skipped", "no model configured"
		return rep
	}
	system := `You are the Reviewer agent for a trip planner. Read the itinerary as a skeptical, experienced traveler and list concrete problems only. Reply with JSON only: {"verdict":"good|fixable|poor","issues":[string]} with at most 5 issues, each one sentence naming the day and the item. Look for: outdoor plans on rainy days, two far-apart places scheduled an hour apart, repeated places, a day with nothing to do, safety advice ignored when safety_level is "elevated", and anything that contradicts the weather or the traveler's interests.
Already guaranteed by the system, so NOT issues: arrival-day items are scheduled after the computed arrival time; dinner is in the evening; last-day items end before the trip home with airport lead time (a quiet morning before a flight is fine); short gaps between check-in and the next item; opening hours and transit availability are unverified by design (the plan says so); prices. The weather feed only covers the next 7 days, so when the trip is further out the forecast dates will not match the trip dates: treat it as a current outlook, not an error. Do not report any of those. If nothing else is wrong, verdict is "good" with an empty issues list.`
	tripJSON, _ := json.Marshal(trip)
	itinJSON, _ := json.Marshal(itinerary)
	var out struct {
		Verdict string   `json:"verdict"`
		Issues  []string `json:"issues"`
	}
	cctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	if err := c.ChatJSON(cctx, system, fmt.Sprintf("Trip: %s\nItinerary: %s", tripJSON, itinJSON), &out); err != nil {
		log.Printf("agents: reviewer: %v", err)
		rep.Status, rep.Summary = "failed", "review unavailable"
		return rep
	}
	rep.Status = "ok"
	rep.Notes = clip(out.Issues, 5)
	if out.Verdict == "" {
		out.Verdict = "good"
	}
	rep.Summary = fmt.Sprintf("verdict: %s, %d issue(s)", out.Verdict, len(rep.Notes))
	return rep
}

// ---------------------------------------------------------------- helpers

// matchPlace finds the real place an itinerary title refers to. Models write
// "Dinner at Joe's Diner" or "Joe's Diner (outdoor seating)", so a title
// matches when it contains the place name or the name contains the title.
func matchPlace(title string, places []travel.Place) (travel.Place, bool) {
	t := norm(title)
	best, bestLen := travel.Place{}, 0
	for _, p := range places {
		n := norm(p.Name)
		if n == "" || len(n) < 3 {
			continue
		}
		if strings.Contains(t, n) || strings.Contains(n, t) {
			if len(n) > bestLen {
				best, bestLen = p, len(n)
			}
		}
	}
	return best, bestLen > 0
}

func norm(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == ' ':
			b.WriteRune(r)
		case r == '&':
			b.WriteString(" and ")
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func dedupe(in []travel.Place) []travel.Place {
	seen := map[string]bool{}
	out := make([]travel.Place, 0, len(in))
	for _, p := range in {
		k := norm(p.Name)
		if len(k) < 3 || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, p)
	}
	return out
}

func clip(s []string, n int) []string {
	out := make([]string, 0, min(n, len(s)))
	for _, v := range s {
		if v = strings.TrimSpace(v); v != "" && len(out) < n {
			out = append(out, v)
		}
	}
	return out
}

func milesBetween(a, b travel.LatLng) float64 {
	if (a.Lat == 0 && a.Lng == 0) || (b.Lat == 0 && b.Lng == 0) {
		return 0
	}
	const r = 3958.8
	rad := math.Pi / 180
	dLat, dLng := (b.Lat-a.Lat)*rad, (b.Lng-a.Lng)*rad
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(a.Lat*rad)*math.Cos(b.Lat*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * r * math.Asin(math.Sqrt(h))
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
