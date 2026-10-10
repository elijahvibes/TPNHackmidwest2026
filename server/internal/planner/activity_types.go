package planner

import (
	"fmt"
	"strings"

	"trailmix/internal/travel"
)

func activityType(place travel.Place) string {
	for _, group := range []struct {
		name       string
		categories []string
	}{
		{"museum", []string{"entertainment.museum", "museum", "art_gallery"}},
		{"zoo", []string{"entertainment.zoo", "zoo", "aquarium"}},
		{"theme park", []string{"entertainment.theme_park", "amusement_park"}},
		{"park", []string{"leisure.park", "park", "national_park", "botanical_garden"}},
		{"nature", []string{"natural", "hiking_area", "campground"}},
		{"sightseeing", []string{"tourism.sights", "tourist_attraction", "historical_landmark", "monument"}},
		{"nightlife", []string{"catering.bar", "catering.pub", "bar", "night_club"}},
	} {
		for _, category := range place.Categories {
			for _, prefix := range group.categories {
				if category == prefix || strings.HasPrefix(category, prefix+".") {
					return group.name
				}
			}
		}
	}
	if place.Category != "" {
		return place.Category
	}
	return "unknown"
}

func differentNearbyActivities(a, b travel.Place) bool {
	return distance(a.Location, b.Location) > 1 || activityType(a) != activityType(b)
}

func validateActivityVariety(days []Day, places []travel.Place) error {
	for _, day := range days {
		var scheduled []travel.Place
		for _, item := range day.Items {
			if item.Category != "activity" {
				continue
			}
			for _, place := range places {
				if strings.EqualFold(strings.TrimSpace(item.Title), strings.TrimSpace(place.Name)) {
					for _, previous := range scheduled {
						if !differentNearbyActivities(previous, place) {
							return fmt.Errorf("day %d repeats activity type %s within one mile", day.Day, activityType(place))
						}
					}
					scheduled = append(scheduled, place)
				}
			}
		}
	}
	return nil
}
