package planner

import (
	"context"
	"testing"
	"time"

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
	if got := pickHotel(hotels, 1000)[0].Name; got != "Fancy" {
		t.Errorf("big budget should pick best rated, got %s", got)
	}
	if got := pickHotel(hotels, 350)[0].Name; got != "Value" {
		t.Errorf("mid budget should pick best rated that fits, got %s", got)
	}
	if got := pickHotel(hotels, 100)[0].Name; got != "Cheap" {
		t.Errorf("tiny budget should pick cheapest, got %s", got)
	}
	if got := pickHotel(hotels, 0)[0].Name; got != "Value" {
		t.Errorf("no budget should keep order, got %s", got)
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
