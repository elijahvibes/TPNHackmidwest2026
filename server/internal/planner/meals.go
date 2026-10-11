package planner

import (
	"fmt"
	"strings"

	"trailmix/internal/travel"
)

// Three meals a day. Breakfast comes from cafés and bakeries (Category
// "breakfast"); lunch and dinner from restaurants. Lunch is priced at 75% of
// a place's dinner price, the usual gap between lunch and dinner menus.

type dayMeals struct {
	Breakfast, Lunch, Dinner *travel.Place
}

const lunchFactor = 0.75

func isBreakfastPlace(p travel.Place) bool { return p.Category == "breakfast" }

// splitFood separates breakfast spots from restaurants.
func splitFood(food []travel.Place) (breakfast, restaurants []travel.Place) {
	for _, p := range food {
		if isBreakfastPlace(p) {
			breakfast = append(breakfast, p)
		} else {
			restaurants = append(restaurants, p)
		}
	}
	return
}

// mealsWanted says which meals a day gets. Day 1 starts on the road at 08:00
// (no breakfast) and has lunch only when the traveler arrives by 13:00; the
// last day has breakfast and, with a late trip home, lunch.
func mealsWanted(i, n int, arrival int, homeLegAt int) (breakfast, lunch, dinner bool) {
	first, last := i == 0, i == n-1
	switch {
	case first && last:
		return false, false, false
	case first:
		return false, arrival <= 13*60, arrival <= dinnerCutoff
	case last:
		return true, homeLegAt >= 14*60, false
	default:
		return true, true, true
	}
}

// planMeals assigns meals deterministically, best-ranked first, and never
// sends the traveler to the same place twice: dinners are placed first,
// then lunches from what is left, and a meal with no place left stays open.
// It is the rule-based fallback and fills whatever the Writer's outline left out.
func planMeals(plan *Plan, n int, arrival int, homeLegAt int) []dayMeals {
	breakfast, restaurants := splitFood(plan.Food)
	out := make([]dayMeals, n)
	bi, ri := 0, 0
	nextB := func() *travel.Place {
		if bi >= len(breakfast) {
			return nil
		}
		p := breakfast[bi]
		bi++
		return &p
	}
	nextR := func() *travel.Place {
		if ri >= len(restaurants) {
			return nil
		}
		p := restaurants[ri]
		ri++
		return &p
	}
	for i := 0; i < n; i++ {
		if _, _, d := mealsWanted(i, n, arrival, homeLegAt); d {
			out[i].Dinner = nextR()
		}
	}
	for i := 0; i < n; i++ {
		b, l, _ := mealsWanted(i, n, arrival, homeLegAt)
		if b {
			out[i].Breakfast = nextB()
		}
		if l {
			out[i].Lunch = nextR()
		}
	}
	return out
}

// mealCost is the group's cost for a day's meals.
func mealCost(m dayMeals, travelers int) float64 {
	t := float64(max(1, travelers))
	c := 0.0
	if m.Breakfast != nil {
		c += m.Breakfast.PriceUSD * t
	}
	if m.Lunch != nil {
		c += m.Lunch.PriceUSD * lunchFactor * t
	}
	if m.Dinner != nil {
		c += m.Dinner.PriceUSD * t
	}
	return round2(c)
}

func mealTitle(slot string, p travel.Place) string {
	return strings.ToUpper(slot[:1]) + slot[1:] + " at " + p.Name
}

// mealItem builds the itinerary line for one meal.
func mealItem(slot, hhmm string, p travel.Place, travelers int) DayItem {
	cost := p.PriceUSD * float64(max(1, travelers))
	if slot == "lunch" {
		cost *= lunchFactor
	}
	detail := strings.TrimSpace(p.Note)
	if detail != "" {
		detail += "; "
	}
	detail += fmt.Sprintf("%s hours and availability unverified", slot)
	return DayItem{Time: hhmm, Title: mealTitle(slot, p), Detail: detail, CostUSD: round2(cost), CostUnknown: p.PriceUnknown, Category: "food"}
}

// mealSlot recognizes a meal line by its title.
func mealSlot(it DayItem) string {
	if it.Category != "food" {
		return ""
	}
	t := strings.ToLower(it.Title)
	for _, s := range []string{"breakfast", "lunch", "dinner", "brunch"} {
		if strings.HasPrefix(t, s) {
			if s == "brunch" {
				return "breakfast"
			}
			return s
		}
	}
	return "dinner" // an untagged meal is the evening one
}
