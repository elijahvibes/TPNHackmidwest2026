package travel

import "strings"

// Typical per-person US prices by provider category, used when a place comes
// without a price (Geoapify and OpenStreetMap never have one). They are
// estimates and every place priced this way is marked PriceEstimated so the
// UI can say so. Order matters: the first matching rule wins.
var priceRules = []struct {
	match []string
	usd   float64
}{
	{[]string{"entertainment.theme_park", "amusement_park", "theme_park"}, 65},
	{[]string{"entertainment.aquarium", "aquarium"}, 32},
	{[]string{"entertainment.zoo", "zoo"}, 22},
	{[]string{"entertainment.museum", "museum", "art_gallery", "gallery"}, 18},
	{[]string{"entertainment.planetarium", "planetarium"}, 15},
	{[]string{"tourism.attraction", "tourist_attraction", "attraction"}, 15},
	{[]string{"historic", "historical_landmark", "monument", "memorial", "tourism.sights"}, 0},
	{[]string{"leisure.park", "park", "national_park", "natural", "nature_reserve", "viewpoint", "hiking_area", "campground"}, 0},
	{[]string{"catering.fast_food", "fast_food"}, 12},
	{[]string{"catering.cafe", "cafe", "coffee", "bakery", "food_and_drink.bakery"}, 12},
	{[]string{"catering.bar", "catering.pub", "bar", "pub", "night_club", "nightclub"}, 18},
	{[]string{"catering.restaurant", "restaurant"}, 28},
	{[]string{"service.vehicle.fuel", "gas_station", "fuel", "rest_area", "rest_stop", "services"}, 0},
}

// kindDefaults cover a place whose categories say nothing useful.
var kindDefaults = map[string]float64{
	"attraction": 15, "museums": 18, "family": 25, "history": 0, "outdoors": 0, "nightlife": 18, "food": 28, "breakfast": 12, "rest_stop": 0,
}

// EstimatePrice returns a typical per-person price for a place with no known
// price. ok is false only when nothing about the place is recognizable.
func EstimatePrice(p Place) (usd float64, ok bool) {
	for _, r := range priceRules {
		for _, c := range p.Categories {
			lc := strings.ToLower(c)
			for _, m := range r.match {
				if lc == m || strings.HasPrefix(lc, m+".") || strings.Contains(lc, "."+m) {
					return r.usd, true
				}
			}
		}
	}
	name := strings.ToLower(p.Name)
	for _, r := range priceRules {
		for _, m := range r.match {
			if strings.Contains(m, ".") || strings.Contains(m, "_") {
				continue
			}
			if strings.Contains(name, m) {
				return r.usd, true
			}
		}
	}
	if v, found := kindDefaults[p.Category]; found {
		return v, true
	}
	return 0, false
}

// FillPriceEstimates gives every place without a price an estimated one and
// marks it. Places that already carry a price are left alone.
func FillPriceEstimates(places []Place) {
	for i := range places {
		p := &places[i]
		if p.PriceUSD > 0 || !p.PriceUnknown {
			continue
		}
		if v, ok := EstimatePrice(*p); ok {
			p.PriceUSD, p.PriceUnknown, p.PriceEstimated = v, false, true
		}
	}
}

// LocalTransportEstimate is a rough per-day cost for the whole group of
// getting around the destination, by the transport the plan settled on.
// Rideshare and rental numbers are national averages; transit is a day pass.
func LocalTransportEstimate(transport string, travelers, days int, flying bool) (perDay float64, note string) {
	t := float64(max(1, travelers))
	switch transport {
	case "own car":
		perDay, note = 18, "parking and local fuel, about $18/day"
	case "rental car":
		perDay, note = 55+18+8, "mid-size rental about $55/day plus parking and fuel"
	case "walking":
		perDay, note = 0, "walking; no daily cost"
	default: // transit/rideshare
		perDay = 12*t + 20
		note = "a transit day pass per person plus roughly one rideshare a day"
	}
	if flying && transport != "rental car" {
		// Airport transfers each way, spread across the trip.
		perDay += 70 / float64(max(1, days))
		note += ", plus airport transfers"
	}
	return round2(perDay), note
}
