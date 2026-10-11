// Package planner turns a trip request into a fully priced itinerary.
// It gathers prices from every travel provider, scores the drive-vs-fly
// options for value, and asks the AI (Akamai-hosted LLM) to write the
// day-by-day plan around the real numbers.
package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"trailmix/internal/agents"
	"trailmix/internal/ai"
	"trailmix/internal/travel"
)

type Request struct {
	Origin              string         `json:"origin"`
	Destination         string         `json:"destination"`
	StartDate           string         `json:"start_date"` // YYYY-MM-DD
	EndDate             string         `json:"end_date"`
	Travelers           int            `json:"travelers"`
	BudgetUSD           float64        `json:"budget_usd"`
	Mode                string         `json:"mode"` // auto | drive | fly
	VehicleMPG          float64        `json:"vehicle_mpg"`
	TankGallons         float64        `json:"tank_gallons"` // 0 = typical 14 gal
	Interests           []string       `json:"interests"`
	OriginLoc           *travel.LatLng `json:"origin_loc,omitempty"` // from browser geolocation or the autocomplete pick
	DestLoc             *travel.LatLng `json:"dest_loc,omitempty"`   // from the autocomplete pick
	LocalTransport      string         `json:"local_transport"`
	ActivityRadiusMiles float64        `json:"activity_radius_miles"`
	Progress            Progress       `json:"-"` // optional milestone callback (see Event)
}

type Option struct {
	Mode          string   `json:"mode"`
	TransportUSD  float64  `json:"transport_usd"`
	LodgingUSD    float64  `json:"lodging_usd"`
	ActivitiesUSD float64  `json:"activities_usd"`
	LocalUSD      float64  `json:"local_usd"` // getting around at the destination (estimate)
	TotalUSD      float64  `json:"total_usd"`
	TravelHours   float64  `json:"travel_hours"`
	ValueScore    float64  `json:"value_score"` // higher is better
	Why           string   `json:"why"`
	Factors       []string `json:"factors"` // human-readable breakdown of the score
}

type DayItem struct {
	Time        string  `json:"time"`
	Title       string  `json:"title"`
	Detail      string  `json:"detail,omitempty"`
	CostUSD     float64 `json:"cost_usd"`
	CostUnknown bool    `json:"cost_unknown,omitempty"`
	Category    string  `json:"category"`
}

type Day struct {
	Day   int       `json:"day"`
	Date  string    `json:"date"`
	Theme string    `json:"theme"`
	Items []DayItem `json:"items"`
}

type Plan struct {
	Title               string               `json:"title"`
	Summary             string               `json:"summary"`
	RecommendedMode     string               `json:"recommended_mode"`
	Options             []Option             `json:"options"`
	Drive               travel.DriveEstimate `json:"drive"`
	Flights             []travel.FlightQuote `json:"flights"`
	Hotels              []travel.HotelQuote  `json:"hotels"`
	Stops               []travel.Place       `json:"stops"`
	Activities          []travel.Place       `json:"activities"`
	Food                []travel.Place       `json:"food"`
	Days                []Day                `json:"days"`
	TotalUSD            float64              `json:"total_usd"`
	UnderBudget         bool                 `json:"under_budget"`
	Nights              int                  `json:"nights"`
	OriginLoc           travel.LatLng        `json:"origin_loc"`
	DestLoc             travel.LatLng        `json:"dest_loc"`
	RouteLine           []travel.LatLng      `json:"route_line,omitempty"` // driven road from origin to destination (OSRM), for the map
	OriginPlace         travel.Placemark     `json:"origin_place"`             // what the origin resolved to ("Lawrence, KS")
	DestPlace           travel.Placemark     `json:"dest_place"`               // what the destination resolved to
	OriginAirport       *travel.Airport      `json:"origin_airport,omitempty"` // nearest airport used for fares
	DestAirport         *travel.Airport      `json:"dest_airport,omitempty"`
	Weather             travel.Weather       `json:"weather"`
	Safety              travel.Safety        `json:"safety"`
	AIGenerated         bool                 `json:"ai_generated"`
	scheduleNotes       []string             // what normalizeSchedule changed in the AI's draft, for the audit report
	Sources             map[string]string    `json:"sources"`
	GeneratedAt         time.Time            `json:"generated_at"`
	Travelers           int                  `json:"travelers"`
	BudgetUSD           float64              `json:"budget_usd"`
	SelectedFlight      int                  `json:"selected_flight"`
	SelectedReturn      *travel.FlightQuote  `json:"selected_return,omitempty"`
	FlightSelected      bool                 `json:"flight_selected"`
	OriginName          string               `json:"origin_name"`
	DestinationName     string               `json:"destination_name"`
	LocalTransport      string               `json:"local_transport"`
	ActivityRadiusMiles float64              `json:"activity_radius_miles"`
	Interests           []string             `json:"interests,omitempty"`
	Research            *agents.Research     `json:"research,omitempty"` // Researcher agent's destination notes
	Agents              []agents.Report      `json:"agents,omitempty"`   // what each AI agent did to this plan
}

type TravelSearch interface {
	Enabled() bool
	Flights(context.Context, string, string, string, string, int) ([]travel.FlightQuote, error)
	Hotels(context.Context, string, string, string, int, int) ([]travel.HotelQuote, error)
	ReturnFlights(context.Context, travel.FlightSearch) ([]travel.FlightQuote, error)
	BookingOptions(context.Context, travel.FlightSearch) ([]travel.BookingOption, error)
}

type Planner struct {
	AI      *ai.Client
	Geo     *travel.Geo
	Fuel    *travel.Fuel
	SerpAPI TravelSearch
	Places  *travel.Places
	Weather *travel.NWS  // nil = mock forecast (tests)
	Wiki    *travel.Wiki // nil = no popularity signal (tests)
}

// review runs the Reviewer agent over the written itinerary: a deterministic
// audit that removes lines naming places not in the real data and clamps
// costs above the known estimate, then a model pass that lists remaining
// problems for the traveler to see.
func (p *Planner) review(ctx context.Context, trip agents.Trip, plan *Plan) []agents.Report {
	known := agents.Known{Travelers: plan.Travelers}
	known.Places = append(known.Places, plan.Activities...)
	known.Places = append(known.Places, plan.Food...)
	known.Places = append(known.Places, plan.Stops...)
	var items []agents.Item
	type pos struct{ d, i int }
	var where []pos
	for d := range plan.Days {
		for i, it := range plan.Days[d].Items {
			items = append(items, agents.Item{Day: plan.Days[d].Day, Title: it.Title, Category: it.Category, CostUSD: it.CostUSD})
			where = append(where, pos{d, i})
		}
	}
	drop, clamp := agents.Audit(items, known)
	audit := agents.Report{Agent: "reviewer:audit", Status: "ok"}
	for idx, to := range clamp {
		w := where[idx]
		it := &plan.Days[w.d].Items[w.i]
		audit.Notes = append(audit.Notes, fmt.Sprintf("day %d: %s cost $%.0f → $%.0f (real estimate)", plan.Days[w.d].Day, it.Title, it.CostUSD, to))
		it.CostUSD = to
	}
	if len(drop) > 0 {
		// Rebuild each day without the invented lines; keep at least one item per day.
		dropped := map[pos]bool{}
		for idx, why := range drop {
			dropped[where[idx]] = true
			audit.Notes = append(audit.Notes, "removed "+why)
		}
		for d := range plan.Days {
			kept := make([]DayItem, 0, len(plan.Days[d].Items))
			for i, it := range plan.Days[d].Items {
				if !dropped[pos{d, i}] {
					kept = append(kept, it)
				}
			}
			if len(kept) > 0 {
				plan.Days[d].Items = kept
			}
		}
	}
	audit.Notes = append(audit.Notes, plan.scheduleNotes...)
	audit.Summary = fmt.Sprintf("checked %d lines against real data: %d removed, %d costs corrected, %d times fixed", len(items), len(drop), len(clamp), len(plan.scheduleNotes))
	// The Reviewer only sees forecast entries for the trip's own dates; the
	// 7-day outlook for other dates would just mislead it.
	var tripWeather []travel.DayWeather
	for _, d := range plan.Days {
		if w := weatherOn(plan.Weather, d.Date); w != nil {
			dw := *w
			// NWS can label a day "Sunny" with a 57% chance of showers later;
			// the number is what the plan acts on, so the label follows it.
			if dw.PrecipPct >= rainyPct && !strings.Contains(strings.ToLower(dw.Short), "rain") && !strings.Contains(strings.ToLower(dw.Short), "shower") {
				dw.Short = fmt.Sprintf("Showers likely (%d%%), %s early", dw.PrecipPct, strings.ToLower(dw.Short))
			}
			tripWeather = append(tripWeather, dw)
		}
	}
	// A compact view (no details or costs) keeps the Reviewer's call short on
	// a shared, slow endpoint.
	type line struct {
		Time, Title, Category string
		Setting               string `json:"setting,omitempty"` // indoor | outdoor, from the place data
	}
	type dayView struct {
		Day   int
		Date  string
		Theme string
		Items []line
	}
	setting := func(title string) string {
		if p, ok := agents.MatchPlace(title, known.Places); ok {
			if isOutdoor(p) {
				return "outdoor"
			}
			return "indoor"
		}
		return ""
	}
	compact := make([]dayView, 0, len(plan.Days))
	for _, d := range plan.Days {
		dv := dayView{Day: d.Day, Date: d.Date, Theme: d.Theme}
		for _, it := range d.Items {
			l := line{Time: it.Time, Title: it.Title, Category: it.Category}
			if it.Category == "activity" {
				l.Setting = setting(it.Title)
			}
			dv.Items = append(dv.Items, l)
		}
		compact = append(compact, dv)
	}
	critique := agents.Critique(ctx, p.AI, trip, map[string]any{"summary": plan.Summary, "days": compact, "forecast_for_trip_days": tripWeather})
	return []agents.Report{audit, critique}
}

// weatherLine summarizes the forecast for the agents' prompts.
func weatherLine(w travel.Weather) string {
	if len(w.Days) == 0 {
		return "forecast unavailable"
	}
	parts := make([]string, 0, len(w.Days))
	for _, d := range w.Days[:min(7, len(w.Days))] {
		parts = append(parts, fmt.Sprintf("%s %s %d/%d°F %d%% rain", d.Date, d.Short, d.HighF, d.LowF, d.PrecipPct))
	}
	return strings.Join(parts, "; ")
}

// keepPlaceNotes drops Researcher notes about candidates the Curator rejected.
func keepPlaceNotes(r *agents.Research, lists ...[]travel.Place) {
	kept := map[string]bool{}
	for _, l := range lists {
		for _, p := range l {
			kept[strings.ToLower(strings.TrimSpace(p.Name))] = true
		}
	}
	out := r.PlaceNotes[:0]
	for _, n := range r.PlaceNotes {
		if kept[strings.ToLower(strings.TrimSpace(n.Name))] {
			out = append(out, n)
		}
	}
	r.PlaceNotes = out
}

// resolve turns a typed place plus an optional pinned point into a Placemark.
// A pinned point (browser GPS or an autocomplete pick) is trusted as-is and
// keeps whatever label the user saw; only the text path hits a geocoder.
func (p *Planner) resolve(ctx context.Context, text string, pinned *travel.LatLng) (travel.Placemark, error) {
	if pinned != nil {
		label := strings.TrimSpace(text)
		if label == "" {
			label = fmt.Sprintf("%.2f, %.2f", pinned.Lat, pinned.Lng)
		}
		name, region := label, ""
		if i := strings.Index(label, ","); i > 0 {
			name, region = strings.TrimSpace(label[:i]), strings.TrimSpace(label[i+1:])
		}
		src := "pinned"
		if strings.HasPrefix(strings.ToLower(label), "my location") {
			src, name = "gps", "My location"
		}
		return travel.Placemark{Label: label, Name: name, Region: region, Country: "US", LatLng: *pinned, Source: src}, nil
	}
	return p.Geo.Lookup(ctx, text)
}

// ChooseHotel makes the named stay the plan's lodging: it moves to the front,
// every option's lodging and total are recomputed, the budget flag is
// refreshed and the itinerary's check-in line is rewritten. The itinerary
// text is otherwise left alone (the AI wrote it around the same activities).
func (pl *Plan) ChooseHotel(name string, budget float64) error {
	idx := -1
	for i, h := range pl.Hotels {
		if strings.EqualFold(h.Name, name) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("no hotel named %q in this plan", name)
	}
	pl.Hotels = moveFront(pl.Hotels, idx)
	h := pl.Hotels[0]
	note := fmt.Sprintf("%s ★%.1f at $%.0f/night", h.Name, h.Rating, h.NightlyUSD)
	for i := range pl.Options {
		o := &pl.Options[i]
		o.LodgingUSD = h.TotalUSD
		o.TotalUSD = round2(o.TransportUSD + o.LodgingUSD + o.ActivitiesUSD + o.LocalUSD)
		o.ValueScore = valueScore(*o, pl.Nights)
		for j := range o.Factors {
			switch {
			case strings.HasSuffix(o.Factors[j], "per night all-in"):
				o.Factors[j] = fmt.Sprintf("$%.0f per night all-in", o.TotalUSD/float64(pl.Nights))
			case strings.Contains(o.Factors[j], "per night all-in (estimates included)"):
				o.Factors[j] = fmt.Sprintf("$%.0f per night all-in (estimates included)", o.TotalUSD/float64(pl.Nights))
			case strings.Contains(o.Factors[j], "per night priced subtotal"):
				o.Factors[j] = fmt.Sprintf("$%.0f per night priced subtotal (local transport excluded)", o.TotalUSD/float64(pl.Nights))
			case strings.Contains(o.Factors[j], "/night"):
				o.Factors[j] = note
			}
		}
		if o.Mode == pl.RecommendedMode {
			pl.TotalUSD = o.TotalUSD
		}
	}
	pl.UnderBudget = budget <= 0 || pl.TotalUSD <= budget
	// The summary quotes the old total; keep it honest with a trailing note.
	if i := strings.Index(pl.Summary, " Stay changed to "); i >= 0 {
		pl.Summary = pl.Summary[:i]
	}
	pl.Summary += fmt.Sprintf(" Stay changed to %s; total now $%.0f.", h.Name, pl.TotalUSD)
	for d := range pl.Days {
		for i := range pl.Days[d].Items {
			it := &pl.Days[d].Items[i]
			if it.Category == "lodging" {
				it.Title = "Check in: " + h.Name
				it.Detail = fmt.Sprintf("%d nights at $%.0f/night · %s", h.Nights, h.NightlyUSD, h.Area)
				it.CostUSD = h.TotalUSD
			}
		}
	}
	return nil
}

// activitiesFor returns things to do shaped by the user's interests: a few per
// chosen interest (deduped, interleaved so no single interest dominates),
// without unrelated sightseeing. No activity interests → general attractions.
func (p *Planner) activitiesFor(ctx context.Context, dest travel.LatLng, interests []string, radius float64) []travel.Place {
	const want = 12
	var lists [][]travel.Place
	for _, in := range interests {
		if in == "food" { // food is handled by its own section
			continue
		}
		if _, ok := map[string]bool{"outdoors": true, "museums": true, "nightlife": true, "family": true, "history": true}[in]; ok {
			lists = append(lists, p.Places.Explore(ctx, dest, in, want, radius))
		}
	}
	if len(lists) == 0 {
		lists = append(lists, p.Places.Explore(ctx, dest, "attraction", want, radius))
	}
	seen := map[string]bool{}
	out := make([]travel.Place, 0, want)
	for i := 0; len(out) < want; i++ {
		progressed := false
		for _, l := range lists {
			if i < len(l) {
				progressed = true
				if !seen[l[i].Name] {
					seen[l[i].Name] = true
					out = append(out, l[i])
					if len(out) == want {
						break
					}
				}
			}
		}
		if !progressed {
			break
		}
	}
	return out
}

// pickHotel moves the chosen stay to index 0: the best-rated option whose total
// fits in budgetLeft, else the cheapest. budgetLeft <= 0 means "no budget set",
// so we keep the value ordering. preferCentral (elevated-crime destinations)
// restricts the pick to stays within ~3 km of the center when any exist.
func pickHotel(hotels []travel.HotelQuote, budgetLeft float64, preferCentral bool) []travel.HotelQuote {
	if len(hotels) < 2 {
		return hotels
	}
	central := func(h travel.HotelQuote) bool { return h.DistanceKm == 0 || h.DistanceKm <= 3 }
	anyCentral := false
	for _, h := range hotels {
		if central(h) {
			anyCentral = true
			break
		}
	}
	if budgetLeft <= 0 {
		if preferCentral && anyCentral && !central(hotels[0]) {
			for i, h := range hotels {
				if central(h) {
					return moveFront(hotels, i)
				}
			}
		}
		return hotels
	}
	best, cheapest := -1, -1
	for i, h := range hotels {
		if preferCentral && anyCentral && !central(h) {
			continue
		}
		if cheapest < 0 || h.TotalUSD < hotels[cheapest].TotalUSD {
			cheapest = i
		}
		if h.TotalUSD <= budgetLeft && (best < 0 || h.Rating > hotels[best].Rating) {
			best = i
		}
	}
	pick := best
	if pick < 0 {
		pick = cheapest
	}
	return moveFront(hotels, pick)
}

func moveFront(hotels []travel.HotelQuote, pick int) []travel.HotelQuote {
	out := make([]travel.HotelQuote, 0, len(hotels))
	out = append(out, hotels[pick])
	for i, h := range hotels {
		if i != pick {
			out = append(out, h)
		}
	}
	return out
}

// valueScore rewards low cost and low travel time per night of vacation.
func valueScore(o Option, nights int) float64 {
	costPerNight := o.TotalUSD / float64(nights)
	score := 10000/(costPerNight+50) - o.TravelHours*0.6
	return round1(score)
}

// outline is the Writer agent's first pass: which real places go on which
// day, plus the summary. Indices point into plan.Activities / Food / Stops.
type outline struct {
	Summary string `json:"summary"`
	Days    []struct {
		Theme      string `json:"theme"`
		Activities []int  `json:"activities"`
		Breakfast  *int   `json:"breakfast"`
		Lunch      *int   `json:"lunch"`
		Dinner     *int   `json:"dinner"`
		Stops      []int  `json:"stops"`
	} `json:"days"`
}

// dayAssignment is the validated form of one outline day.
type dayAssignment struct {
	Theme      string
	Activities []travel.Place
	Meals      dayMeals
	Stops      []travel.Place
}

type dayResponse struct {
	Items []DayItem `json:"items"`
}

// narrate is the Writer agent. The inference service generates roughly 17
// tokens a second, so one call for a whole itinerary takes minutes. Instead it
// makes one short outline call (summary + which place goes on which day) and
// then writes every day in parallel; each call is a few hundred tokens. A day
// whose call fails falls back to the rule-based schedule for that day.
func (p *Planner) narrate(ctx context.Context, req Request, plan *Plan, start time.Time, emit Progress) error {
	if emit == nil {
		emit = func(Event) {}
	}
	emit(Event{Stage: "writer", Message: "Writer is outlining the trip"})
	assign, summary, err := p.outlineDays(ctx, req, plan, start)
	if err != nil {
		return fmt.Errorf("outline: %w", err)
	}
	fallbackSummary, fallbackDays := fallbackItinerary(req, plan, start)
	if summary == "" {
		summary = fallbackSummary
	}
	emit(Event{Stage: "writer", Message: fmt.Sprintf("outline done; writing %d days in parallel", plan.Nights+1)})

	days := make([]Day, plan.Nights+1)
	var wg sync.WaitGroup
	for i := range days {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			date := start.AddDate(0, 0, i).Format("2006-01-02")
			items, err := p.writeDay(ctx, plan, i, date, assign[i])
			if err != nil {
				log.Printf("planner: writer day %d failed, using rule-based day: %v", i+1, err)
				days[i] = fallbackDays[i]
				emit(Event{Stage: "writer", Message: fmt.Sprintf("day %d kept the rule-based schedule (%v)", i+1, err)})
				return
			}
			days[i] = Day{Day: i + 1, Date: date, Theme: assign[i].Theme, Items: items}
			emit(Event{Stage: "writer", Message: fmt.Sprintf("day %d written: %s", i+1, assign[i].Theme)})
		}(i)
	}
	wg.Wait()

	// Safety net: travel legs and the hotel check-in always appear, even when
	// the model skipped them, and road stops never appear on a flight.
	for i := range days {
		ensureCategory := func(cat string) {
			for _, it := range days[i].Items {
				if it.Category == cat {
					return
				}
			}
			for _, it := range fallbackDays[i].Items {
				if it.Category == cat {
					days[i].Items = append(days[i].Items, it)
				}
			}
		}
		if i == 0 || i == len(days)-1 {
			ensureCategory("transport")
		}
		if i == 0 {
			ensureCategory("lodging")
		}
		if plan.RecommendedMode == "fly" {
			kept := make([]DayItem, 0, len(days[i].Items))
			for _, it := range days[i].Items {
				if it.Category != "stop" {
					kept = append(kept, it)
				}
			}
			days[i].Items = kept
		}
		sort.SliceStable(days[i].Items, func(a, b int) bool {
			ta, tb := days[i].Items[a].Time, days[i].Items[b].Time
			if ta == "--:--" || tb == "--:--" {
				return false
			}
			return ta < tb
		})
	}
	plan.scheduleNotes = append(plan.scheduleNotes, normalizeSchedule(plan, days)...)
	days = sanitizeDays(days, start, plan.Nights+1)
	if len(days) == 0 {
		return fmt.Errorf("ai returned empty plan")
	}
	if err := validateActivityVariety(days, plan.Activities); err != nil {
		log.Printf("planner: %v", err)
	}
	plan.Summary = strings.TrimSpace(summary) + mobilityNote(plan)
	plan.Days = days
	if effectiveTransport(plan) != "own car" {
		plan.Days[0].Items = append(plan.Days[0].Items, destinationTransfer(plan))
	}
	return nil
}

// outlineDays asks the model to assign the curated places to days and write the
// summary, then validates the assignment: valid indices only, no place twice,
// at most one activity on travel days and two otherwise, nearby same-type
// activities never paired, stops only on a driving day 1.
func (p *Planner) outlineDays(ctx context.Context, req Request, plan *Plan, start time.Time) ([]dayAssignment, string, error) {
	n := plan.Nights + 1
	rec := plan.Options[0]
	for _, o := range plan.Options {
		if o.Mode == plan.RecommendedMode {
			rec = o
		}
	}
	type idxPlace struct {
		I     int     `json:"i"`
		Name  string  `json:"name"`
		Type  string  `json:"type,omitempty"`
		Miles float64 `json:"miles_from_center"`
		Note  string  `json:"note,omitempty"`
	}
	index := func(list []travel.Place, typed bool) []idxPlace {
		out := make([]idxPlace, 0, len(list))
		for i, pl := range list {
			ip := idxPlace{I: i, Name: pl.Name, Miles: round1(distance(plan.DestLoc, pl.Location)), Note: pl.Note}
			if typed {
				ip.Type = activityType(pl)
			}
			out = append(out, ip)
		}
		return out
	}
	weather := make([]string, 0, n)
	for i := 0; i < n; i++ {
		d := start.AddDate(0, 0, i).Format("2006-01-02")
		line := fmt.Sprintf("day %d (%s): forecast unavailable", i+1, d)
		if w := weatherOn(plan.Weather, d); w != nil {
			line = fmt.Sprintf("day %d (%s): %s, %d/%d°F, %d%% rain", i+1, d, w.Short, w.HighF, w.LowF, w.PrecipPct)
		}
		weather = append(weather, line)
	}
	data := map[string]any{
		"origin": plan.OriginPlace.Label, "destination": plan.DestPlace.Label,
		"days": n, "nights": plan.Nights, "travelers": req.Travelers, "budget_usd": req.BudgetUSD,
		"interests": req.Interests, "mode": plan.RecommendedMode, "why_this_mode": rec.Why,
		"total_usd": plan.TotalUSD, "under_budget": plan.UnderBudget,
		"transport_usd": rec.TransportUSD, "lodging_usd": rec.LodgingUSD, "activities_usd": rec.ActivitiesUSD,
		"hotel": first(plan.Hotels), "weather": weather, "safety_level": plan.Safety.Level,
		"local_transport": effectiveTransport(plan), "activity_radius_miles": plan.ActivityRadiusMiles,
		"activities": index(plan.Activities, true), "food": indexFood(plan), "stops": index(plan.Stops, false),
	}
	if plan.Research != nil {
		data["research"] = map[string]any{"overview": plan.Research.Overview, "tips": plan.Research.Tips, "avoid": plan.Research.Avoid, "seasonal": plan.Research.Seasonal}
	}
	raw, _ := json.Marshal(data)
	system := fmt.Sprintf(`You are the Writer agent of TPN, a sharp, budget-savvy travel planner, doing the outline pass. Reply with JSON only:
{"summary": string, "days": [{"theme": string, "activities": [int], "breakfast": int|null, "lunch": int|null, "dinner": int|null, "stops": [int]}]}
"days" has exactly %d entries, in order. Day 1 is the travel-out day and the last day is the travel-home day: give each at most one activity (within 3 miles of center on the last day) and no activity on day 1 when the trip is long. Middle days get TWO activities (one for the morning, one for the afternoon) whenever two unused places of DIFFERENT types exist, preferring ones near each other; vary types across days too (not a small park every day; pick the most distinctive places first); prefer indoor types on days with 50%%+ rain and say so in the theme. Each activity index at most once in the whole trip. Meals come from the food list: "breakfast" must be an entry whose meal is "breakfast" (cafés, bakeries); "lunch" and "dinner" are restaurants (meal "any"), each restaurant used at most once across the trip, lunch near that day's morning activity and dinner near the hotel. Day 1: no breakfast (you are travelling) and dinner only unless you arrive by 13:00; last day: breakfast only. Stops only on day 1 and only when mode is "drive", at most 2. Use only indices that exist. Themes are 3-7 words. The summary is 2-3 sentences: name the total cost and why this mode is the best value, note the budget fit, and when safety_level is "elevated" include one concrete safety tip; use research for one specific, useful observation. Never invent prices or places.`, n)
	var out outline
	cctx, cancel := context.WithTimeout(ctx, 100*time.Second)
	defer cancel()
	if err := p.AI.ChatJSON(cctx, system, string(raw), &out); err != nil {
		return nil, "", err
	}
	if len(out.Days) == 0 {
		return nil, "", fmt.Errorf("outline had no days")
	}

	assign := make([]dayAssignment, n)
	usedAct, usedFood := map[int]bool{}, map[int]bool{}
	arrival := arrivalMinutes(plan, min(2, len(plan.Stops)))
	defaults := planMeals(plan, n, arrival, 12*60)
	// pickMeal takes the model's index when it is a valid, unused place of the
	// right kind, else the deterministic default for that slot.
	pickMeal := func(idx *int, wantBreakfast bool, def *travel.Place) *travel.Place {
		if idx != nil && *idx >= 0 && *idx < len(plan.Food) && !usedFood[*idx] && isBreakfastPlace(plan.Food[*idx]) == wantBreakfast {
			usedFood[*idx] = true
			p := plan.Food[*idx]
			return &p
		}
		return def
	}
	for i := 0; i < n; i++ {
		wantB, wantL, wantD := mealsWanted(i, n, arrival, 12*60)
		if i >= len(out.Days) {
			assign[i].Theme = "Explore " + plan.DestPlace.Name
			assign[i].Meals = defaults[i]
			continue
		}
		d := out.Days[i]
		assign[i].Theme = strings.TrimSpace(d.Theme)
		maxAct := 2
		if i == 0 || i == n-1 {
			maxAct = 1
		}
		for _, idx := range d.Activities {
			if idx < 0 || idx >= len(plan.Activities) || usedAct[idx] || len(assign[i].Activities) == maxAct {
				continue
			}
			pl := plan.Activities[idx]
			if i == n-1 && distance(plan.DestLoc, pl.Location) > 3 {
				continue
			}
			if len(assign[i].Activities) == 1 && !differentNearbyActivities(assign[i].Activities[0], pl) {
				continue
			}
			usedAct[idx] = true
			assign[i].Activities = append(assign[i].Activities, pl)
		}
		if wantB {
			assign[i].Meals.Breakfast = pickMeal(d.Breakfast, true, defaults[i].Breakfast)
		}
		if wantL {
			assign[i].Meals.Lunch = pickMeal(d.Lunch, false, defaults[i].Lunch)
		}
		if wantD {
			assign[i].Meals.Dinner = pickMeal(d.Dinner, false, defaults[i].Dinner)
		}
		if i == 0 && plan.RecommendedMode == "drive" {
			for _, idx := range d.Stops {
				if idx >= 0 && idx < len(plan.Stops) && len(assign[i].Stops) < 2 {
					assign[i].Stops = append(assign[i].Stops, plan.Stops[idx])
				}
			}
		}
	}
	// Rain is handled deterministically, whatever the model did with the forecast.
	if notes := rainAwareAssign(assign, plan, start); len(notes) > 0 {
		plan.scheduleNotes = append(plan.scheduleNotes, notes...)
	}
	return assign, strings.TrimSpace(out.Summary), nil
}

// writeDay asks the model for one day's schedule around the places the outline
// assigned to it and the travel facts for that day.
func (p *Planner) writeDay(ctx context.Context, plan *Plan, i int, date string, a dayAssignment) ([]DayItem, error) {
	n := plan.Nights + 1
	trav := float64(max(1, plan.Travelers))
	type placeFacts struct {
		Name         string  `json:"name"`
		Note         string  `json:"note,omitempty"`
		ResearchNote string  `json:"research_note,omitempty"`
		Miles        float64 `json:"miles_from_center"`
		GroupCostUSD float64 `json:"group_cost_usd"`
		PriceUnknown bool    `json:"price_unknown,omitempty"`
	}
	facts := func(list []travel.Place) []placeFacts {
		out := make([]placeFacts, 0, len(list))
		for _, pl := range list {
			f := placeFacts{Name: pl.Name, Note: pl.Note, Miles: round1(distance(plan.DestLoc, pl.Location)), GroupCostUSD: round2(pl.PriceUSD * trav), PriceUnknown: pl.PriceUnknown}
			if plan.Research != nil {
				for _, rn := range plan.Research.PlaceNotes {
					if strings.EqualFold(strings.TrimSpace(rn.Name), strings.TrimSpace(pl.Name)) {
						f.ResearchNote = rn.Note
					}
				}
			}
			out = append(out, f)
		}
		return out
	}
	// A road stop's cost is the refuel for the car, not per person.
	stopFacts := func(list []travel.Place) []placeFacts {
		out := make([]placeFacts, 0, len(list))
		for _, pl := range list {
			out = append(out, placeFacts{Name: pl.Name, Note: pl.Note, Miles: round1(distance(plan.OriginLoc, pl.Location)), GroupCostUSD: round2(pl.PriceUSD), PriceUnknown: pl.PriceUnknown})
		}
		return out
	}
	// Fixed items: the model keeps title, category and cost verbatim and only
	// writes the time and the detail sentence.
	type fixed struct {
		Title       string  `json:"title"`
		Category    string  `json:"category"`
		CostUSD     float64 `json:"cost_usd"`
		Facts       string  `json:"facts"`
		CostUnknown bool    `json:"cost_unknown,omitempty"`
	}
	var travelFacts []fixed
	if i == 0 {
		if plan.RecommendedMode == "drive" {
			travelFacts = append(travelFacts, fixed{Title: "Depart " + plan.OriginPlace.Name, Category: "transport", CostUSD: plan.Drive.StartTankUSD,
				Facts: fmt.Sprintf("%.0f miles, about %dh %dm; leave on a full tank (%.0f gal, ~%.0f mi range, $%.0f to fill); the stops below are the planned refuels on this route", plan.Drive.DistanceMiles, plan.Drive.DurationMin/60, plan.Drive.DurationMin%60, plan.Drive.TankGallons, plan.Drive.RangeMiles, plan.Drive.StartTankUSD)})
		} else if f := first(plan.Flights); f != nil {
			if a := plan.OriginAirport; a != nil && a.DistanceMiles > 20 {
				travelFacts = append(travelFacts, fixed{Title: fmt.Sprintf("Travel to %s (%s)", a.Code, a.Name), Category: "transport", CostUnknown: true,
					Facts: fmt.Sprintf("%.0f miles from %s; transfer cost unknown", a.DistanceMiles, plan.OriginPlace.Name)})
			}
			travelFacts = append(travelFacts, fixed{Title: fmt.Sprintf("Fly %s → %s on %s", f.Origin, f.Destination, f.Carrier), Category: "transport", CostUSD: round2(f.PriceUSD * trav),
				Facts: fmt.Sprintf("round trip for %d traveler(s), %s, %dh %dm in the air", plan.Travelers, stopsLabel(f.Stops), f.DurationMin/60, f.DurationMin%60)})
			if a := plan.DestAirport; a != nil && a.DistanceMiles > 20 {
				travelFacts = append(travelFacts, fixed{Title: fmt.Sprintf("%s → %s", a.Code, plan.DestPlace.Name), Category: "transport", CostUnknown: true,
					Facts: fmt.Sprintf("%.0f miles by %s; cost unknown", a.DistanceMiles, effectiveTransport(plan))})
			}
		}
		if h := first(plan.Hotels); h != nil {
			travelFacts = append(travelFacts, fixed{Title: "Check in: " + h.Name, Category: "lodging", CostUSD: h.TotalUSD,
				Facts: fmt.Sprintf("%d nights at $%.0f/night, rated %.1f, %s", h.Nights, h.NightlyUSD, h.Rating, firstNonEmptyStr(h.Area, "location on the map"))})
		}
	}
	if i == n-1 {
		if plan.RecommendedMode == "drive" {
			travelFacts = append(travelFacts, fixed{Title: "Drive back to " + plan.OriginPlace.Name, Category: "transport", CostUSD: plan.Drive.StartTankUSD,
				Facts: fmt.Sprintf("%.0f miles, about %dh %dm; fill the tank before leaving ($%.0f)", plan.Drive.DistanceMiles, plan.Drive.DurationMin/60, plan.Drive.DurationMin%60, plan.Drive.StartTankUSD)})
			if n := len(plan.Drive.Refuels); n > 0 {
				travelFacts = append(travelFacts, refuelHomeItem(plan))
			}
		} else {
			home := plan.OriginPlace.Name
			if a := plan.OriginAirport; a != nil {
				home = a.Code + ", then home to " + plan.OriginPlace.Name
			}
			travelFacts = append(travelFacts, fixed{Title: "Fly back to " + home, Category: "transport", Facts: "return fare already included in the outbound price"})
		}
	}
	w := weatherOn(plan.Weather, date)
	// Plain words beat the raw record: NWS can say "Sunny" with a 57% chance
	// of rain (showers later), and the model should not call that a sunny day.
	weatherText := "no forecast for this date yet; do not describe the weather"
	if w != nil {
		switch {
		case w.PrecipPct >= rainyPct:
			weatherText = fmt.Sprintf("likely rain (%d%% chance), high %d°F: call it a rainy day, plan indoors, suggest a jacket", w.PrecipPct, w.HighF)
		case w.PrecipPct >= 30:
			weatherText = fmt.Sprintf("%s with a %d%% chance of showers, high %d°F", w.Short, w.PrecipPct, w.HighF)
		default:
			weatherText = fmt.Sprintf("%s, high %d°F, low %d°F", w.Short, w.HighF, w.LowF)
		}
	}
	data := map[string]any{
		"day": i + 1, "of": n, "date": date, "theme": a.Theme, "destination": plan.DestPlace.Label,
		"timing_rules": dayTimingRules(plan, i, n, len(a.Stops)),
		"travelers":    plan.Travelers, "local_transport": effectiveTransport(plan),
		"weather": weatherText, "safety_level": plan.Safety.Level, "safety_tip": firstOf(plan.Safety.Tips),
		"fixed_items": travelFacts, "activities": facts(a.Activities), "meals": mealFacts(a.Meals, plan), "stops": stopFacts(a.Stops),
	}
	raw, _ := json.Marshal(data)
	system := `You are the Writer agent of TPN, a budget-savvy travel planner, writing ONE day. Reply with JSON only: {"items":[{"time":"HH:MM","title":string,"detail":string,"cost_usd":number,"category":"transport|lodging|activity|food|stop"}]}.
Rules: 4-9 items in clock order. Obey timing_rules exactly: they give the real arrival time and the latest time to be done before the trip home. Meal windows: breakfast 07:30-09:30, lunch 12:00-13:30, dinner 17:30-20:30. Include every fixed_item as its own item copying its title, category and cost_usd exactly; write only its time and detail from its facts. Include every listed activity, meal and stop exactly once: activities and stops use the exact name as the title; each meal uses its given title verbatim (e.g. "Lunch at <name>") with category food; cost_usd = group_cost_usd. Never add a place that is not listed and never invent a cost. Each detail is one concrete sentence of at most 25 words drawing on note, research_note and the weather; when safety_level is "elevated" keep evening items near the hotel and mention rideshare after 22:00. Say opening hours are unverified at most once per day. Leave gaps free rather than padding.`
	var out dayResponse
	cctx, cancel := context.WithTimeout(ctx, 100*time.Second)
	defer cancel()
	if err := p.AI.ChatJSON(cctx, system, string(raw), &out); err != nil {
		return nil, err
	}
	if len(out.Items) == 0 {
		return nil, fmt.Errorf("empty day")
	}
	// Fixed items keep their known cost even if the model changed it.
	for k := range out.Items {
		for _, f := range travelFacts {
			if strings.EqualFold(strings.TrimSpace(out.Items[k].Title), f.Title) {
				out.Items[k].Category, out.Items[k].CostUSD, out.Items[k].CostUnknown = f.Category, f.CostUSD, f.CostUnknown
			}
		}
	}
	return out.Items, nil
}

// dayTimingRules spells out the schedule physics for one day so the Writer
// gets them right the first time; normalizeSchedule enforces them anyway.
func dayTimingRules(plan *Plan, i, n, stops int) []string {
	var rules []string
	if i == 0 {
		at := arrivalMinutes(plan, stops)
		rules = append(rules, fmt.Sprintf("You leave home at 08:00 and arrive at the destination at %s. Nothing at the destination (check-in, activities, dinner) can start before %s.", fmtHM(at), fmtHM(at)))
		switch {
		case at > dinnerCutoff:
			rules = append(rules, "Arrival is too late for dinner or an activity: only the travel legs and check-in.")
		case at+60 > activityLast:
			rules = append(rules, "Arrival is too late for an activity; dinner right after check-in is fine.")
		}
	}
	if i == n-1 && n > 1 {
		if plan.RecommendedMode == "fly" {
			rules = append(rules, "This is the flight-home day: every activity must finish at least 3 hours before the flight; put the flight at 14:00 or later if there is a morning activity, and nothing after it.")
		} else {
			rules = append(rules, "This is the drive-home day: finish the last activity at least 45 minutes before departing, and schedule nothing after the drive home.")
		}
	}
	if i > 0 && i < n-1 {
		rules = append(rules, "A full day: breakfast 08:00-09:00, first activity 09:30-11:00, lunch 12:00-13:00, second activity 13:30-15:30, dinner 18:00-19:30.")
	}
	return rules
}

// refuelHomeItem is the return leg's gas stops rolled into one line, since
// no stations are looked up for the way back.
func refuelHomeItem(plan *Plan) struct {
	Title       string  `json:"title"`
	Category    string  `json:"category"`
	CostUSD     float64 `json:"cost_usd"`
	Facts       string  `json:"facts"`
	CostUnknown bool    `json:"cost_unknown,omitempty"`
} {
	total := 0.0
	for _, r := range plan.Drive.Refuels {
		total += r.CostUSD
	}
	n := len(plan.Drive.Refuels)
	return struct {
		Title       string  `json:"title"`
		Category    string  `json:"category"`
		CostUSD     float64 `json:"cost_usd"`
		Facts       string  `json:"facts"`
		CostUnknown bool    `json:"cost_unknown,omitempty"`
	}{Title: fmt.Sprintf("Refuel on the way home (%d stop%s)", n, plural(n)), Category: "transport", CostUSD: round2(total),
		Facts: fmt.Sprintf("about every %.0f miles, %.1f gal each", plan.Drive.RangeMiles*0.85, plan.Drive.Refuels[0].Gallons)}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// indexFood lists the food candidates for the outline with their meal kind.
func indexFood(plan *Plan) []map[string]any {
	out := make([]map[string]any, 0, len(plan.Food))
	for i, pl := range plan.Food {
		meal := "any"
		if isBreakfastPlace(pl) {
			meal = "breakfast"
		}
		out = append(out, map[string]any{"i": i, "name": pl.Name, "meal": meal, "miles_from_center": round1(distance(plan.DestLoc, pl.Location)), "note": pl.Note})
	}
	return out
}

// mealFacts describes a day's meals for the Writer, each with the exact
// title it must use and the group cost (lunch at the lunch rate).
func mealFacts(m dayMeals, plan *Plan) []map[string]any {
	trav := float64(max(1, plan.Travelers))
	var out []map[string]any
	add := func(slot string, p *travel.Place, factor float64) {
		if p == nil {
			return
		}
		f := map[string]any{"slot": slot, "title": mealTitle(slot, *p), "name": p.Name, "note": p.Note,
			"miles_from_center": round1(distance(plan.DestLoc, p.Location)), "group_cost_usd": round2(p.PriceUSD * factor * trav), "price_unknown": p.PriceUnknown}
		if plan.Research != nil {
			for _, rn := range plan.Research.PlaceNotes {
				if strings.EqualFold(strings.TrimSpace(rn.Name), strings.TrimSpace(p.Name)) {
					f["research_note"] = rn.Note
				}
			}
		}
		out = append(out, f)
	}
	add("breakfast", m.Breakfast, 1)
	add("lunch", m.Lunch, lunchFactor)
	add("dinner", m.Dinner, 1)
	return out
}

func firstNonEmptyStr(s ...string) string {
	for _, v := range s {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func firstOf(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}

var allowedCategories = map[string]bool{"transport": true, "lodging": true, "activity": true, "food": true, "stop": true}

// sanitizeDays makes the model's itinerary safe to render: valid categories,
// non-negative costs, sequential day numbers and dates, no empty days.
func sanitizeDays(in []Day, start time.Time, maxDays int) []Day {
	out := make([]Day, 0, len(in))
	for _, d := range in {
		items := make([]DayItem, 0, len(d.Items))
		for _, it := range d.Items {
			if strings.TrimSpace(it.Title) == "" {
				continue
			}
			if !allowedCategories[it.Category] {
				it.Category = "activity"
			}
			if it.CostUSD < 0 {
				it.CostUSD = 0
			}
			if len(it.Time) != 5 {
				it.Time = "--:--"
			}
			items = append(items, it)
		}
		if len(items) == 0 {
			continue
		}
		n := len(out) + 1
		if n > maxDays {
			break
		}
		out = append(out, Day{Day: n, Date: start.AddDate(0, 0, n-1).Format("2006-01-02"), Theme: d.Theme, Items: items})
	}
	return out
}

func fallbackItinerary(req Request, plan *Plan, start time.Time) (string, []Day) {
	days := []Day{}
	total := plan.Nights + 1
	activitiesByDay := dailyActivities(plan.Activities, plan.DestLoc, plan.Weather, start, plan.Nights)
	homeAt := 12 * 60
	mealsByDay := planMeals(plan, total, arrivalMinutes(plan, min(2, len(plan.Stops))), homeAt)
	for i := 0; i < total; i++ {
		d := Day{Day: i + 1, Date: start.AddDate(0, 0, i).Format("2006-01-02")}
		switch {
		case i == 0:
			d.Theme = "Travel day"
			if plan.RecommendedMode == "drive" {
				d.Items = append(d.Items, DayItem{Time: "08:00", Title: "Depart " + plan.OriginPlace.Name, Detail: fmt.Sprintf("%.0f mi, about %dh %dm; leave on a full tank (%.0f gal, $%.0f to fill)", plan.Drive.DistanceMiles, plan.Drive.DurationMin/60, plan.Drive.DurationMin%60, plan.Drive.TankGallons, plan.Drive.StartTankUSD), CostUSD: plan.Drive.StartTankUSD, Category: "transport"})
				for k, s := range plan.Stops {
					// Space the stops across the drive by their position on the route.
					at := departMin + int(float64(plan.Drive.DurationMin)*float64(k+1)/float64(len(plan.Stops)+1))
					d.Items = append(d.Items, DayItem{Time: fmtHM(at), Title: s.Name, Detail: s.Note, CostUSD: s.PriceUSD, CostUnknown: s.PriceUnknown, Category: "stop"})
				}
			} else if len(plan.Flights) > 0 {
				f := plan.Flights[plan.SelectedFlight]
				price := f.PriceUSD
				if plan.SelectedReturn != nil {
					price = plan.SelectedReturn.PriceUSD
				}
				if a := plan.OriginAirport; a != nil && a.DistanceMiles > 20 {
					d.Items = append(d.Items, DayItem{Time: "07:00", Title: fmt.Sprintf("Travel to %s (%s)", a.Code, a.Name), Detail: fmt.Sprintf("%.0f mi from %s; arrange airport transportation, cost unverified", a.DistanceMiles, plan.OriginPlace.Name), CostUnknown: true, Category: "transport"})
				}
				d.Items = append(d.Items, DayItem{Time: "09:00", Title: fmt.Sprintf("Fly %s → %s on %s", f.Origin, f.Destination, f.Carrier), Detail: fmt.Sprintf("round trip, %d traveler(s), %s", req.Travelers, stopsLabel(f.Stops)), CostUSD: price * float64(req.Travelers), Category: "transport"})
				if a := plan.DestAirport; a != nil && a.DistanceMiles > 20 {
					d.Items = append(d.Items, DayItem{Time: "13:00", Title: fmt.Sprintf("%s → %s", a.Code, plan.DestPlace.Name), Detail: fmt.Sprintf("%.0f mi; arrange transfer using %s, cost unverified", a.DistanceMiles, effectiveTransport(plan)), CostUnknown: true, Category: "transport"})
				}
			}
			if effectiveTransport(plan) != "own car" {
				d.Items = append(d.Items, destinationTransfer(plan))
			}
			if h := first(plan.Hotels); h != nil {
				d.Items = append(d.Items, DayItem{Time: "16:00", Title: "Check in: " + h.Name, Detail: fmt.Sprintf("%d nights at $%.0f/night · %s", h.Nights, h.NightlyUSD, h.Area), CostUSD: h.TotalUSD, Category: "lodging"})
			}
			if f := mealsByDay[i].Lunch; f != nil {
				d.Items = append(d.Items, mealItem("lunch", "12:30", *f, req.Travelers))
			}
			if f := mealsByDay[i].Dinner; f != nil {
				d.Items = append(d.Items, mealItem("dinner", "19:00", *f, req.Travelers))
			}
		case i == total-1:
			d.Theme = "Head home"
			if f := mealsByDay[i].Breakfast; f != nil {
				d.Items = append(d.Items, mealItem("breakfast", "08:00", *f, req.Travelers))
			}
			if f := mealsByDay[i].Lunch; f != nil {
				d.Items = append(d.Items, mealItem("lunch", "12:15", *f, req.Travelers))
			}
			if len(activitiesByDay[i]) > 0 {
				a := activitiesByDay[i][0]
				d.Items = append(d.Items, DayItem{Time: "09:00", Title: a.Name, Detail: a.Note + " · " + localTravelDetail(plan, a), CostUSD: a.PriceUSD * float64(req.Travelers), Category: "activity"})
			}
			if plan.RecommendedMode == "drive" {
				d.Items = append(d.Items, DayItem{Time: "12:00", Title: "Drive back to " + plan.OriginPlace.Name, Detail: fmt.Sprintf("fill the tank before leaving ($%.0f)", plan.Drive.StartTankUSD), CostUSD: plan.Drive.StartTankUSD, Category: "transport"})
				if len(plan.Drive.Refuels) > 0 {
					r := refuelHomeItem(plan)
					d.Items = append(d.Items, DayItem{Time: "15:00", Title: r.Title, Detail: r.Facts, CostUSD: r.CostUSD, Category: "transport"})
				}
			} else {
				home := plan.OriginPlace.Name
				if a := plan.OriginAirport; a != nil {
					home = a.Code + ", then home to " + plan.OriginPlace.Name
				}
				detail := "return fare included; return flight not yet selected"
				if plan.SelectedReturn != nil {
					detail = fmt.Sprintf("%s, %s; fare already included", plan.SelectedReturn.Carrier, plan.SelectedReturn.DepartureTime)
				}
				d.Items = append(d.Items, DayItem{Time: "12:00", Title: "Fly back to " + home, Detail: detail, CostUSD: 0, Category: "transport"})
			}
		default:
			d.Theme = "Explore " + plan.DestPlace.Name
			if w := weatherOn(plan.Weather, d.Date); w != nil && w.PrecipPct >= 50 {
				d.Theme = fmt.Sprintf("Indoor day (%s, %d%% chance of rain)", strings.ToLower(w.Short), w.PrecipPct)
			}
			for j, a := range activitiesByDay[i] {
				if distance(plan.DestLoc, a.Location) > 20 {
					d.Theme = "Excursion: " + a.Name
				}
				d.Items = append(d.Items, DayItem{Time: []string{"10:00", "14:00"}[j], Title: a.Name, Detail: a.Note + " · " + localTravelDetail(plan, a), CostUSD: a.PriceUSD * float64(req.Travelers), Category: "activity"})
			}
			if plan.Safety.Level == "elevated" && len(plan.Activities) > 0 {
				// Late items get a safety nudge; the AI path gets the same instruction in its prompt.
				d.Items = append(d.Items, DayItem{Time: "21:30", Title: "Evening out near the hotel", Detail: "rideshare back after 10pm (safety: elevated)", CostUSD: 0, Category: "activity"})
			}
			if f := mealsByDay[i].Breakfast; f != nil {
				d.Items = append(d.Items, mealItem("breakfast", "08:30", *f, req.Travelers))
			}
			if f := mealsByDay[i].Lunch; f != nil {
				d.Items = append(d.Items, mealItem("lunch", "12:30", *f, req.Travelers))
			}
			if f := mealsByDay[i].Dinner; f != nil {
				d.Items = append(d.Items, mealItem("dinner", "18:30", *f, req.Travelers))
			}
		}
		// Keep each day in clock order (the safety nudge is appended late).
		sort.SliceStable(d.Items, func(a, b int) bool { return d.Items[a].Time < d.Items[b].Time })
		days = append(days, d)
	}
	rec := plan.Options[0]
	for _, o := range plan.Options {
		if o.Mode == plan.RecommendedMode {
			rec = o
		}
	}
	fits := ""
	if req.BudgetUSD > 0 {
		if plan.UnderBudget {
			fits = fmt.Sprintf(" That leaves $%.0f of your $%.0f budget.", req.BudgetUSD-plan.TotalUSD, req.BudgetUSD)
		} else {
			fits = fmt.Sprintf(" That is $%.0f over your $%.0f budget.", plan.TotalUSD-req.BudgetUSD, req.BudgetUSD)
		}
	}
	extra := ""
	if len(plan.Weather.Days) > 0 {
		w := plan.Weather.Days[0]
		if w.Date == req.StartDate {
			extra += fmt.Sprintf(" Weather on arrival: %s, high %d°F.", strings.ToLower(w.Short), w.HighF)
		} else {
			extra += fmt.Sprintf(" Current outlook in %s: %s, high %d°F.", plan.DestPlace.Name, strings.ToLower(w.Short), w.HighF)
		}
	}
	if plan.Safety.Level == "elevated" && len(plan.Safety.Tips) > 0 {
		extra += " Safety: " + plan.Safety.Tips[0]
	}
	summary := fmt.Sprintf("%s to %s for %d traveler(s), %d night(s). Best value is to %s: about $%.0f priced subtotal (%s).%s%s",
		plan.OriginPlace.Label, plan.DestPlace.Label, req.Travelers, plan.Nights, plan.RecommendedMode, plan.TotalUSD, rec.Why, fits, extra)
	normalizeSchedule(plan, days)
	return summary + mobilityNote(plan), days
}

func weatherOn(w travel.Weather, date string) *travel.DayWeather {
	for i := range w.Days {
		if w.Days[i].Date == date {
			return &w.Days[i]
		}
	}
	return nil
}

// indoorFirst stable-sorts activities so likely-indoor ones lead on wet days.
func indoorFirst(in []travel.Place) []travel.Place {
	indoor := func(p travel.Place) bool {
		n := strings.ToLower(p.Name)
		if p.Category == "museums" || p.Category == "nightlife" {
			return true
		}
		for _, k := range []string{"museum", "gallery", "center", "centre", "hall", "aquarium", "theater", "theatre", "market", "brewery"} {
			if strings.Contains(n, k) {
				return true
			}
		}
		return false
	}
	out := make([]travel.Place, 0, len(in))
	for _, p := range in {
		if indoor(p) {
			out = append(out, p)
		}
	}
	for _, p := range in {
		if !indoor(p) {
			out = append(out, p)
		}
	}
	return out
}

func stopsLabel(n int) string {
	switch n {
	case 0:
		return "nonstop"
	case 1:
		return "1 stop"
	}
	return fmt.Sprintf("%d stops", n)
}

func rest[T any](s []T, n int) []T {
	if len(s) <= 1 {
		return nil
	}
	if len(s)-1 < n {
		n = len(s) - 1
	}
	return s[1 : 1+n]
}

func first[T any](s []T) *T {
	if len(s) == 0 {
		return nil
	}
	return &s[0]
}

// placesSource names the provider(s) behind the place lists, e.g. "geoapify",
// "geoapify+osm", "mock"; "no results" when nothing came back.
func placesSource(lists ...[]travel.Place) string {
	sources := map[string]bool{}
	for _, list := range lists {
		for _, p := range list {
			sources[p.Source] = true
		}
	}
	if len(sources) == 0 {
		return "no results"
	}
	if len(sources) > 1 {
		return "mixed"
	}
	for source := range sources {
		return source
	}
	return "no results"
}

func titleCase(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func round1(v float64) float64 { return float64(int(v*10+0.5)) / 10 }
func round2(v float64) float64 { return float64(int(v*100+0.5)) / 100 }
