package planner

import (
	"fmt"
	"math"
	"strings"
	"time"

	"trailmix/internal/travel"
)

func normalizeMobility(req *Request) error {
	if req.LocalTransport == "" {
		req.LocalTransport = "auto"
	}
	switch req.LocalTransport {
	case "auto", "rental_car", "transit_rideshare", "walking":
	default:
		return fmt.Errorf("local_transport must be auto, rental_car, transit_rideshare or walking")
	}
	if req.ActivityRadiusMiles == 0 {
		req.ActivityRadiusMiles = 25
		if req.LocalTransport == "walking" {
			req.ActivityRadiusMiles = 2
		}
	}
	if math.IsNaN(req.ActivityRadiusMiles) || math.IsInf(req.ActivityRadiusMiles, 0) ||
		req.ActivityRadiusMiles < 1 || req.ActivityRadiusMiles > 150 {
		return fmt.Errorf("activity_radius_miles must be between 1 and 150")
	}
	return nil
}

func effectiveTransport(plan *Plan) string {
	return transportFor(plan.RecommendedMode, plan.LocalTransport)
}

// transportFor resolves the requested local transport against the trip mode:
// driving in means the traveler has their own car at the destination.
func transportFor(mode, local string) string {
	if mode == "drive" && (local == "auto" || local == "" || local == "rental_car") {
		return "own car"
	}
	if local == "" || local == "auto" {
		return "transit/rideshare"
	}
	switch local {
	case "rental_car":
		return "rental car"
	case "walking":
		return "walking"
	default:
		return "transit/rideshare"
	}
}

func mobilityNote(plan *Plan) string {
	costs := "fares and transfers"
	switch effectiveTransport(plan) {
	case "own car":
		costs = "parking and extra local fuel"
	case "rental car":
		costs = "rental, fares, parking and extra fuel"
	case "walking":
		costs = "any additional transfers"
	}
	note := fmt.Sprintf(" Explore within %.0f miles of the destination using %s; %s are included as an estimate, and routes and availability are not verified.",
		plan.ActivityRadiusMiles, effectiveTransport(plan), costs)
	if plan.LocalTransport == "walking" && plan.ActivityRadiusMiles > 3 {
		note += " Your range exceeds a typical walking area; distant activities need additional transportation."
	}
	return note
}

func distance(a, b travel.LatLng) float64 {
	const r = 3958.8
	lat1, lat2 := a.Lat*math.Pi/180, b.Lat*math.Pi/180
	dlat, dlon := lat2-lat1, (b.Lng-a.Lng)*math.Pi/180
	h := math.Pow(math.Sin(dlat/2), 2) + math.Cos(lat1)*math.Cos(lat2)*math.Pow(math.Sin(dlon/2), 2)
	return 2 * r * math.Asin(math.Sqrt(math.Min(1, h)))
}

func localTravelDetail(plan *Plan, place travel.Place) string {
	miles := distance(plan.DestLoc, place.Location)
	detail := fmt.Sprintf("%s; %.1f miles straight-line from destination center", effectiveTransport(plan), miles)
	if miles > 20 {
		detail += "; plan as an out-of-city excursion"
	}
	if effectiveTransport(plan) == "walking" && miles > 3 {
		detail += "; beyond typical walking range, additional transportation needed"
	}
	return detail + "; travel time, cost and opening hours not verified"
}

func nearbyFood(food []travel.Place, at travel.LatLng) *travel.Place {
	if len(food) == 0 {
		return nil
	}
	best := 0
	for i := 1; i < len(food); i++ {
		if distance(at, food[i].Location) < distance(at, food[best].Location) {
			best = i
		}
	}
	return &food[best]
}

func groupActivities(in []travel.Place) []travel.Place {
	if len(in) < 2 {
		return in
	}
	remaining := append([]travel.Place(nil), in[1:]...)
	out := []travel.Place{in[0]}
	for len(remaining) > 0 {
		last := out[len(out)-1]
		best := 0
		for i := 1; i < len(remaining); i++ {
			if distance(last.Location, remaining[i].Location) < distance(last.Location, remaining[best].Location) {
				best = i
			}
		}
		out = append(out, remaining[best])
		remaining = append(remaining[:best], remaining[best+1:]...)
	}
	return out
}

func dailyActivities(activities []travel.Place, center travel.LatLng, weather travel.Weather, start time.Time, nights int) [][]travel.Place {
	days := make([][]travel.Place, nights+1)
	remaining := append([]travel.Place(nil), activities...)
	for day := 1; day < nights && len(remaining) > 0; day++ {
		if w := weatherOn(weather, start.AddDate(0, 0, day).Format("2006-01-02")); w != nil && w.PrecipPct >= 50 {
			remaining = indoorFirst(remaining)
		}
		anchor := remaining[0]
		days[day] = []travel.Place{anchor}
		remaining = remaining[1:]
		if distance(center, anchor.Location) <= 20 && len(remaining) > 0 {
			best := -1
			for i, candidate := range remaining {
				if !differentNearbyActivities(anchor, candidate) {
					continue
				}
				if distance(center, candidate.Location) <= 20 &&
					(best < 0 || distance(anchor.Location, candidate.Location) < distance(anchor.Location, remaining[best].Location)) {
					best = i
				}
			}
			if best >= 0 {
				days[day] = append(days[day], remaining[best])
				remaining = append(remaining[:best], remaining[best+1:]...)
			}
		}
	}
	for _, a := range remaining {
		if distance(center, a.Location) <= 3 {
			days[nights] = []travel.Place{a}
			break
		}
	}
	return days
}

// plannedPlaceCost is what the rule-based itinerary will charge for places
// and meals; arrival (minutes after midnight on day 1) decides whether day 1
// gets lunch, exactly as fallbackItinerary decides it.
func plannedPlaceCost(activities, food []travel.Place, center travel.LatLng, weather travel.Weather, start time.Time, nights, travelers int, arrival int) float64 {
	cost := 0.0
	days := dailyActivities(activities, center, weather, start, nights)
	for _, places := range days {
		for _, p := range places {
			cost += p.PriceUSD * float64(travelers)
		}
	}
	for _, m := range planMeals(&Plan{Food: food}, nights+1, arrival, 12*60) {
		cost += mealCost(m, travelers)
	}
	return round2(cost)
}

func dailyMeals(food []travel.Place, center travel.LatLng, days [][]travel.Place, nights int) []*travel.Place {
	meals := make([]*travel.Place, nights+1)
	used := map[string]bool{}
	for day := 0; day < nights; day++ {
		area := center
		if len(days[day]) > 0 {
			area = days[day][len(days[day])-1].Location
		}
		for i := range food {
			candidate := &food[i]
			name := strings.ToLower(strings.TrimSpace(candidate.Name))
			if used[name] {
				continue
			}
			if meals[day] == nil || distance(area, candidate.Location) < distance(area, meals[day].Location) {
				meals[day] = candidate
			}
		}
		if meals[day] != nil {
			used[strings.ToLower(strings.TrimSpace(meals[day].Name))] = true
		}
	}
	return meals
}

func destinationTransfer(plan *Plan) DayItem {
	detail := "Check parking and allow for local fuel; costs excluded."
	switch effectiveTransport(plan) {
	case "rental car":
		detail = "Reserve a rental car and confirm pickup, parking and fuel costs; availability and costs unverified."
	case "transit/rideshare":
		detail = "Check transit coverage or rideshare availability for the hotel and activity areas; airport transfers and fares unverified."
	case "walking":
		detail = "Check walkable access to the hotel and activities."
		if plan.RecommendedMode == "fly" {
			detail += " Arrange an airport transfer separately; walking access from the airport is not assumed."
		}
	}
	return DayItem{Time: "--:--", Title: "Arrange destination transportation: " + effectiveTransport(plan),
		Detail: detail + " Travel times are not verified.", Category: "transport", CostUnknown: true}
}
