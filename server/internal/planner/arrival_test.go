package planner

import (
	"context"
	"strings"
	"testing"

	"trailmix/internal/travel"
)

func TestArrivalModeControlsStopsAndCarPickup(t *testing.T) {
	for _, mode := range []string{"drive", "fly"} {
		t.Run(mode, func(t *testing.T) {
			p := newOffline()
			plan, err := p.Plan(context.Background(), Request{
				Origin: "Kansas City", Destination: "Denver", StartDate: "2026-11-06",
				EndDate: "2026-11-09", Travelers: 2, Mode: mode,
				LocalTransport: "rental_car", ActivityRadiusMiles: 50,
			})
			if err != nil {
				t.Fatal(err)
			}
			if mode == "drive" {
				if len(plan.Stops) == 0 || effectiveTransport(plan) != "own car" {
					t.Fatal("driving must retain road stops and own car")
				}
				if strings.Contains(plan.Summary, "rental") {
					t.Fatal("driving summary must not suggest rental costs or pickup")
				}
			} else if len(plan.Stops) != 0 || effectiveTransport(plan) != "rental car" {
				t.Fatal("flying must omit road stops and retain requested rental")
			}
			for _, day := range plan.Days {
				for _, item := range day.Items {
					if mode == "fly" && item.Category == "stop" {
						t.Fatalf("flight itinerary has road stop: %+v", item)
					}
					if mode == "drive" && strings.Contains(strings.ToLower(item.Detail), "reserve a rental") {
						t.Fatalf("driving itinerary has rental pickup: %+v", item)
					}
					if mode == "drive" && strings.HasPrefix(item.Title, "Arrange destination transportation:") {
						t.Fatalf("own-car itinerary has unnecessary setup: %+v", item)
					}
				}
			}
			if mode == "drive" {
				plan.Activities = []travel.Place{{Name: "Real activity", Source: "geoapify"}}
				plan.Food = nil
				plan.Sources["places"] = "mixed"
				if err := p.SelectFlight(context.Background(), plan, 0, ""); err != nil {
					t.Fatal(err)
				}
				if len(plan.Stops) != 0 || plan.Sources["places"] != "geoapify" ||
					effectiveTransport(plan) != "rental car" {
					t.Fatal("switching to flight must clear stops/provenance and restore rental preference")
				}
			}
		})
	}
}
