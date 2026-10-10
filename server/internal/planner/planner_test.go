package planner

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"trailmix/internal/ai"
	"trailmix/internal/travel"
)

type fixtureSearch struct{}

func (fixtureSearch) Enabled() bool { return true }

func (fixtureSearch) ReturnFlights(context.Context, travel.FlightSearch) ([]travel.FlightQuote, error) {
	return []travel.FlightQuote{{Carrier: "Return airline", PriceUSD: 300, DurationMin: 180, Source: "serpapi", BookingToken: "booking"}}, nil
}

func (fixtureSearch) BookingOptions(context.Context, travel.FlightSearch) ([]travel.BookingOption, error) {
	return nil, nil
}

func (fixtureSearch) Flights(_ context.Context, origin, dest, depart, ret string, _ int) ([]travel.FlightQuote, error) {
	return []travel.FlightQuote{{
		Carrier: "Test airline", Origin: origin, Destination: dest, Depart: depart, Return: ret,
		PriceUSD: 250, DurationMin: 120, Source: "serpapi",
	}}, nil
}

func (fixtureSearch) Hotels(_ context.Context, _ string, _, _ string, nights, _ int) ([]travel.HotelQuote, error) {
	stay := func(name string, rating, nightly float64, kind string) travel.HotelQuote {
		return travel.HotelQuote{
			Name: name, Rating: rating, NightlyUSD: nightly, TotalUSD: float64(nights) * nightly,
			Nights: nights, Kind: kind, Source: "serpapi",
		}
	}
	return []travel.HotelQuote{
		stay("Test hotel", 4, 100, "hotel"),
		stay("Budget inn", 3, 70, "hotel"),
		stay("Test rental", 4.5, 160, "rental"),
	}, nil
}

// Travel search fixtures exercise scoring offline without production mock quotes.
func newOffline() *Planner {
	return &Planner{
		AI:      ai.New("", "", ""),
		Geo:     travel.NewGeo(""),
		Fuel:    travel.NewFuel(""),
		SerpAPI: fixtureSearch{},
		Places:  travel.NewPlaces(""),
	}
}

func TestPlanOffline(t *testing.T) {
	p := newOffline()
	plan, err := p.Plan(context.Background(), Request{
		Origin: "Kansas City", Destination: "Denver",
		StartDate: "2026-11-06", EndDate: "2026-11-09",
		Travelers: 2, BudgetUSD: 1500, Mode: "auto", VehicleMPG: 32,
	})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.Nights != 3 {
		t.Errorf("nights = %d, want 3", plan.Nights)
	}
	if len(plan.Options) != 2 {
		t.Fatalf("options = %d, want drive + fly", len(plan.Options))
	}
	if plan.Drive.DistanceMiles < 500 || plan.Drive.DistanceMiles > 800 {
		t.Errorf("KC→Denver distance = %.0f mi, expected roughly 600", plan.Drive.DistanceMiles)
	}
	if plan.Drive.VehicleMPG != 32 {
		t.Errorf("vehicle mpg not honored: %v", plan.Drive.VehicleMPG)
	}
	if plan.TotalUSD <= 0 {
		t.Errorf("total = %v", plan.TotalUSD)
	}
	if len(plan.Days) != 4 {
		t.Errorf("days = %d, want 4 (3 nights)", len(plan.Days))
	}
	if plan.AIGenerated {
		t.Error("ai_generated should be false with no AI_BASE_URL")
	}
	if plan.Sources["flights"] != "serpapi" || plan.Sources["hotels"] != "serpapi" {
		t.Errorf("sources should identify the travel provider: %v", plan.Sources)
	}
	// Options are sorted best value first and the recommendation matches.
	if plan.Options[0].ValueScore < plan.Options[1].ValueScore {
		t.Error("options not sorted by value")
	}
	// Recommendation is the highest-value option that fits the budget.
	var rec *Option
	for i := range plan.Options {
		if plan.Options[i].Mode == plan.RecommendedMode {
			rec = &plan.Options[i]
		}
	}
	if rec == nil {
		t.Fatalf("recommended mode %q not among options", plan.RecommendedMode)
	}
	if rec.TotalUSD > 1500 {
		for _, o := range plan.Options {
			if o.TotalUSD <= 1500 {
				t.Errorf("recommended %s ($%.0f) busts budget while %s ($%.0f) fits", rec.Mode, rec.TotalUSD, o.Mode, o.TotalUSD)
			}
		}
	}
	if !plan.UnderBudget && rec.TotalUSD <= 1500 {
		t.Error("under_budget flag disagrees with total")
	}
}

func TestPlanForcedMode(t *testing.T) {
	p := newOffline()
	plan, err := p.Plan(context.Background(), Request{
		Origin: "Chicago", Destination: "Miami", StartDate: "2026-12-01", EndDate: "2026-12-05", Travelers: 1, Mode: "fly",
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.RecommendedMode != "fly" {
		t.Errorf("forced fly, got %s", plan.RecommendedMode)
	}
}

func TestPickHotelHonorsBudget(t *testing.T) {
	hotels := []travel.HotelQuote{
		{Name: "Value", Rating: 4.0, TotalUSD: 300},
		{Name: "Fancy", Rating: 4.8, TotalUSD: 900},
		{Name: "Cheap", Rating: 3.2, TotalUSD: 180},
	}
	if got := pickHotel(hotels, 1000, false)[0].Name; got != "Fancy" {
		t.Errorf("big budget should pick best rated, got %s", got)
	}
	if got := pickHotel(hotels, 350, false)[0].Name; got != "Value" {
		t.Errorf("mid budget should pick best rated that fits, got %s", got)
	}
	if got := pickHotel(hotels, 100, false)[0].Name; got != "Cheap" {
		t.Errorf("tiny budget should pick cheapest, got %s", got)
	}
	if got := pickHotel(hotels, 0, false)[0].Name; got != "Value" {
		t.Errorf("no budget should keep order, got %s", got)
	}
}

func TestPickHotelPrefersCentralWhenElevated(t *testing.T) {
	hotels := []travel.HotelQuote{
		{Name: "Far & Fancy", Rating: 4.9, TotalUSD: 500, DistanceKm: 9},
		{Name: "Central OK", Rating: 4.1, TotalUSD: 450, DistanceKm: 1.2},
		{Name: "Far & Cheap", Rating: 3.5, TotalUSD: 200, DistanceKm: 7},
	}
	if got := pickHotel(hotels, 1000, true)[0].Name; got != "Central OK" {
		t.Errorf("elevated crime should prefer the central stay, got %s", got)
	}
	if got := pickHotel(hotels, 1000, false)[0].Name; got != "Far & Fancy" {
		t.Errorf("normal crime should pick best rated, got %s", got)
	}
	if got := pickHotel(hotels, 0, true)[0].Name; got != "Central OK" {
		t.Errorf("no budget + elevated should still move central first, got %s", got)
	}
}

func TestWeatherAndSafetyInPlan(t *testing.T) {
	p := newOffline() // Weather nil → mock forecast, no network
	plan, err := p.Plan(context.Background(), Request{Origin: "Omaha", Destination: "St. Louis", StartDate: "2026-11-06", EndDate: "2026-11-09", Travelers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Weather.Source != "mock" || len(plan.Weather.Days) != 4 {
		t.Errorf("weather = %s/%d days, want mock/4", plan.Weather.Source, len(plan.Weather.Days))
	}
	if plan.Safety.Level != "elevated" || plan.Safety.Score <= 0 {
		t.Errorf("St. Louis should rate elevated, got %+v", plan.Safety)
	}
	if plan.Sources["weather"] != "mock" || plan.Sources["safety"] == "" {
		t.Errorf("sources missing weather/safety: %v", plan.Sources)
	}
}

func TestSanitizeDays(t *testing.T) {
	start, _ := time.Parse("2006-01-02", "2026-11-06")
	in := []Day{
		{Day: 9, Date: "bogus", Theme: "A", Items: []DayItem{{Time: "9am", Title: "Thing", Category: "weird", CostUSD: -5}}},
		{Day: 2, Theme: "empty", Items: nil},
		{Day: 3, Theme: "B", Items: []DayItem{{Time: "10:00", Title: "", Category: "food"}, {Time: "11:00", Title: "Lunch", Category: "food", CostUSD: 20}}},
	}
	out := sanitizeDays(in, start, 2)
	if len(out) != 2 {
		t.Fatalf("want 2 days, got %d", len(out))
	}
	if out[0].Day != 1 || out[0].Date != "2026-11-06" || out[1].Day != 2 || out[1].Date != "2026-11-07" {
		t.Errorf("days not renumbered: %+v", out)
	}
	it := out[0].Items[0]
	if it.Category != "activity" || it.CostUSD != 0 || it.Time != "--:--" {
		t.Errorf("item not sanitized: %+v", it)
	}
	if len(out[1].Items) != 1 || out[1].Items[0].Title != "Lunch" {
		t.Errorf("empty-title item not dropped: %+v", out[1].Items)
	}
}

func TestInterestsShapeActivities(t *testing.T) {
	p := newOffline()
	base := Request{Origin: "Omaha", Destination: "Denver", StartDate: "2026-11-01", EndDate: "2026-11-03", Travelers: 1}
	plain, err := p.Plan(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	withInterests := base
	withInterests.Interests = []string{"outdoors", "history"}
	shaped, err := p.Plan(context.Background(), withInterests)
	if err != nil {
		t.Fatal(err)
	}
	if len(shaped.Activities) == 0 || len(shaped.Activities) > 6 {
		t.Fatalf("activities = %d, want 1..6", len(shaped.Activities))
	}
	names := map[string]bool{}
	for _, a := range shaped.Activities {
		if names[a.Name] {
			t.Errorf("duplicate activity %q", a.Name)
		}
		names[a.Name] = true
	}
	if shaped.Activities[0].Category != "outdoors" {
		t.Errorf("first activity should come from the first interest, got %q", shaped.Activities[0].Category)
	}
	if shaped.Activities[0].Name == plain.Activities[0].Name {
		t.Error("interests had no effect on activities")
	}
}

func TestPlanRejectsBadDates(t *testing.T) {
	p := newOffline()
	if _, err := p.Plan(context.Background(), Request{Origin: "Omaha", Destination: "Denver", StartDate: "2026-12-05", EndDate: "2026-12-01"}); err == nil {
		t.Error("expected error for end before start")
	}
}

func TestPlanUsesBrowserLocation(t *testing.T) {
	p := newOffline()
	plan, err := p.Plan(context.Background(), Request{
		Origin: "somewhere unknown", Destination: "Nashville", StartDate: "2026-11-01", EndDate: "2026-11-02",
		OriginLoc: &travel.LatLng{Lat: 39.0, Lng: -94.5},
	})
	if err != nil {
		t.Fatalf("origin_loc should bypass geocoding: %v", err)
	}

	if plan.OriginLoc.Lat != 39.0 {
		t.Errorf("origin loc not used: %+v", plan.OriginLoc)
	}
}

func TestAnyCityGetsNearestAirport(t *testing.T) {
	p := newOffline()
	plan, err := p.Plan(context.Background(), Request{
		Origin: "Lawrence, KS", Destination: "Estes Park, CO", StartDate: "2026-11-06", EndDate: "2026-11-09", Travelers: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.OriginPlace.Label != "Lawrence, KS" || plan.DestPlace.Label != "Estes Park, CO" {
		t.Errorf("places = %q → %q", plan.OriginPlace.Label, plan.DestPlace.Label)
	}
	if plan.Title != "Lawrence, KS → Estes Park, CO" {
		t.Errorf("title = %q", plan.Title)
	}
	if plan.OriginAirport == nil || plan.OriginAirport.Code != "MCI" || plan.DestAirport == nil || plan.DestAirport.Code != "DEN" {
		t.Fatalf("airports = %+v / %+v", plan.OriginAirport, plan.DestAirport)
	}
	if len(plan.Flights) == 0 || plan.Flights[0].Origin != "MCI" {
		t.Errorf("flights should be priced MCI→DEN: %+v", first(plan.Flights))
	}
	if plan.Sources["geocode"] != "gazetteer" {
		t.Errorf("geocode source = %q", plan.Sources["geocode"])
	}
	// Flying from a town 45 mi from its airport costs extra door-to-door hours.
	var fly *Option
	for i := range plan.Options {
		if plan.Options[i].Mode == "fly" {
			fly = &plan.Options[i]
		}
	}
	if fly == nil {
		t.Fatal("no fly option")
	}
	found := false
	for _, f := range fly.Factors {
		if strings.Contains(f, "MCI is") || strings.Contains(f, "DEN is") {
			found = true
		}
	}
	if !found {
		t.Errorf("fly factors should mention the airport drive: %v", fly.Factors)
	}
}

func TestPinnedDestinationSkipsGeocoding(t *testing.T) {
	p := newOffline()
	plan, err := p.Plan(context.Background(), Request{
		Origin: "Kansas City", Destination: "Lake Nobody, KS", DestLoc: &travel.LatLng{Lat: 38.5, Lng: -96.0},
		StartDate: "2026-11-06", EndDate: "2026-11-07", Travelers: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.DestPlace.Label != "Lake Nobody, KS" || plan.DestLoc.Lat != 38.5 {
		t.Errorf("pinned destination not honored: %+v", plan.DestPlace)
	}
}

func TestChooseHotelRetotals(t *testing.T) {
	p := newOffline()
	plan, err := p.Plan(context.Background(), Request{
		Origin: "Omaha", Destination: "Denver", StartDate: "2026-11-06", EndDate: "2026-11-09", Travelers: 2, BudgetUSD: 1500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Hotels) < 2 {
		t.Fatal("need alternatives")
	}
	alt := plan.Hotels[len(plan.Hotels)-1]
	before := plan.TotalUSD
	if err := plan.ChooseHotel(alt.Name, 1500); err != nil {
		t.Fatal(err)
	}
	if plan.Hotels[0].Name != alt.Name {
		t.Errorf("chosen hotel not first: %s", plan.Hotels[0].Name)
	}
	var rec *Option
	for i := range plan.Options {
		if plan.Options[i].LodgingUSD != alt.TotalUSD {
			t.Errorf("option %s lodging %.0f != %.0f", plan.Options[i].Mode, plan.Options[i].LodgingUSD, alt.TotalUSD)
		}
		if plan.Options[i].Mode == plan.RecommendedMode {
			rec = &plan.Options[i]
		}
	}
	if rec == nil || plan.TotalUSD != rec.TotalUSD {
		t.Errorf("plan total %.0f should follow the recommended option", plan.TotalUSD)
	}
	if plan.TotalUSD == before && plan.Hotels[0].TotalUSD != plan.Hotels[1].TotalUSD {
		t.Error("total did not change after swapping hotels")
	}
	checkin := false
	for _, d := range plan.Days {
		for _, it := range d.Items {
			if it.Category == "lodging" {
				checkin = true
				if !strings.Contains(it.Title, alt.Name) || it.CostUSD != alt.TotalUSD {
					t.Errorf("check-in line not rewritten: %+v", it)
				}
			}
		}
	}
	if !checkin {
		t.Error("itinerary has no lodging item")
	}
	if err := plan.ChooseHotel("Nope Hotel", 0); err == nil {
		t.Error("unknown hotel should error")
	}
}

func TestPlanLiveSearchFailure(t *testing.T) {
	p := newOffline()
	p.SerpAPI = travel.NewSerpAPI("test-secret")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	plan, err := p.Plan(ctx, Request{
		Origin: "Kansas City", Destination: "Chicago",
		StartDate: "2026-11-10", EndDate: "2026-11-13", Travelers: 2,
	})
	if plan != nil || !errors.Is(err, travel.ErrProvider) {
		t.Fatalf("expected no mock plan on live failure, got %v, %v", plan, err)
	}
}

func TestPlanLiveRequiresOvernightStay(t *testing.T) {
	p := newOffline()
	p.SerpAPI = travel.NewSerpAPI("test-secret")
	_, err := p.Plan(context.Background(), Request{
		Origin: "Kansas City", Destination: "Chicago",
		StartDate: "2026-11-10", EndDate: "2026-11-10", Travelers: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "end_date must be after") {
		t.Fatalf("expected live date validation, got %v", err)
	}
}

func TestPlanRequiresSerpAPIKey(t *testing.T) {
	p := newOffline()
	p.SerpAPI = travel.NewSerpAPI("")
	plan, err := p.Plan(context.Background(), Request{
		Origin: "Kansas City", Destination: "Chicago",
		StartDate: "2026-11-10", EndDate: "2026-11-13", Travelers: 2,
	})
	if plan != nil || !errors.Is(err, travel.ErrProvider) || !strings.Contains(err.Error(), "SERPAPI_KEY") {
		t.Fatalf("expected missing key error with no plan, got %v, %v", plan, err)
	}
}
