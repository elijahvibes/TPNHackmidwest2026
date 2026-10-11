package planner

import (
	"strings"
	"testing"
	"time"

	"trailmix/internal/travel"
)

func TestRainAwareAssignSwapsOutdoorOffRainyDay(t *testing.T) {
	start := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	museum := travel.Place{Name: "Denver Art Museum", Categories: []string{"entertainment.museum"}, Location: travel.LatLng{Lat: 39.737, Lng: -104.989}}
	garden := travel.Place{Name: "Denver Botanic Gardens", Categories: []string{"wikivoyage.see"}, Location: travel.LatLng{Lat: 39.732, Lng: -104.960}}
	meow := travel.Place{Name: "Convergence Station", Categories: []string{"wikivoyage.see"}, Location: travel.LatLng{Lat: 39.743, Lng: -105.019}}
	park := travel.Place{Name: "Cheesman Park", Categories: []string{"leisure.park"}, Location: travel.LatLng{Lat: 39.733, Lng: -104.966}}
	plan := &Plan{Activities: []travel.Place{museum, garden, meow, park}, Weather: travel.Weather{Days: []travel.DayWeather{
		{Date: "2026-11-07", PrecipPct: 68}, {Date: "2026-11-08", PrecipPct: 5},
	}}}
	assign := []dayAssignment{
		{Theme: "Arrive"},
		{Theme: "Museum morning, garden afternoon", Activities: []travel.Place{museum, garden}},
		{Theme: "Immersive art and park", Activities: []travel.Place{meow, park}},
		{Theme: "Depart"},
	}
	notes := rainAwareAssign(assign, plan, start)
	if len(notes) != 1 || assign[1].Activities[1].Name != "Convergence Station" || assign[2].Activities[0].Name != "Denver Botanic Gardens" {
		t.Fatalf("garden should swap with Meow Wolf onto the dry day: %v\n%+v", notes, assign)
	}
	if assign[1].Theme != "Indoor day, 68% chance of rain" {
		t.Errorf("theme: %q", assign[1].Theme)
	}
	if assign[2].Theme != "Immersive art and park" {
		t.Errorf("dry day theme should not change: %q", assign[2].Theme)
	}
}

func TestRainAwareAssignReplacesFromSpareAndLeavesUnknownForecast(t *testing.T) {
	start := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	park := travel.Place{Name: "Cheesman Park", Location: travel.LatLng{Lat: 39.733, Lng: -104.966}}
	mint := travel.Place{Name: "Denver Mint", Location: travel.LatLng{Lat: 39.739, Lng: -104.992}}
	plan := &Plan{Activities: []travel.Place{park, mint}, Weather: travel.Weather{Days: []travel.DayWeather{{Date: "2026-11-07", PrecipPct: 80}}}}
	assign := []dayAssignment{{}, {Theme: "Park day", Activities: []travel.Place{park}}, {}}
	notes := rainAwareAssign(assign, plan, start)
	if assign[1].Activities[0].Name != "Denver Mint" || len(notes) != 1 || len(assign[1].Activities) != 1 {
		t.Errorf("park should be replaced by the unused Mint, nothing left to top up with: %v %+v", notes, assign[1])
	}
	// With a spare indoor candidate, a one-activity day is topped up.
	plan.Activities = append(plan.Activities, travel.Place{Name: "Clyfford Still Museum", Categories: []string{"entertainment.museum"}, Location: travel.LatLng{Lat: 39.736, Lng: -104.990}})
	assign = []dayAssignment{{}, {Activities: []travel.Place{park}}, {}}
	notes = rainAwareAssign(assign, plan, start)
	if len(assign[1].Activities) != 2 || assign[1].Activities[1].Name != "Clyfford Still Museum" {
		t.Errorf("top-up: %v %+v", notes, assign[1])
	}
	// No forecast for the trip dates → nothing moves.
	plan.Weather = travel.Weather{Days: []travel.DayWeather{{Date: "2026-10-11", PrecipPct: 90}}}
	assign = []dayAssignment{{}, {Activities: []travel.Place{park}}, {}}
	notes = rainAwareAssign(assign, plan, start)
	if assign[1].Activities[0].Name != "Cheesman Park" || len(notes) != 1 || !strings.Contains(notes[0], "added") {
		t.Errorf("outlook for other dates must not move the park (only the top-up applies): %v %+v", notes, assign[1])
	}
	if !isOutdoor(park) || isOutdoor(mint) || !isOutdoor(travel.Place{Name: "Red Rocks Park and Amphitheatre"}) {
		t.Error("isOutdoor")
	}
}
