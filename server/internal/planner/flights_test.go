package planner

import (
	"context"
	"strings"
	"testing"

	"trailmix/internal/travel"
)

func selectionPlan(t *testing.T) (*Planner, *Plan) {
	t.Helper()
	p := newOffline()
	plan, err := p.Plan(context.Background(), Request{
		Origin: "Kansas City", Destination: "Chicago", StartDate: "2026-11-10",
		EndDate: "2026-11-13", Travelers: 2, Mode: "fly", BudgetUSD: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	f := plan.Flights[0]
	f.Carrier, f.PriceUSD, f.DurationMin = "Alternative airline", 400, 240
	f.DepartureToken = "departure"
	plan.Flights = append(plan.Flights, f)
	return p, plan
}

func TestSelectFlightUpdatesWholePlan(t *testing.T) {
	p, plan := selectionPlan(t)
	err := p.SelectFlight(context.Background(), plan, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	var fly Option
	for _, o := range plan.Options {
		if o.Mode == "fly" {
			fly = o
		}
	}
	if fly.TransportUSD != 800 || fly.TravelHours != 13 || plan.TotalUSD != fly.TotalUSD {
		t.Fatalf("selection totals inconsistent: %+v, total %v", fly, plan.TotalUSD)
	}
	if plan.SelectedFlight != 1 || plan.SelectedReturn != nil || plan.RecommendedMode != "fly" || !plan.FlightSelected {
		t.Fatalf("selection state inconsistent: %+v", plan)
	}
	if !strings.Contains(plan.Summary, "You chose to fly") || !strings.Contains(plan.Summary, "Kansas City") {
		t.Fatalf("summary not updated: %s", plan.Summary)
	}
	if plan.UnderBudget || plan.AIGenerated {
		t.Error("budget and AI flags not updated")
	}
	for _, item := range plan.Days[0].Items {
		if item.Category == "transport" && (!strings.Contains(item.Title, "Alternative airline") || item.CostUSD != 800) {
			t.Fatalf("itinerary transport not updated: %+v", item)
		}
	}
}

func TestSelectReturnUsesProviderRoundTripPrice(t *testing.T) {
	p, plan := selectionPlan(t)
	if err := p.SelectFlight(context.Background(), plan, 1, "booking"); err != nil {
		t.Fatal(err)
	}
	for _, o := range plan.Options {
		if o.Mode == "fly" && (o.TransportUSD != 600 || o.TravelHours != 12) {
			t.Fatalf("round-trip fare should not add outbound fare or double duration: %+v", o)
		}
	}
	if plan.SelectedReturn == nil || plan.SelectedReturn.BookingToken != "booking" {
		t.Fatal("selected return not retained")
	}
	if err := p.SelectFlight(context.Background(), plan, 0, ""); err != nil {
		t.Fatal(err)
	}
	if plan.SelectedReturn != nil {
		t.Fatal("changing departure must clear previous return")
	}
}

func TestSelectFlightRejectsInvalidAndExpiredSelections(t *testing.T) {
	p, plan := selectionPlan(t)
	before := plan.TotalUSD
	for _, index := range []int{-1, 99} {
		if err := p.SelectFlight(context.Background(), plan, index, ""); err == nil {
			t.Error("expected invalid index error")
		}
	}
	if err := p.SelectFlight(context.Background(), plan, 1, "expired"); err == nil {
		t.Error("expected expired return error")
	}
	if plan.TotalUSD != before || plan.SelectedReturn != nil {
		t.Fatal("failed selection mutated plan")
	}
	plan.Travelers = 0
	if _, err := FlightContext(plan, 0); err == nil {
		t.Fatal("older plan should require re-planning")
	}
}

func TestPlanRanksValueAheadOfCheapest(t *testing.T) {
	flights := []travel.FlightQuote{
		{Carrier: "Long layover", PriceUSD: 209.5, DurationMin: 1658, Stops: 1},
		{Carrier: "Nonstop", PriceUSD: 237, DurationMin: 108},
	}
	travel.RankFlights(flights)
	if flights[0].Carrier != "Nonstop" || len(flights) != 2 {
		t.Fatalf("value rank = %+v", flights)
	}
}
