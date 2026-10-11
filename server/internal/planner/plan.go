package planner

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"trailmix/internal/agents"
	"trailmix/internal/travel"
)

// Event is a planning milestone. The API relays these to the browser so a
// rule-based draft appears within seconds and the AI's work streams in behind
// it instead of a blank two-minute wait.
type Event struct {
	Stage   string         `json:"stage"` // search | draft | agents | writer | review | done
	Message string         `json:"message"`
	Plan    *Plan          `json:"plan,omitempty"`   // draft (stage=draft/agents) or final (stage=done)
	Report  *agents.Report `json:"report,omitempty"` // one agent's result
}

// Progress receives Events; nil is fine.
type Progress func(Event)

// gathered is everything the providers returned for one request.
type gathered struct {
	req                        Request
	start                      time.Time
	nights                     int
	originPlace, destPlace     travel.Placemark
	routeSrc                   string
	drive                      travel.DriveEstimate
	flights                    []travel.FlightQuote
	originAirport, destAirport *travel.Airport
	hotels                     []travel.HotelQuote
	weather                    travel.Weather
	safety                     travel.Safety
}

func (p *Planner) Plan(ctx context.Context, req Request) (*Plan, error) {
	emit := req.Progress
	if emit == nil {
		emit = func(Event) {}
	}
	req.Progress = nil
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
	stage := time.Now()
	timing := func(name string) {
		log.Printf("planner: %s took %.1fs", name, time.Since(stage).Seconds())
		stage = time.Now()
	}

	// 1. Where are we going? Any city works: a pinned point from the form
	// wins, otherwise the name is geocoded (gazetteer → Mapbox → OpenStreetMap).
	emit(Event{Stage: "search", Message: "locating origin and destination"})
	g := gathered{req: req, start: start, nights: nights}
	if g.originPlace, err = p.resolve(ctx, req.Origin, req.OriginLoc); err != nil {
		return nil, fmt.Errorf("origin: %w", err)
	}
	if g.destPlace, err = p.resolve(ctx, req.Destination, req.DestLoc); err != nil {
		return nil, fmt.Errorf("destination: %w", err)
	}
	origin, dest := g.originPlace.LatLng, g.destPlace.LatLng

	// 2. Gather prices. Flights and hotels come from SerpApi; the rest from
	// free sources (OSRM, EIA, Geoapify/OSM, NWS, FBI). Independent calls run
	// concurrently so the draft is ready in a few seconds.
	emit(Event{Stage: "search", Message: "routing the drive and pricing gas"})
	var miles float64
	var minutes int
	miles, minutes, g.routeSrc = p.Geo.Route(ctx, origin, dest)
	g.drive = p.Fuel.Drive(ctx, miles, minutes, req.VehicleMPG, req.TankGallons)

	var wg sync.WaitGroup
	var flightErr, hotelErr error
	var activities, food, breakfast, stops []travel.Place
	wg.Add(1)
	go func() { // flights go airport to airport, so a town without one still gets a fare
		defer wg.Done()
		if oa, ok := travel.NearestAirport(origin); ok {
			if da, ok := travel.NearestAirport(dest); ok && oa.Code != da.Code {
				g.originAirport, g.destAirport = &oa, &da
				g.flights, flightErr = p.SerpAPI.Flights(ctx, oa.Code, da.Code, req.StartDate, req.EndDate, req.Travelers)
			}
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		g.hotels, hotelErr = p.SerpAPI.Hotels(ctx, g.destPlace.Label, req.StartDate, req.EndDate, nights, req.Travelers)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		activities = p.activitiesFor(ctx, dest, req.Interests, req.ActivityRadiusMiles)
		// Wikivoyage's curated listings (with published prices) join the
		// provider candidates, and Wikipedia readership ranks them all, so the
		// Curator sees Red Rocks ahead of a pocket park and can prefer it.
		activities = travel.MergeNotable(activities, p.Wiki.Listings(ctx, g.destPlace.Name, dest, req.ActivityRadiusMiles, "attraction", 15))
		if views := p.Wiki.Popularity(ctx, placeNames(activities)); len(views) > 0 {
			for i := range activities {
				if v := views[activities[i].Name]; activities[i].Popularity == 0 && v > 0 {
					activities[i].Popularity = v
				}
			}
			sort.SliceStable(activities, func(i, j int) bool { return activities[i].Popularity > activities[j].Popularity })
		}
		food = travel.MergeNotable(p.Places.Explore(ctx, dest, "food", 14, req.ActivityRadiusMiles),
			p.Wiki.Listings(ctx, g.destPlace.Name, dest, req.ActivityRadiusMiles, "food", 8))
		// Breakfast spots (cafés, bakeries) are a separate list so every day
		// can start with one; they share plan.Food with Category "breakfast".
		breakfast = p.Places.Explore(ctx, dest, "breakfast", 8, req.ActivityRadiusMiles)
		for i := range breakfast {
			breakfast[i].Category = "breakfast"
		}
		// Road stops sit where the tank runs low (one per planned refuel, on
		// the driven road), or at the halfway point when one tank covers it.
		stops = p.refuelStops(ctx, origin, dest, g.drive)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		g.weather = p.Weather.Forecast(ctx, dest, g.destPlace.Label, req.StartDate, req.EndDate)
		g.safety = travel.SafetyFor(ctx, g.destPlace.Name+", "+g.destPlace.Region)
	}()
	emit(Event{Stage: "search", Message: "searching flights, stays, places, weather and safety"})
	wg.Wait()
	if flightErr != nil {
		return nil, fmt.Errorf("flight search: %w", flightErr)
	}
	if hotelErr != nil {
		return nil, fmt.Errorf("hotel search: %w", hotelErr)
	}
	travel.RankFlights(g.flights)
	sort.Slice(g.hotels, func(i, j int) bool { // value = rating per dollar
		return g.hotels[i].Rating/g.hotels[i].NightlyUSD > g.hotels[j].Rating/g.hotels[j].NightlyUSD
	})
	// Places without a provider price get a typical-price estimate, marked as such.
	travel.FillPriceEstimates(activities)
	travel.FillPriceEstimates(food)
	travel.FillPriceEstimates(breakfast)
	travel.FillPriceEstimates(stops)
	timing("search (route, gas, flights, hotels, places, weather, safety)")

	// 3. Draft: scored options and a rule-based itinerary, available right away.
	plan := p.assemble(g, activities, append(append([]travel.Place{}, food...), breakfast...), stops)
	plan.Summary, plan.Days = fallbackItinerary(req, plan, start)
	emit(Event{Stage: "draft", Message: "draft plan ready; AI agents are refining it", Plan: plan})
	if !p.AI.Enabled() {
		emit(Event{Stage: "done", Message: "plan ready (AI offline, rule-based itinerary)", Plan: plan})
		return plan, nil
	}

	// 4. Agents: the Curator filters the real candidates down to the ones
	// worth a traveler's time while the Researcher reads up on the destination.
	trip := agents.Trip{
		Destination: g.destPlace.Label, StartDate: req.StartDate, EndDate: req.EndDate,
		Travelers: req.Travelers, Interests: req.Interests, LocalTransport: req.LocalTransport,
		RadiusMiles: req.ActivityRadiusMiles, Weather: weatherLine(g.weather), SafetyLevel: g.safety.Level, Center: dest,
	}
	emit(Event{Stage: "agents", Message: "Curator is filtering places; Researcher is reading up on " + g.destPlace.Name})
	var research *agents.Research
	var researchRep agents.Report
	var rwg sync.WaitGroup
	rwg.Add(1)
	go func() {
		defer rwg.Done()
		research, researchRep = agents.DoResearch(ctx, p.AI, trip, append(append([]travel.Place{}, activities...), food...))
	}()
	lists := map[string][]travel.Place{"activities": activities, "food": food, "breakfast": breakfast}
	want := map[string]int{"activities": 8, "food": 2 + 2*plan.Nights, "breakfast": max(2, plan.Nights)} // lunch + dinner per day, a breakfast per morning
	if plan.RecommendedMode == "drive" {
		lists["stops"], want["stops"] = stops, 4
	}
	curated, reports := agents.CurateAll(ctx, p.AI, trip, lists, want)
	for i := range reports {
		emit(Event{Stage: "agents", Message: reports[i].Summary, Report: &reports[i]})
	}
	activities, food, breakfast = curated["activities"], curated["food"], curated["breakfast"]
	if s, ok := curated["stops"]; ok {
		stops = s
	}
	food = append(append([]travel.Place{}, food...), breakfast...)
	rwg.Wait()
	reports = append(reports, researchRep)
	emit(Event{Stage: "agents", Message: researchRep.Summary, Report: &researchRep})
	if research != nil {
		keepPlaceNotes(research, activities, food)
	}
	timing("curator + researcher")

	// Re-score with the curated places; the itinerary is still the rule-based
	// one until the Writer finishes, so the page keeps showing something real.
	plan = p.assemble(g, activities, food, stops)
	plan.Research, plan.Agents = research, reports
	if research != nil {
		plan.Sources["research"] = "akamai"
	}
	plan.Summary, plan.Days = fallbackItinerary(req, plan, start)
	emit(Event{Stage: "agents", Message: "places curated; Writer is drafting the itinerary", Plan: plan})

	// 5. The Writer drafts the narrative + daily schedule around the numbers,
	// the Reviewer audits it against the real data.
	if err := p.narrate(ctx, req, plan, start, emit); err != nil {
		log.Printf("planner: ai narration failed, using fallback: %v", err)
		rep := agents.Report{Agent: "writer", Status: "failed", Summary: "itinerary fell back to rules: " + err.Error()}
		plan.Agents = append(plan.Agents, rep)
		emit(Event{Stage: "writer", Message: rep.Summary, Report: &rep})
	} else {
		plan.AIGenerated = true
		plan.Sources["ai"] = "akamai"
		emit(Event{Stage: "review", Message: "itinerary written; Reviewer is checking it against the real data"})
		reviews := p.review(ctx, trip, plan)
		plan.Agents = append(plan.Agents, reviews...)
		for i := range reviews {
			emit(Event{Stage: "review", Message: reviews[i].Summary, Report: &reviews[i]})
		}
	}
	timing("writer + reviewer")
	emit(Event{Stage: "done", Message: "plan ready", Plan: plan})
	return plan, nil
}

// assemble turns gathered provider data plus place lists into a scored Plan
// (without an itinerary). It is pure, so it runs once for the draft and again
// after the Curator trims the lists.
func (p *Planner) assemble(g gathered, activities, food, stops []travel.Place) *Plan {
	req, nights := g.req, g.nights
	drive, flights := g.drive, g.flights
	hotels := append([]travel.HotelQuote(nil), g.hotels...)

	// Places and meals cost a little differently per mode: an early flight
	// arrival earns a lunch on day 1 that a ten-hour drive does not.
	costFor := func(mode string) float64 {
		probe := &Plan{RecommendedMode: mode, Drive: drive, Flights: flights, OriginAirport: g.originAirport, DestAirport: g.destAirport}
		return plannedPlaceCost(activities, food, g.destPlace.LatLng, g.weather, g.start, nights, req.Travelers, arrivalMinutes(probe, min(2, len(stops))))
	}
	actCostDrive, actCostFly := costFor("drive"), costFor("fly")
	actCost := min(actCostDrive, actCostFly)

	// Hotel choice honors the budget: the best-rated stay that still fits after
	// the cheapest transport and activities; otherwise the cheapest stay.
	minTransport := drive.RoundTripUSD
	for _, f := range flights {
		if cost := f.PriceUSD * float64(req.Travelers); cost < minTransport {
			minTransport = cost
		}
	}
	hotels = pickHotel(hotels, req.BudgetUSD-minTransport-actCost, g.safety.Level == "elevated")
	actCost = actCostDrive
	lodging := 0.0
	hotelNote := "no stay found"
	if len(hotels) > 0 {
		lodging = hotels[0].TotalUSD
		hotelNote = fmt.Sprintf("%s ★%.1f at $%.0f/night", hotels[0].Name, hotels[0].Rating, hotels[0].NightlyUSD)
	}
	days := nights + 1

	// Getting around at the destination is estimated per mode so the total is
	// a real all-in number rather than a subtotal with a footnote.
	localFor := func(mode string) (float64, string) {
		perDay, note := travel.LocalTransportEstimate(transportFor(mode, req.LocalTransport), req.Travelers, days, mode == "fly")
		return round2(perDay * float64(days)), note
	}

	driveLocal, driveLocalNote := localFor("drive")
	driveOpt := Option{
		Mode: "drive", TransportUSD: drive.RoundTripUSD, LodgingUSD: lodging, ActivitiesUSD: actCost, LocalUSD: driveLocal,
		TravelHours: round1(float64(drive.DurationMin) * 2 / 60),
	}
	driveOpt.TotalUSD = round2(driveOpt.TransportUSD + lodging + actCost + driveLocal)
	driveOpt.Why = fmt.Sprintf("%.0f mi each way, %.1f gal of gas at $%.2f: a full tank ($%.0f) plus %d refuel(s) per direction", drive.DistanceMiles, drive.GallonsNeeded, drive.GasPriceUSD, drive.StartTankUSD, len(drive.Refuels))
	driveOpt.Factors = []string{
		fmt.Sprintf("$%.0f per night all-in (estimates included)", driveOpt.TotalUSD/float64(nights)),
		fmt.Sprintf("%.1f h on the road round trip", driveOpt.TravelHours),
		fmt.Sprintf("gas $%.0f for %d traveler(s), no per-seat cost", drive.RoundTripUSD, req.Travelers),
		hotelNote,
		fmt.Sprintf("getting around ~$%.0f: %s (estimate)", driveLocal, driveLocalNote),
	}

	options := []Option{driveOpt}
	if len(flights) > 0 {
		f := flights[0]
		flyLocal, flyLocalNote := localFor("fly")
		flyOpt := Option{
			Mode: "fly", TransportUSD: round2(f.PriceUSD * float64(req.Travelers)), LodgingUSD: lodging, ActivitiesUSD: actCostFly, LocalUSD: flyLocal,
			TravelHours: round1(float64(f.DurationMin+150) * 2 / 60), // + airport time
		}
		flyOpt.TotalUSD = round2(flyOpt.TransportUSD + lodging + actCostFly + flyLocal)
		flyOpt.Why = fmt.Sprintf("%s %s→%s round trip from $%.0f/person, %s", f.Carrier, f.Origin, f.Destination, f.PriceUSD, stopsLabel(f.Stops))
		if g.originAirport != nil && g.destAirport != nil {
			// Door to door: the drive to and from each terminal counts too.
			flyOpt.TravelHours = round1(flyOpt.TravelHours + (g.originAirport.DistanceMiles+g.destAirport.DistanceMiles)*2/45)
		}
		flyOpt.Factors = []string{
			fmt.Sprintf("$%.0f per night all-in (estimates included)", flyOpt.TotalUSD/float64(nights)),
			fmt.Sprintf("%.1f h door to door round trip incl. airport time", flyOpt.TravelHours),
			fmt.Sprintf("$%.0f/person × %d = $%.0f airfare", f.PriceUSD, req.Travelers, flyOpt.TransportUSD),
			hotelNote,
			fmt.Sprintf("getting around ~$%.0f: %s (estimate)", flyLocal, flyLocalNote),
		}
		if g.originAirport != nil && g.originAirport.DistanceMiles > 20 {
			flyOpt.Factors = append(flyOpt.Factors, fmt.Sprintf("nearest airport %s is %.0f mi from %s", g.originAirport.Code, g.originAirport.DistanceMiles, g.originPlace.Name))
		}
		if g.destAirport != nil && g.destAirport.DistanceMiles > 20 {
			flyOpt.Factors = append(flyOpt.Factors, fmt.Sprintf("%s is %.0f mi from %s; add a rental or rideshare", g.destAirport.Code, g.destAirport.DistanceMiles, g.destPlace.Name))
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
	if recommended.Mode == "fly" {
		stops = []travel.Place{}
	}
	if stops == nil {
		stops = []travel.Place{}
	}

	safetySrc := "fbi-ucr"
	if strings.Contains(g.safety.Source, "fbi-cde") {
		safetySrc = "fbi-cde"
	}
	plan := &Plan{
		Title:           fmt.Sprintf("%s → %s", g.originPlace.Label, g.destPlace.Label),
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
		OriginLoc:       g.originPlace.LatLng,
		DestLoc:         g.destPlace.LatLng,
		RouteLine:       p.Geo.RouteLine(g.originPlace.LatLng, g.destPlace.LatLng, 400),
		OriginPlace:     g.originPlace,
		DestPlace:       g.destPlace,
		OriginAirport:   g.originAirport,
		DestAirport:     g.destAirport,
		Sources: map[string]string{
			"route": g.routeSrc, "gas": drive.Source, "geocode": g.destPlace.Source,
			"flights": "serpapi", "hotels": "serpapi",
			"places":  placesSource(activities, food, stops),
			"weather": g.weather.Source, "safety": safetySrc,
			"local_transport": "estimate",
		},
		Weather:     g.weather,
		Safety:      g.safety,
		GeneratedAt: time.Now(),
		Travelers:   req.Travelers, BudgetUSD: req.BudgetUSD,
		OriginName: req.Origin, DestinationName: req.Destination,
		LocalTransport: req.LocalTransport, ActivityRadiusMiles: req.ActivityRadiusMiles, Interests: req.Interests,
	}
	if anyEstimated(activities, food, stops) {
		plan.Sources["place_prices"] = "estimate"
	}
	return plan
}

// refuelStops finds a gas station near each planned refuel point. The stop's
// PriceUSD carries the refuel cost for the group (not per person), and the
// note says which fill it is, so the itinerary can show "refuel #1, ~$48".
func (p *Planner) refuelStops(ctx context.Context, origin, dest travel.LatLng, drive travel.DriveEstimate) []travel.Place {
	fracs := make([]float64, 0, len(drive.Refuels))
	for _, r := range drive.Refuels {
		if drive.DistanceMiles > 0 {
			fracs = append(fracs, r.AtMile/drive.DistanceMiles)
		}
	}
	halfway := len(fracs) == 0
	if halfway {
		fracs = []float64{0.5}
	}
	var stops []travel.Place
	seen := map[string]bool{}
	for k, pt := range p.Geo.WaypointsAt(origin, dest, fracs) {
		for _, s := range p.Places.Nearby(ctx, pt, "rest_stop", 3) {
			if seen[s.Name] {
				continue
			}
			seen[s.Name] = true
			if halfway {
				s.Note = fmt.Sprintf("halfway break, about mile %.0f; one tank covers the drive", drive.DistanceMiles/2)
				s.PriceUSD, s.PriceUnknown, s.PriceEstimated = 0, false, false
			} else {
				r := drive.Refuels[k]
				s.Note = fmt.Sprintf("refuel #%d at about mile %.0f: %.1f gal, ~$%.0f", k+1, r.AtMile, r.Gallons, r.CostUSD)
				s.PriceUSD, s.PriceUnknown, s.PriceEstimated = r.CostUSD, false, true
			}
			stops = append(stops, s)
			break // one station per refuel point
		}
	}
	return stops
}

func placeNames(ps []travel.Place) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Name)
	}
	return out
}

func anyEstimated(lists ...[]travel.Place) bool {
	for _, l := range lists {
		for _, p := range l {
			if p.PriceEstimated {
				return true
			}
		}
	}
	return false
}
