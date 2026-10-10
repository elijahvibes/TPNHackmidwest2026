package planner

import (
	"context"
	"testing"

	"trailmix/internal/ai"
	"trailmix/internal/travel"
)

// With no API keys every provider returns mock data, so this exercises the
// whole pricing + scoring + fallback-itinerary path offline.
func newOffline() *Planner {
	return &Planner{
		AI:      ai.New("", "", ""),
		Geo:     travel.NewGeo(""),
		Fuel:    travel.NewFuel(""),
		Amadeus: travel.NewAmadeus("", ""),
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
	if plan.Sources["flights"] != "mock" || plan.Sources["hotels"] != "mock" {
		t.Errorf("sources should be mock: %v", plan.Sources)
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
