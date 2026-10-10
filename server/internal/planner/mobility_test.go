package planner

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"trailmix/internal/travel"
)

func TestMobilityValidationAndDefaults(t *testing.T) {
	for _, req := range []Request{
		{LocalTransport: "helicopter"}, {ActivityRadiusMiles: -1},
		{ActivityRadiusMiles: 151}, {ActivityRadiusMiles: math.NaN()},
	} {
		if err := normalizeMobility(&req); err == nil {
			t.Fatalf("accepted invalid mobility: %+v", req)
		}
	}
	walk := Request{LocalTransport: "walking"}
	if err := normalizeMobility(&walk); err != nil || walk.ActivityRadiusMiles != 2 {
		t.Fatalf("walking default: %+v, %v", walk, err)
	}
	car := Request{LocalTransport: "rental_car", ActivityRadiusMiles: 80}
	if err := normalizeMobility(&car); err != nil || car.ActivityRadiusMiles != 80 {
		t.Fatalf("user range changed: %+v, %v", car, err)
	}
}

func TestGroupingPairsNearbyActivitiesAndMeals(t *testing.T) {
	places := []travel.Place{
		{Name: "A", Location: travel.LatLng{Lat: 40, Lng: -90}},
		{Name: "Far", Location: travel.LatLng{Lat: 41, Lng: -90}},
		{Name: "Near", Location: travel.LatLng{Lat: 40.01, Lng: -90}},
	}
	grouped := groupActivities(places)
	if grouped[0].Name != "A" || grouped[1].Name != "Near" || places[1].Name != "Far" {
		t.Fatalf("bad grouping or mutated candidates: %+v", grouped)
	}
	if f := nearbyFood(places, places[2].Location); f == nil || f.Name != "Near" {
		t.Fatalf("meal not near activity: %+v", f)
	}
}

func TestFlightSelectionPreservesDestinationPreferences(t *testing.T) {
	p, plan := selectionPlan(t)
	plan.LocalTransport = "rental_car"
	plan.ActivityRadiusMiles = 75
	if err := p.SelectFlight(context.Background(), plan, 1, ""); err != nil {
		t.Fatal(err)
	}
	if plan.LocalTransport != "rental_car" || plan.ActivityRadiusMiles != 75 ||
		!strings.Contains(plan.Summary, "using rental car") {
		t.Fatalf("selection lost preferences: %s", plan.Summary)
	}
}

func TestAutomaticMobilityChangesWithArrivalMode(t *testing.T) {
	p, plan := selectionPlan(t)
	plan.LocalTransport = "auto"
	plan.RecommendedMode = "drive"
	if effectiveTransport(plan) != "own car" {
		t.Fatal("driving should retain own car")
	}
	if err := p.SelectFlight(context.Background(), plan, 0, ""); err != nil {
		t.Fatal(err)
	}
	if effectiveTransport(plan) != "transit/rideshare" {
		t.Fatal("flying must not imply an own car")
	}
}

func TestWideWalkingRangeIsExplicitlyFlagged(t *testing.T) {
	plan := &Plan{LocalTransport: "walking", ActivityRadiusMiles: 50}
	if !strings.Contains(mobilityNote(plan), "additional transportation") {
		t.Fatal("wide walking range lacks warning")
	}
}

func TestWalkingFlightDoesNotAssumeAirportAccess(t *testing.T) {
	item := destinationTransfer(&Plan{LocalTransport: "walking", RecommendedMode: "fly"})
	if !item.CostUnknown || !strings.Contains(item.Detail, "Arrange an airport transfer separately") {
		t.Fatalf("walking airport transfer not disclosed: %+v", item)
	}
}

func TestDailyActivitiesAvoidRepeatsAndDepartureExcursions(t *testing.T) {
	center := travel.LatLng{Lat: 40, Lng: -90}
	acts := []travel.Place{
		{Name: "Park", Location: center, PriceUSD: 10},
		{Name: "Day trip", Location: travel.LatLng{Lat: 40.5, Lng: -90}, PriceUSD: 20},
		{Name: "Museum", Category: "museums", Location: travel.LatLng{Lat: 40.01, Lng: -90}, PriceUSD: 30},
		{Name: "Long trip", Location: travel.LatLng{Lat: 40.6, Lng: -90}, PriceUSD: 40},
	}
	start := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	weather := travel.Weather{Days: []travel.DayWeather{{Date: "2026-10-25", PrecipPct: 80}}}
	days := dailyActivities(acts, center, weather, start, 4)
	seen := map[string]bool{}
	if days[1][0].Name != "Museum" {
		t.Fatal("rain-day indoor preference was lost")
	}
	for day, places := range days {
		for _, a := range places {
			if seen[a.Name] {
				t.Fatalf("repeated %s", a.Name)
			}
			seen[a.Name] = true
			if distance(center, a.Location) > 20 && (len(places) != 1 || day == 4) {
				t.Fatal("excursion must have its own non-departure day")
			}
		}
	}
}

func TestExpandedScheduleCostMatchesPlannedItems(t *testing.T) {
	_, plan := selectionPlan(t)
	plan.LocalTransport = "rental_car"
	plan.ActivityRadiusMiles = 50
	plan.Activities = []travel.Place{
		{Name: "A", Location: plan.DestLoc, PriceUSD: 10},
		{Name: "B", Location: plan.DestLoc, PriceUSD: 20},
		{Name: "C", Location: plan.DestLoc, PriceUSD: 30},
	}
	plan.Food = []travel.Place{{Name: "Meal", Location: plan.DestLoc, PriceUSD: 12}}
	start, _ := time.Parse("2006-01-02", plan.Flights[0].Depart)
	cost := plannedPlaceCost(plan.Activities, plan.Food, plan.DestLoc, plan.Weather, start, plan.Nights, plan.Travelers)
	_, days := fallbackItinerary(Request{Travelers: plan.Travelers}, plan, start)
	actual := 0.0
	for _, day := range days {
		for _, item := range day.Items {
			if item.Category == "activity" || item.Category == "food" {
				actual += item.CostUSD
			}
		}
	}
	if cost != actual {
		t.Fatalf("schedule costs %.2f, option costs %.2f", actual, cost)
	}
}
