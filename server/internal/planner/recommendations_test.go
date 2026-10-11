package planner

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"trailmix/internal/travel"
)

func TestOutdoorFoodDoesNotAddGenericSightseeing(t *testing.T) {
	p := newOffline()
	plan, err := p.Plan(context.Background(), Request{
		Origin: "Omaha", Destination: "Kansas City", StartDate: "2026-10-24",
		EndDate: "2026-10-27", Travelers: 2, Mode: "fly", Interests: []string{"outdoors", "food"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, activity := range plan.Activities {
		if activity.Category != "outdoors" {
			t.Fatalf("unrelated sightseeing added: %+v", activity)
		}
	}
	seen := map[string]bool{}
	for _, day := range plan.Days {
		for _, item := range day.Items {
			if item.Category != "food" {
				continue
			}
			if seen[item.Title] {
				t.Fatalf("meal repeated: %s", item.Title)
			}
			seen[item.Title] = true
			if !strings.Contains(item.Detail, "hours and availability unverified") {
				t.Fatalf("meal hours asserted: %+v", item)
			}
		}
	}
}

func TestDailyAnchorsPreserveDiscoveryVariety(t *testing.T) {
	center := travel.LatLng{Lat: 40, Lng: -90}
	activities := []travel.Place{
		{Name: "Downtown", Category: "outdoors", Location: center},
		{Name: "Different area", Category: "outdoors", Location: travel.LatLng{Lat: 40.2, Lng: -90}},
		{Name: "Near downtown", Category: "museums", Location: travel.LatLng{Lat: 40.001, Lng: -90}},
		{Name: "Another downtown", Category: "outdoors", Location: travel.LatLng{Lat: 40.002, Lng: -90}},
	}
	days := dailyActivities(activities, center, travel.Weather{}, time.Now(), 3)
	if days[1][1].Name != "Near downtown" || days[2][0].Name != "Different area" {
		t.Fatalf("geographic diversity collapsed into downtown: %+v", days)
	}
}

func TestDinnerCandidatesAreNotRepeatedWhenExhausted(t *testing.T) {
	meals := dailyMeals([]travel.Place{{Name: "Restaurant"}, {Name: " restaurant "}},
		travel.LatLng{}, make([][]travel.Place, 4), 3)
	if meals[0] == nil || meals[1] != nil || meals[2] != nil {
		t.Fatalf("repeated venue or fabricated dinner: %+v", meals)
	}

}

func TestKansasCityOutdoorFoodLive(t *testing.T) {
	if os.Getenv("GEOAPIFY_LIVE_TEST") != "1" {
		t.Skip("set GEOAPIFY_LIVE_TEST=1 and GEOAPIFY_KEY for live verification")
	}
	p := newOffline()
	p.Places = travel.NewPlacesWithGeoapify("", os.Getenv("GEOAPIFY_KEY"))
	plan, err := p.Plan(context.Background(), Request{
		Origin: "Omaha", Destination: "Kansas City", StartDate: "2026-10-24",
		EndDate: "2026-10-27", Travelers: 2, Mode: "fly", Interests: []string{"outdoors", "food"},
		ActivityRadiusMiles: 25,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Activities) == 0 || len(plan.Food) < 3 {
		t.Fatalf("insufficient live matches: %d activities, %d restaurants", len(plan.Activities), len(plan.Food))
	}
	if err := validateActivityVariety(plan.Days, plan.Activities); err != nil {
		t.Fatalf("live schedule violates nearby activity variety: %v", err)
	}
	for _, activity := range plan.Activities {
		if activity.Source != "geoapify" || activity.Category != "outdoors" {
			t.Fatalf("unexpected live activity: %+v", activity)
		}
	}
	seen := map[string]bool{}
	for _, day := range plan.Days {
		for _, item := range day.Items {
			if item.Category == "food" {
				if seen[item.Title] {
					t.Fatalf("repeated live dinner: %s", item.Title)
				}
				seen[item.Title] = true
				t.Logf("day %d: %s", day.Day, item.Title)
			} else if item.Category == "activity" {
				t.Logf("day %d: %s", day.Day, item.Title)
			}
		}
	}
}
