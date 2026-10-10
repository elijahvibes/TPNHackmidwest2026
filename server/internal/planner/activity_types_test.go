package planner

import (
	"testing"
	"time"

	"trailmix/internal/travel"
)

func TestNearbyActivityTypes(t *testing.T) {
	park := travel.Place{Category: "outdoors", Categories: []string{"leisure", "leisure.park"}, Location: travel.LatLng{Lat: 40, Lng: -90}}
	secondPark := park
	secondPark.Location.Lat += 0.005
	museum := secondPark
	museum.Categories = []string{"entertainment", "entertainment.museum.history"}
	nature := secondPark
	nature.Categories = []string{"natural.water.lake"}
	if differentNearbyActivities(park, secondPark) {
		t.Fatal("two nearby parks must not be paired")
	}
	if !differentNearbyActivities(park, museum) || !differentNearbyActivities(park, nature) {
		t.Fatal("nearby different types should be allowed")
	}
	secondPark.Location.Lat += 0.03
	if !differentNearbyActivities(park, secondPark) {
		t.Fatal("same type beyond one mile should remain allowed")
	}
	for _, tc := range []struct {
		category string
		want     string
	}{
		{"park", "park"}, {"museum", "museum"}, {"natural.forest", "nature"},
		{"tourism.sights.memorial", "sightseeing"},
	} {
		if got := activityType(travel.Place{Categories: []string{tc.category}}); got != tc.want {
			t.Fatalf("%s classified as %s, want %s", tc.category, got, tc.want)
		}
	}
	if differentNearbyActivities(travel.Place{Category: "outdoors"}, travel.Place{Category: "outdoors"}) {
		t.Fatal("legacy same-category results should be treated conservatively")
	}
}

func TestDailyActivitiesSkipSameTypeNearbyPair(t *testing.T) {
	center := travel.LatLng{Lat: 40, Lng: -90}
	park := travel.Place{Name: "Park", Category: "outdoors", Categories: []string{"leisure.park"}, Location: center}
	nearPark := park
	nearPark.Name, nearPark.Location.Lat = "Another park", 40.001
	museum := travel.Place{Name: "Museum", Category: "museums", Categories: []string{"entertainment.museum"}, Location: travel.LatLng{Lat: 40.003, Lng: -90}}
	days := dailyActivities([]travel.Place{park, nearPark, museum}, center, travel.Weather{}, time.Now(), 3)
	if len(days[1]) != 2 || days[1][1].Name != "Museum" {
		t.Fatalf("should choose nearby museum instead of closer park: %+v", days[1])
	}
	single := dailyActivities([]travel.Place{park, nearPark}, center, travel.Weather{}, time.Now(), 3)
	if len(single[1]) != 1 || len(single[2]) != 1 {
		t.Fatalf("same-type nearby activities should split across days: %+v", single)
	}
}

func TestAIActivityVarietyValidation(t *testing.T) {
	places := []travel.Place{
		{Name: "Park A", Categories: []string{"leisure.park"}, Location: travel.LatLng{Lat: 40, Lng: -90}},
		{Name: "Park B", Categories: []string{"park"}, Location: travel.LatLng{Lat: 40.001, Lng: -90}},
		{Name: "Museum", Categories: []string{"museum"}, Location: travel.LatLng{Lat: 40.002, Lng: -90}},
	}
	days := []Day{{Day: 1, Items: []DayItem{{Title: "Park A", Category: "activity"}, {Title: "Park B", Category: "activity"}}}}
	if err := validateActivityVariety(days, places); err == nil {
		t.Fatal("AI nearby same-type pair should trigger fallback")
	}
	days[0].Items[1].Title = "Museum"
	if err := validateActivityVariety(days, places); err != nil {
		t.Fatalf("valid mixed pair rejected: %v", err)
	}
}
