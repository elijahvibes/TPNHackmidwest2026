package travel

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// Ranker is the slice of ai.Client that place ranking needs. A nil Ranker, or
// one that is not Enabled, falls back to a deterministic ordering.
type Ranker interface {
	Enabled() bool
	ChatJSON(ctx context.Context, system, user string, out any) error
}

var overpassEndpoints = []string{
	"https://overpass-api.de/api/interpreter",
	"https://overpass.kumi.systems/api/interpreter",
}

// osmSelectors maps a place kind to Overpass tag filters.
var osmSelectors = map[string][]string{
	"food":       {`["amenity"="restaurant"]`},
	"breakfast":  {`["amenity"="cafe"]`, `["shop"="bakery"]`},
	"attraction": {`["tourism"~"^(attraction|museum|zoo|aquarium|gallery|theme_park|viewpoint)$"]`, `["leisure"="park"]`},
	"rest_stop":  {`["amenity"="fuel"]`, `["highway"~"^(rest_area|services)$"]`},
	"outdoors":   {`["leisure"~"^(park|nature_reserve)$"]`, `["boundary"="national_park"]`},
	"museums":    {`["tourism"~"^(museum|gallery)$"]`},
	"nightlife":  {`["amenity"~"^(bar|pub|nightclub)$"]`},
	"family":     {`["tourism"~"^(zoo|aquarium|theme_park)$"]`},
	"history":    {`["historic"~"^(monument|memorial|castle|fort|ruins|archaeological_site)$"]`, `["tourism"="museum"]["museum"="history"]`},
}

// osmPriceUSD gives a rough per-person cost. OpenStreetMap carries no prices,
// so these are estimates and the plan flags them as such.
var osmPriceUSD = map[string]float64{"restaurant": 22, "cafe": 10, "bakery": 8, "bar": 15, "pub": 15, "nightclub": 20}

type osmElement struct {
	Lat    float64                     `json:"lat"`
	Lon    float64                     `json:"lon"`
	Center *struct{ Lat, Lon float64 } `json:"center"`
	Tags   map[string]string           `json:"tags"`
}

type osmCacheEntry struct {
	at     time.Time
	places []Place
}

var (
	osmMu    sync.Mutex
	osmCache = map[string]osmCacheEntry{}
)

// EnableOSM switches the finder to real data only: Google when a key is set,
// otherwise OpenStreetMap. The mock pool is never used again. An optional
// ranker (an AI client) picks and describes the best candidates, but it can
// only choose places OpenStreetMap returned.
func (p *Places) EnableOSM(r Ranker) *Places {
	p.osm = true
	p.ranker = r
	return p
}

func (p *Places) openStreetMap(ctx context.Context, at LatLng, kind string, limit int) []Place {
	sel, ok := osmSelectors[kind]
	if !ok {
		sel = osmSelectors["attraction"]
	}
	key := fmt.Sprintf("%s|%.2f|%.2f|%d", kind, at.Lat, at.Lng, limit)
	osmMu.Lock()
	if e, hit := osmCache[key]; hit && time.Since(e.at) < time.Hour {
		osmMu.Unlock()
		return e.places
	}
	osmMu.Unlock()

	var q strings.Builder
	q.WriteString("[out:json][timeout:20];(")
	for _, s := range sel {
		for _, el := range []string{"node", "way"} {
			fmt.Fprintf(&q, `%s%s["name"](around:15000,%f,%f);`, el, s, at.Lat, at.Lng)
		}
	}
	q.WriteString(");out center 150;")

	var elems []osmElement
	for _, ep := range overpassEndpoints {
		var out struct {
			Elements []osmElement `json:"elements"`
		}
		req, err := http.NewRequestWithContext(ctx, "POST", ep, strings.NewReader(url.Values{"data": {q.String()}}.Encode()))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("User-Agent", "TrailMix-TPN/1.0 (HackMidwest 2026)")
		res, err := p.http.Do(req)
		if err != nil {
			continue
		}
		err = json.NewDecoder(res.Body).Decode(&out)
		res.Body.Close()
		if res.StatusCode == 200 && err == nil {
			elems = out.Elements
			break
		}
	}

	cands := make([]Place, 0, len(elems))
	seen := map[string]bool{}
	for _, e := range elems {
		name := strings.TrimSpace(e.Tags["name"])
		lat, lng := e.Lat, e.Lon
		if e.Center != nil {
			lat, lng = e.Center.Lat, e.Center.Lon
		}
		if name == "" || (lat == 0 && lng == 0) || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		kindTag := firstNonEmpty(e.Tags["amenity"], e.Tags["tourism"], e.Tags["leisure"], e.Tags["historic"])
		price, priced := osmPriceUSD[kindTag]
		cands = append(cands, Place{
			Name: name, Category: kind, PriceUSD: price, PriceUnknown: !priced && kind != "rest_stop",
			Location: LatLng{Lat: lat, Lng: lng}, Note: osmNote(e.Tags), Source: "osm",
		})
	}
	// Better-documented places first (cuisine, hours, website), then nearer ones.
	score := func(pl Place) float64 { return float64(len(pl.Note)) - haversineKm(at, pl.Location)*2 }
	sort.SliceStable(cands, func(i, j int) bool { return score(cands[i]) > score(cands[j]) })

	out := cands
	if len(out) > limit {
		out = out[:limit]
	}
	if p.ranker != nil && p.ranker.Enabled() && len(cands) > limit {
		if ranked := p.rankWithAI(ctx, kind, cands, limit); len(ranked) > 0 {
			out = ranked
		}
	}

	if len(out) > 0 {
		osmMu.Lock()
		osmCache[key] = osmCacheEntry{at: time.Now(), places: out}
		osmMu.Unlock()
	}
	return out
}

// rankWithAI shows the model a numbered list of real places and asks for the
// best indices plus a one-line note drawn only from the listed facts. Names,
// locations and prices stay as OpenStreetMap returned them.
func (p *Places) rankWithAI(ctx context.Context, kind string, cands []Place, limit int) []Place {
	if len(cands) > 30 {
		cands = cands[:30]
	}
	var list strings.Builder
	for i, c := range cands {
		fmt.Fprintf(&list, "%d. %s", i, c.Name)
		if c.Note != "" {
			fmt.Fprintf(&list, " (%s)", c.Note)
		}
		list.WriteByte('\n')
	}
	sys := `You choose from a numbered list of real places. Reply with JSON only: {"picks":[{"index":0,"note":"..."}]}.
Pick a varied, appealing set. Each note is at most 12 words and uses only facts in the list. If the list gives no facts, leave note empty. Never add places that are not listed.`
	user := fmt.Sprintf("Choose the best %d %s places for a visitor.\n%s", limit, kind, list.String())
	var out struct {
		Picks []struct {
			Index int    `json:"index"`
			Note  string `json:"note"`
		} `json:"picks"`
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if p.ranker.ChatJSON(cctx, sys, user, &out) != nil {
		return nil
	}
	used := map[int]bool{}
	res := make([]Place, 0, limit)
	for _, pk := range out.Picks {
		if pk.Index < 0 || pk.Index >= len(cands) || used[pk.Index] || len(res) == limit {
			continue
		}
		used[pk.Index] = true
		pl := cands[pk.Index]
		if n := strings.TrimSpace(pk.Note); n != "" {
			pl.Note = n
		}
		res = append(res, pl)
	}
	return res
}

func osmNote(t map[string]string) string {
	var parts []string
	if c := t["cuisine"]; c != "" {
		parts = append(parts, strings.ReplaceAll(strings.ReplaceAll(c, ";", ", "), "_", " "))
	}
	if t["outdoor_seating"] == "yes" {
		parts = append(parts, "outdoor seating")
	}
	if t["diet:vegetarian"] == "yes" || t["diet:vegan"] == "yes" {
		parts = append(parts, "vegetarian options")
	}
	if h := t["opening_hours"]; h != "" && len(h) <= 40 {
		parts = append(parts, "hours "+h)
	}
	return strings.Join(parts, " · ")
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

func haversineKm(a, b LatLng) float64 {
	const r = 6371.0
	rad := math.Pi / 180
	dLat, dLng := (b.Lat-a.Lat)*rad, (b.Lng-a.Lng)*rad
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(a.Lat*rad)*math.Cos(b.Lat*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * r * math.Asin(math.Sqrt(h))
}
