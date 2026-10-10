package planner

import (
	"context"
	"errors"
	"strings"
	"testing"

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
	return []travel.HotelQuote{{
		Name: "Test hotel", Rating: 4, NightlyUSD: 100, TotalUSD: float64(nights) * 100,
		Nights: nights, Kind: "hotel", Source: "serpapi",
	}}, nil
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
	if plan.RecommendedMode != plan.Options[0].Mode {
		t.Errorf("recommended %s != best option %s", plan.RecommendedMode, plan.Options[0].Mode)
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
