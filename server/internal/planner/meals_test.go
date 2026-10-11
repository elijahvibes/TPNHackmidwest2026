package planner

import (
	"testing"

	"trailmix/internal/travel"
)

func TestPlanMealsCoversEveryDay(t *testing.T) {
	plan := &Plan{Travelers: 2, Food: []travel.Place{
		{Name: "Snooze", Category: "breakfast", PriceUSD: 14},
		{Name: "Buckhorn Exchange", Category: "food", PriceUSD: 40},
		{Name: "Pasque", Category: "food", PriceUSD: 25},
	}}
	// 4 days, arrive 19:11 on day 1, drive home at noon on day 4.
	m := planMeals(plan, 4, 19*60+11, 12*60)
	if m[0].Breakfast != nil || m[0].Lunch != nil || m[0].Dinner == nil {
		t.Errorf("day 1 should be dinner only: %+v", m[0])
	}
	if m[1].Breakfast == nil || m[1].Dinner == nil || m[2].Breakfast != nil {
		t.Errorf("middle days get meals while places last (one café here): %+v %+v", m[1], m[2])
	}
	if m[3].Lunch != nil || m[3].Dinner != nil {
		t.Errorf("last day with a noon departure: breakfast only: %+v", m[3])
	}
	// Two restaurants for five restaurant meals: dinners are placed first,
	// best-ranked first, and nobody eats at the same place twice.
	if m[0].Dinner.Name != "Buckhorn Exchange" || m[1].Dinner.Name != "Pasque" || m[2].Dinner != nil || m[1].Lunch != nil {
		t.Errorf("dinners first, no reuse: %+v %+v %+v", m[0], m[1], m[2])
	}
	if c := mealCost(m[1], 2); c != 14*2+25*2 {
		t.Errorf("day 2 meal cost %.2f", c)
	}
	if c := mealCost(dayMeals{Lunch: &plan.Food[2]}, 2); c != 25*0.75*2 {
		t.Errorf("lunch rate: %.2f", c)
	}
	if mealSlot(DayItem{Category: "food", Title: "Lunch at Pasque"}) != "lunch" || mealSlot(DayItem{Category: "food", Title: "Pasque"}) != "dinner" || mealSlot(DayItem{Category: "activity", Title: "Lunch Park"}) != "" {
		t.Error("mealSlot")
	}
	if it := mealItem("lunch", "12:30", plan.Food[2], 2); it.Title != "Lunch at Pasque" || it.CostUSD != 37.5 {
		t.Errorf("lunch item: %+v", it)
	}
}
