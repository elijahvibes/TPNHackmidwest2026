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
	"time"

	"trailmix/internal/ai"
	"trailmix/internal/travel"
)

type Request struct {
	Origin      string         `json:"origin"`
	Destination string         `json:"destination"`
	StartDate   string         `json:"start_date"` // YYYY-MM-DD
	EndDate     string         `json:"end_date"`
	Travelers   int            `json:"travelers"`
	BudgetUSD   float64        `json:"budget_usd"`
	Mode        string         `json:"mode"` // auto | drive | fly
	VehicleMPG  float64        `json:"vehicle_mpg"`
	Interests   []string       `json:"interests"`
	OriginLoc   *travel.LatLng `json:"origin_loc,omitempty"` // from browser geolocation
}

type Option struct {
	Mode          string   `json:"mode"`
	TransportUSD  float64  `json:"transport_usd"`
	LodgingUSD    float64  `json:"lodging_usd"`
	ActivitiesUSD float64  `json:"activities_usd"`
	TotalUSD      float64  `json:"total_usd"`
	TravelHours   float64  `json:"travel_hours"`
	ValueScore    float64  `json:"value_score"` // higher is better
	Why           string   `json:"why"`
	Factors       []string `json:"factors"` // human-readable breakdown of the score
}

type DayItem struct {
	Time     string  `json:"time"`
	Title    string  `json:"title"`
	Detail   string  `json:"detail,omitempty"`
	CostUSD  float64 `json:"cost_usd"`
	Category string  `json:"category"`
}

type Day struct {
	Day   int       `json:"day"`
	Date  string    `json:"date"`
	Theme string    `json:"theme"`
	Items []DayItem `json:"items"`
}

type Plan struct {
	Title           string               `json:"title"`
	Summary         string               `json:"summary"`
	RecommendedMode string               `json:"recommended_mode"`
	Options         []Option             `json:"options"`
	Drive           travel.DriveEstimate `json:"drive"`
	Flights         []travel.FlightQuote `json:"flights"`
	Hotels          []travel.HotelQuote  `json:"hotels"`
	Stops           []travel.Place       `json:"stops"`
	Activities      []travel.Place       `json:"activities"`
	Food            []travel.Place       `json:"food"`
	Days            []Day                `json:"days"`
	TotalUSD        float64              `json:"total_usd"`
	UnderBudget     bool                 `json:"under_budget"`
	Nights          int                  `json:"nights"`
	OriginLoc       travel.LatLng        `json:"origin_loc"`
	DestLoc         travel.LatLng        `json:"dest_loc"`
	Weather         travel.Weather       `json:"weather"`
	Safety          travel.Safety        `json:"safety"`
	AIGenerated     bool                 `json:"ai_generated"`
	Sources         map[string]string    `json:"sources"`
	GeneratedAt     time.Time            `json:"generated_at"`
}

type Planner struct {
	AI      *ai.Client
	Geo     *travel.Geo
	Fuel    *travel.Fuel
	Amadeus *travel.Amadeus
	Places  *travel.Places
	Weather *travel.NWS // nil = mock forecast (tests)
}

func (p *Planner) Plan(ctx context.Context, req Request) (*Plan, error) {
	if req.Travelers < 1 {
		req.Travelers = 1
	}
	start, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, fmt.Errorf("bad start_date")
	}
	end, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil || end.Before(start) {
		return nil, fmt.Errorf("bad end_date")
	}
	nights := int(end.Sub(start).Hours() / 24)
	if nights < 1 {
		nights = 1
	}

	// 1. Where are we going?
	var origin travel.LatLng
	if req.OriginLoc != nil {
		origin = *req.OriginLoc
	} else if origin, err = p.Geo.Geocode(ctx, req.Origin); err != nil {
		return nil, err
	}
	dest, err := p.Geo.Geocode(ctx, req.Destination)
	if err != nil {
		return nil, err
	}

	// 2. Gather prices. Each call tolerates missing keys.
	miles, minutes, routeSrc := p.Geo.Route(ctx, origin, dest)
	drive := p.Fuel.Drive(ctx, miles, minutes, req.VehicleMPG)
	flights := p.Amadeus.Flights(ctx, travel.IATA(req.Origin), travel.IATA(req.Destination), req.StartDate, req.EndDate, req.Travelers)
	hotels := p.Amadeus.Hotels(ctx, dest, req.StartDate, req.EndDate, nights, req.Destination)
	activities := p.activitiesFor(ctx, dest, req.Interests)
	food := p.Places.Nearby(ctx, dest, "food", 4)
	var stops []travel.Place
	seen := map[string]bool{}
	for _, pt := range travel.Interpolate(origin, dest, 2) {
		for _, s := range p.Places.Nearby(ctx, pt, "rest_stop", 2) {
			if !seen[s.Name] {
				seen[s.Name] = true
				stops = append(stops, s)
			}
		}
	}

	weather := p.Weather.Forecast(ctx, dest, titleCase(req.Destination), req.StartDate, req.EndDate)
	safety := travel.SafetyFor(ctx, req.Destination)

	sort.Slice(flights, func(i, j int) bool { return flights[i].PriceUSD < flights[j].PriceUSD })
	sort.Slice(hotels, func(i, j int) bool {
		// value = rating per dollar
		return hotels[i].Rating/hotels[i].NightlyUSD > hotels[j].Rating/hotels[j].NightlyUSD
	})

	// 3. Score the options.
	actCost := 0.0
	for _, a := range activities[:min(4, len(activities))] {
		actCost += a.PriceUSD * float64(req.Travelers)
	}
	for _, f := range food[:min(3, len(food))] {
		actCost += f.PriceUSD * float64(req.Travelers) * float64(nights)
	}
	actCost = round2(actCost)

	// Hotel choice honors the budget: the best-rated stay that still fits after
	// the cheapest transport and activities; otherwise the cheapest stay.
	minTransport := drive.RoundTripUSD
	if len(flights) > 0 && flights[0].PriceUSD*float64(req.Travelers) < minTransport {
		minTransport = flights[0].PriceUSD * float64(req.Travelers)
	}
	hotels = pickHotel(hotels, req.BudgetUSD-minTransport-actCost, safety.Level == "elevated")
	lodging := 0.0
	hotelNote := "no stay found"
	if len(hotels) > 0 {
		lodging = hotels[0].TotalUSD
		hotelNote = fmt.Sprintf("%s ★%.1f at $%.0f/night", hotels[0].Name, hotels[0].Rating, hotels[0].NightlyUSD)
	}

	driveOpt := Option{
		Mode: "drive", TransportUSD: drive.RoundTripUSD, LodgingUSD: lodging, ActivitiesUSD: actCost,
		TravelHours: round1(float64(drive.DurationMin) * 2 / 60),
	}
	driveOpt.TotalUSD = round2(driveOpt.TransportUSD + lodging + actCost)
	driveOpt.Why = fmt.Sprintf("%.0f mi each way, %.1f gal of gas at $%.2f", drive.DistanceMiles, drive.GallonsNeeded, drive.GasPriceUSD)
	driveOpt.Factors = []string{
		fmt.Sprintf("$%.0f per night all-in", driveOpt.TotalUSD/float64(nights)),
		fmt.Sprintf("%.1f h on the road round trip", driveOpt.TravelHours),
		fmt.Sprintf("gas $%.0f for %d traveler(s), no per-seat cost", drive.RoundTripUSD, req.Travelers),
		hotelNote,
	}

	options := []Option{driveOpt}
	if len(flights) > 0 {
		f := flights[0]
		flyOpt := Option{
			Mode: "fly", TransportUSD: round2(f.PriceUSD * float64(req.Travelers)), LodgingUSD: lodging, ActivitiesUSD: actCost,
			TravelHours: round1(float64(f.DurationMin+150) * 2 / 60), // + airport time
		}
		flyOpt.TotalUSD = round2(flyOpt.TransportUSD + lodging + actCost)
		flyOpt.Why = fmt.Sprintf("%s round trip from $%.0f/person, %d stop(s)", f.Carrier, f.PriceUSD, f.Stops)
		flyOpt.Factors = []string{
			fmt.Sprintf("$%.0f per night all-in", flyOpt.TotalUSD/float64(nights)),
			fmt.Sprintf("%.1f h door to door round trip incl. airport time", flyOpt.TravelHours),
			fmt.Sprintf("$%.0f/person × %d = $%.0f airfare", f.PriceUSD, req.Travelers, flyOpt.TransportUSD),
			hotelNote,
		}
		options = append(options, flyOpt)
	}
	for i := range options {
		options[i].ValueScore = valueScore(options[i], nights)
	}
	sort.Slice(options, func(i, j int) bool { return options[i].ValueScore > options[j].ValueScore })
	recommended := options[0]
	// Best value that still fits the budget beats best value overall.
	if req.BudgetUSD > 0 && recommended.TotalUSD > req.BudgetUSD {
		for _, o := range options {
			if o.TotalUSD <= req.BudgetUSD {
				recommended = o
				break
			}
		}
	}
	if req.Mode == "drive" || req.Mode == "fly" {
		for _, o := range options {
			if o.Mode == req.Mode {
				recommended = o
			}
		}
	}

	plan := &Plan{
		Title:           fmt.Sprintf("%s → %s", titleCase(req.Origin), titleCase(req.Destination)),
		RecommendedMode: recommended.Mode,
		Options:         options,
		Drive:           drive,
		Flights:         flights,
		Hotels:          hotels,
		Stops:           stops,
		Activities:      activities,
		Food:            food,
		TotalUSD:        recommended.TotalUSD,
		UnderBudget:     req.BudgetUSD <= 0 || recommended.TotalUSD <= req.BudgetUSD,
		Nights:          nights,
		OriginLoc:       origin,
		DestLoc:         dest,
		Sources: map[string]string{
			"route": routeSrc, "gas": drive.Source,
			"flights": sourceOf(len(flights) > 0 && flights[0].Source == "amadeus", "amadeus"),
			"hotels":  sourceOf(len(hotels) > 0 && hotels[0].Source == "amadeus", "amadeus"),
			"places":  sourceOf(len(activities) > 0 && activities[0].Source == "google", "google"),
		},
		Weather:     weather,
		Safety:      safety,
		GeneratedAt: time.Now(),
	}
	plan.Sources["weather"] = weather.Source
	plan.Sources["safety"] = "fbi-ucr"

	// 4. Let the model write the narrative + daily schedule around the numbers.
	if p.AI.Enabled() {
		if err := p.narrate(ctx, req, plan, start); err != nil {
			log.Printf("planner: ai narration failed, using fallback: %v", err)
		} else {
			plan.AIGenerated = true
			plan.Sources["ai"] = "akamai"
		}
	}
	if !plan.AIGenerated {
		plan.Summary, plan.Days = fallbackItinerary(req, plan, start)
	}
	return plan, nil
}

// activitiesFor returns things to do shaped by the user's interests: a few per
// chosen interest (deduped, interleaved so no single interest dominates),
// topped up with general attractions. No interests → general attractions.
func (p *Planner) activitiesFor(ctx context.Context, dest travel.LatLng, interests []string) []travel.Place {
	const want = 6
	var lists [][]travel.Place
	for _, in := range interests {
		if in == "food" { // food is handled by its own section
			continue
		}
		if _, ok := map[string]bool{"outdoors": true, "museums": true, "nightlife": true, "family": true, "history": true}[in]; ok {
			lists = append(lists, p.Places.Nearby(ctx, dest, in, 3))
		}
	}
	lists = append(lists, p.Places.Nearby(ctx, dest, "attraction", want))
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

type aiResponse struct {
	Summary string `json:"summary"`
	Days    []Day  `json:"days"`
}

func (p *Planner) narrate(ctx context.Context, req Request, plan *Plan, start time.Time) error {
	data, _ := json.Marshal(map[string]any{
		"origin": req.Origin, "destination": req.Destination,
		"start_date": req.StartDate, "end_date": req.EndDate, "nights": plan.Nights,
		"travelers": req.Travelers, "budget_usd": req.BudgetUSD, "interests": req.Interests,
		"recommended_mode": plan.RecommendedMode, "options": plan.Options,
		"drive": plan.Drive, "best_flight": first(plan.Flights), "best_hotel": first(plan.Hotels),
		"activities": plan.Activities, "food": plan.Food, "stops": plan.Stops,
		"weather": plan.Weather, "safety": plan.Safety,
	})
	system := `You are TPN, a sharp, budget-savvy travel planner. You are given REAL price data; never invent prices, only use the numbers provided. Reply with a single JSON object: {"summary": string (2-3 sentences, mention total cost and why this mode is the best value), "days": [{"day": int, "date": "YYYY-MM-DD", "theme": string, "items": [{"time": "HH:MM", "title": string, "detail": string, "cost_usd": number, "category": "transport|lodging|activity|food|stop"}]}]}. Day 1 starts with travel from origin; the last day ends with travel home. Use the activities, food and stops lists by name. Keep each day to 4-6 items. Use the weather: on days with precip_pct >= 50 schedule indoor activities and say why in the theme. Use the safety data: when level is "elevated", schedule arrival before dark, keep late-night items near the hotel, and note rideshare for anything after 22:00; mention one safety tip in the summary.`
	var out aiResponse
	if err := p.AI.ChatJSON(ctx, system, string(data), &out); err != nil {
		return err
	}
	days := sanitizeDays(out.Days, start, plan.Nights+1)
	if strings.TrimSpace(out.Summary) == "" || len(days) == 0 {
		return fmt.Errorf("ai returned empty plan")
	}
	plan.Summary = strings.TrimSpace(out.Summary)
	plan.Days = days
	return nil
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
	for i := 0; i < total; i++ {
		d := Day{Day: i + 1, Date: start.AddDate(0, 0, i).Format("2006-01-02")}
		switch {
		case i == 0:
			d.Theme = "Travel day"
			if plan.RecommendedMode == "drive" {
				d.Items = append(d.Items, DayItem{Time: "08:00", Title: "Depart " + titleCase(req.Origin), Detail: fmt.Sprintf("%.0f mi, about %dh %dm", plan.Drive.DistanceMiles, plan.Drive.DurationMin/60, plan.Drive.DurationMin%60), CostUSD: plan.Drive.FuelCostUSD, Category: "transport"})
				for _, s := range plan.Stops[:min(2, len(plan.Stops))] {
					d.Items = append(d.Items, DayItem{Time: "11:30", Title: s.Name, Detail: s.Note, CostUSD: s.PriceUSD, Category: "stop"})
				}
			} else if f := first(plan.Flights); f != nil {
				d.Items = append(d.Items, DayItem{Time: "09:00", Title: fmt.Sprintf("Fly %s → %s on %s", f.Origin, f.Destination, f.Carrier), Detail: fmt.Sprintf("round trip, %d traveler(s)", req.Travelers), CostUSD: f.PriceUSD * float64(req.Travelers), Category: "transport"})
			}
			if h := first(plan.Hotels); h != nil {
				d.Items = append(d.Items, DayItem{Time: "16:00", Title: "Check in: " + h.Name, Detail: fmt.Sprintf("%d nights at $%.0f/night", h.Nights, h.NightlyUSD), CostUSD: h.TotalUSD, Category: "lodging"})
			}
			if f := first(plan.Food); f != nil {
				d.Items = append(d.Items, DayItem{Time: "19:00", Title: "Dinner at " + f.Name, Detail: f.Note, CostUSD: f.PriceUSD * float64(req.Travelers), Category: "food"})
			}
		case i == total-1:
			d.Theme = "Head home"
			// Pick the first activity the middle days did not already use.
			if used := (total - 2) * 2; used < len(plan.Activities) {
				a := plan.Activities[used]
				d.Items = append(d.Items, DayItem{Time: "09:00", Title: a.Name, Detail: a.Note, CostUSD: a.PriceUSD * float64(req.Travelers), Category: "activity"})
			}
			if plan.RecommendedMode == "drive" {
				d.Items = append(d.Items, DayItem{Time: "12:00", Title: "Drive back to " + titleCase(req.Origin), Detail: "fuel for the return leg", CostUSD: plan.Drive.FuelCostUSD, Category: "transport"})
			} else {
				d.Items = append(d.Items, DayItem{Time: "12:00", Title: "Fly home to " + titleCase(req.Origin), Detail: "return flight already included", CostUSD: 0, Category: "transport"})
			}
		default:
			d.Theme = "Explore " + titleCase(req.Destination)
			acts := plan.Activities
			if w := weatherOn(plan.Weather, d.Date); w != nil && w.PrecipPct >= 50 {
				d.Theme = fmt.Sprintf("Indoor day (%s, %d%% chance of rain)", strings.ToLower(w.Short), w.PrecipPct)
				acts = indoorFirst(acts)
			}
			idx := (i - 1) * 2
			for j := idx; j < idx+2 && j < len(acts); j++ {
				a := acts[j]
				d.Items = append(d.Items, DayItem{Time: []string{"10:00", "14:00"}[j-idx], Title: a.Name, Detail: a.Note, CostUSD: a.PriceUSD * float64(req.Travelers), Category: "activity"})
			}
			if plan.Safety.Level == "elevated" && len(plan.Activities) > 0 {
				// Late items get a safety nudge; the AI path gets the same instruction in its prompt.
				d.Items = append(d.Items, DayItem{Time: "21:30", Title: "Evening out near the hotel", Detail: "rideshare back after 10pm (safety: elevated)", CostUSD: 0, Category: "activity"})
			}
			if len(plan.Food) > i%len(plan.Food) {
				f := plan.Food[i%len(plan.Food)]
				d.Items = append(d.Items, DayItem{Time: "18:30", Title: "Dinner at " + f.Name, Detail: f.Note, CostUSD: f.PriceUSD * float64(req.Travelers), Category: "food"})
			}
		}
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
		extra += fmt.Sprintf(" Weather on arrival: %s, high %d°F.", strings.ToLower(w.Short), w.HighF)
	}
	if plan.Safety.Level == "elevated" && len(plan.Safety.Tips) > 0 {
		extra += " Safety: " + plan.Safety.Tips[0]
	}
	summary := fmt.Sprintf("%s to %s for %d traveler(s), %d night(s). Best value is to %s: about $%.0f all-in (%s).%s%s",
		titleCase(req.Origin), titleCase(req.Destination), req.Travelers, plan.Nights, plan.RecommendedMode, plan.TotalUSD, rec.Why, fits, extra)
	return summary, days
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

func first[T any](s []T) *T {
	if len(s) == 0 {
		return nil
	}
	return &s[0]
}

func sourceOf(live bool, name string) string {
	if live {
		return name
	}
	return "mock"
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
