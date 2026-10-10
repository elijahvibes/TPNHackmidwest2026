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
	Origin              string         `json:"origin"`
	Destination         string         `json:"destination"`
	StartDate           string         `json:"start_date"` // YYYY-MM-DD
	EndDate             string         `json:"end_date"`
	Travelers           int            `json:"travelers"`
	BudgetUSD           float64        `json:"budget_usd"`
	Mode                string         `json:"mode"` // auto | drive | fly
	VehicleMPG          float64        `json:"vehicle_mpg"`
	Interests           []string       `json:"interests"`
	OriginLoc           *travel.LatLng `json:"origin_loc,omitempty"` // from browser geolocation or the autocomplete pick
	DestLoc             *travel.LatLng `json:"dest_loc,omitempty"`   // from the autocomplete pick
	LocalTransport      string         `json:"local_transport"`
	ActivityRadiusMiles float64        `json:"activity_radius_miles"`
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
	OriginPlace         travel.Placemark     `json:"origin_place"`             // what the origin resolved to ("Lawrence, KS")
	DestPlace           travel.Placemark     `json:"dest_place"`               // what the destination resolved to
	OriginAirport       *travel.Airport      `json:"origin_airport,omitempty"` // nearest airport used for fares
	DestAirport         *travel.Airport      `json:"dest_airport,omitempty"`
	Weather             travel.Weather       `json:"weather"`
	Safety              travel.Safety        `json:"safety"`
	AIGenerated         bool                 `json:"ai_generated"`
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
	Weather *travel.NWS // nil = mock forecast (tests)
}

func (p *Planner) Plan(ctx context.Context, req Request) (*Plan, error) {
	if err := normalizeMobility(&req); err != nil {
		return nil, err
	}
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
		return nil, fmt.Errorf("end_date must be after start_date for hotel searches")
	}
	if !p.SerpAPI.Enabled() {
		return nil, fmt.Errorf("%w: SERPAPI_KEY is required for flight and hotel searches", travel.ErrProvider)
	}

	// 1. Where are we going? Any city works: a pinned point from the form
	// wins, otherwise the name is geocoded (gazetteer → Mapbox → OpenStreetMap).
	originPlace, err := p.resolve(ctx, req.Origin, req.OriginLoc)
	if err != nil {
		return nil, fmt.Errorf("origin: %w", err)
	}
	destPlace, err := p.resolve(ctx, req.Destination, req.DestLoc)
	if err != nil {
		return nil, fmt.Errorf("destination: %w", err)
	}
	origin, dest := originPlace.LatLng, destPlace.LatLng

	// 2. Gather prices. Flight and hotel searches require the configured provider.
	miles, minutes, routeSrc := p.Geo.Route(ctx, origin, dest)
	drive := p.Fuel.Drive(ctx, miles, minutes, req.VehicleMPG)
	// Flights go from the nearest real airport to each place (SerpApi needs IATA
	// codes), so a town with no airport still gets a fare plus the drive to the terminal.
	var flights []travel.FlightQuote
	var originAirport, destAirport *travel.Airport
	if oa, ok := travel.NearestAirport(origin); ok {
		if da, ok := travel.NearestAirport(dest); ok && oa.Code != da.Code {
			originAirport, destAirport = &oa, &da
			flights, err = p.SerpAPI.Flights(ctx, oa.Code, da.Code, req.StartDate, req.EndDate, req.Travelers)
			if err != nil {
				return nil, fmt.Errorf("flight search: %w", err)
			}
		}
	}
	hotels, err := p.SerpAPI.Hotels(ctx, destPlace.Label, req.StartDate, req.EndDate, nights, req.Travelers)
	if err != nil {
		return nil, fmt.Errorf("hotel search: %w", err)
	}
	activities := p.activitiesFor(ctx, dest, req.Interests, req.ActivityRadiusMiles)
	food := p.Places.Explore(ctx, dest, "food", 6, req.ActivityRadiusMiles)
	weather := p.Weather.Forecast(ctx, dest, destPlace.Label, req.StartDate, req.EndDate)
	safety := travel.SafetyFor(ctx, destPlace.Name+", "+destPlace.Region)

	travel.RankFlights(flights)
	sort.Slice(hotels, func(i, j int) bool {
		// value = rating per dollar
		return hotels[i].Rating/hotels[i].NightlyUSD > hotels[j].Rating/hotels[j].NightlyUSD
	})

	// 3. Score the options.
	actCost := plannedPlaceCost(activities, food, dest, weather, start, nights, req.Travelers)

	// Hotel choice honors the budget: the best-rated stay that still fits after
	// the cheapest transport and activities; otherwise the cheapest stay.
	minTransport := drive.RoundTripUSD
	for _, f := range flights {
		if cost := f.PriceUSD * float64(req.Travelers); cost < minTransport {
			minTransport = cost
		}
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
		fmt.Sprintf("$%.0f per night priced subtotal (local transport excluded)", driveOpt.TotalUSD/float64(nights)),
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
		flyOpt.Why = fmt.Sprintf("%s %s→%s round trip from $%.0f/person, %s", f.Carrier, f.Origin, f.Destination, f.PriceUSD, stopsLabel(f.Stops))
		if originAirport != nil && destAirport != nil {
			// Door to door: the drive to and from each terminal counts too.
			flyOpt.TravelHours = round1(flyOpt.TravelHours + (originAirport.DistanceMiles+destAirport.DistanceMiles)*2/45)
		}
		flyOpt.Factors = []string{
			fmt.Sprintf("$%.0f per night priced subtotal (local transport excluded)", flyOpt.TotalUSD/float64(nights)),
			fmt.Sprintf("%.1f h door to door round trip incl. airport time", flyOpt.TravelHours),
			fmt.Sprintf("$%.0f/person × %d = $%.0f airfare", f.PriceUSD, req.Travelers, flyOpt.TransportUSD),
			hotelNote,
		}
		if originAirport != nil && originAirport.DistanceMiles > 20 {
			flyOpt.Factors = append(flyOpt.Factors, fmt.Sprintf("nearest airport %s is %.0f mi from %s", originAirport.Code, originAirport.DistanceMiles, originPlace.Name))
		}
		if destAirport != nil && destAirport.DistanceMiles > 20 {
			flyOpt.Factors = append(flyOpt.Factors, fmt.Sprintf("%s is %.0f mi from %s; add a rental or rideshare", destAirport.Code, destAirport.DistanceMiles, destPlace.Name))
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

	stops := make([]travel.Place, 0)
	if recommended.Mode == "drive" {
		seen := map[string]bool{}
		for _, pt := range travel.Interpolate(origin, dest, 2) {
			for _, s := range p.Places.Nearby(ctx, pt, "rest_stop", 2) {
				if !seen[s.Name] {
					seen[s.Name] = true
					stops = append(stops, s)
				}
			}
		}
	}

	plan := &Plan{
		Title:           fmt.Sprintf("%s → %s", originPlace.Label, destPlace.Label),
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
		OriginPlace:     originPlace,
		DestPlace:       destPlace,
		OriginAirport:   originAirport,
		DestAirport:     destAirport,
		Sources: map[string]string{
			"route": routeSrc, "gas": drive.Source, "geocode": destPlace.Source,
			"flights": "serpapi",
			"hotels":  "serpapi",
			"places":  placesSource(activities, food, stops),
		},
		Weather:     weather,
		Safety:      safety,
		GeneratedAt: time.Now(),
		Travelers:   req.Travelers, BudgetUSD: req.BudgetUSD,
		OriginName: req.Origin, DestinationName: req.Destination,
		LocalTransport: req.LocalTransport, ActivityRadiusMiles: req.ActivityRadiusMiles, Interests: req.Interests,
	}
	plan.Sources["weather"] = weather.Source
	plan.Sources["safety"] = "fbi-ucr"
	plan.Sources["local_transport"] = "estimate"

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
		o.TotalUSD = round2(o.TransportUSD + o.LodgingUSD + o.ActivitiesUSD)
		o.ValueScore = valueScore(*o, pl.Nights)
		for j := range o.Factors {
			switch {
			case strings.HasSuffix(o.Factors[j], "per night all-in"):
				o.Factors[j] = fmt.Sprintf("$%.0f per night all-in", o.TotalUSD/float64(pl.Nights))
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
		"origin_place": plan.OriginPlace.Label, "destination_place": plan.DestPlace.Label,
		"origin_airport": plan.OriginAirport, "destination_airport": plan.DestAirport,
		"drive": plan.Drive, "best_flight": first(plan.Flights), "best_hotel": first(plan.Hotels), "other_hotels": rest(plan.Hotels, 3),
		"activities": plan.Activities, "food": plan.Food, "stops": plan.Stops,
		"weather": plan.Weather, "safety": plan.Safety,
		"local_transport": effectiveTransport(plan), "activity_radius_miles": plan.ActivityRadiusMiles,
		"local_transport_constraint": "Local routes, times and costs are unverified. Flying has no road-trip fuel/rest stops. Driving brings an own car; never schedule rental pickup when recommended_mode is drive. Use only supplied places inside the activity radius. Group nearby activities by coordinates, treat places beyond 20 miles as single-activity excursion days, and keep departure morning within 3 miles. Never repeat an activity or claim transit service or rental availability. Flag additional transportation for walking activities beyond 3 miles.",
	})
	system := `You are TPN, a sharp, budget-savvy travel planner. You are given REAL price data; never invent prices, only use the numbers provided. Reply with a single JSON object: {"summary": string (2-3 sentences, mention total cost and why this mode is the best value), "days": [{"day": int, "date": "YYYY-MM-DD", "theme": string, "items": [{"time": "HH:MM", "title": string, "detail": string, "cost_usd": number, "category": "transport|lodging|activity|food|stop"}]}]}. Day 1 starts with travel from origin; the last day ends with travel home. When flying, name the airports (origin_airport/destination_airport codes) and include the drive to/from each terminal when it is over 20 miles. Day 1 includes a "lodging" item checking in to best_hotel by name with its total cost; mention its area. Use the activities, food and stops lists by name. Keep each day to 4-6 items. Use the weather: on days with precip_pct >= 50 schedule indoor activities and say why in the theme. Use the safety data: when level is "elevated", schedule arrival before dark, keep late-night items near the hotel, and note rideshare for anything after 22:00; mention one safety tip in the summary.`
	system += " Respect selected interests: do not add unrelated monuments or sightseeing. Use each restaurant at most once for dinner; if candidates run out, leave dinner open rather than repeat. Restaurant dinner hours and all activity opening hours are unverified: explicitly tell the traveler to confirm them. Preserve geographic variety across days and pair nearby activities within each day."
	system += " Activities within one mile of each other on the same day must be different types (use the provider categories). Never pair two nearby parks, museums or other same-type activities; if no suitable second activity exists, schedule just one. Use exact activity names."
	var out aiResponse
	if err := p.AI.ChatJSON(ctx, system, string(data), &out); err != nil {
		return err
	}
	days := sanitizeDays(out.Days, start, plan.Nights+1)
	if err := validateActivityVariety(days, plan.Activities); err != nil {
		return err
	}
	if plan.RecommendedMode == "fly" {
		for i := range days {
			items := make([]DayItem, 0, len(days[i].Items))
			for _, item := range days[i].Items {
				if item.Category != "stop" {
					items = append(items, item)
				}
			}
			days[i].Items = items
		}
	}
	if strings.TrimSpace(out.Summary) == "" || len(days) == 0 {
		return fmt.Errorf("ai returned empty plan")
	}
	plan.Summary = strings.TrimSpace(out.Summary)
	plan.Summary += mobilityNote(plan)
	plan.Days = days
	if effectiveTransport(plan) != "own car" {
		plan.Days[0].Items = append(plan.Days[0].Items, destinationTransfer(plan))
	}
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
	activitiesByDay := dailyActivities(plan.Activities, plan.DestLoc, plan.Weather, start, plan.Nights)
	mealsByDay := dailyMeals(plan.Food, plan.DestLoc, activitiesByDay, plan.Nights)
	for i := 0; i < total; i++ {
		d := Day{Day: i + 1, Date: start.AddDate(0, 0, i).Format("2006-01-02")}
		switch {
		case i == 0:
			d.Theme = "Travel day"
			if plan.RecommendedMode == "drive" {
				d.Items = append(d.Items, DayItem{Time: "08:00", Title: "Depart " + plan.OriginPlace.Name, Detail: fmt.Sprintf("%.0f mi, about %dh %dm", plan.Drive.DistanceMiles, plan.Drive.DurationMin/60, plan.Drive.DurationMin%60), CostUSD: plan.Drive.FuelCostUSD, Category: "transport"})
				for _, s := range plan.Stops[:min(2, len(plan.Stops))] {
					d.Items = append(d.Items, DayItem{Time: "11:30", Title: s.Name, Detail: s.Note, CostUSD: s.PriceUSD, Category: "stop"})
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
			if f := mealsByDay[i]; f != nil {
				d.Items = append(d.Items, DayItem{Time: "19:00", Title: "Dinner at " + f.Name, Detail: f.Note + "; dinner hours and availability unverified", CostUSD: f.PriceUSD * float64(req.Travelers), CostUnknown: f.PriceUnknown, Category: "food"})
			}
		case i == total-1:
			d.Theme = "Head home"
			if len(activitiesByDay[i]) > 0 {
				a := activitiesByDay[i][0]
				d.Items = append(d.Items, DayItem{Time: "09:00", Title: a.Name, Detail: a.Note + " · " + localTravelDetail(plan, a), CostUSD: a.PriceUSD * float64(req.Travelers), Category: "activity"})
			}
			if plan.RecommendedMode == "drive" {
				d.Items = append(d.Items, DayItem{Time: "12:00", Title: "Drive back to " + plan.OriginPlace.Name, Detail: "fuel for the return leg", CostUSD: plan.Drive.FuelCostUSD, Category: "transport"})
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
			if f := mealsByDay[i]; f != nil {
				d.Items = append(d.Items, DayItem{Time: "18:30", Title: "Dinner at " + f.Name, Detail: f.Note + "; dinner hours and availability unverified", CostUSD: f.PriceUSD * float64(req.Travelers), CostUnknown: f.PriceUnknown, Category: "food"})
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
