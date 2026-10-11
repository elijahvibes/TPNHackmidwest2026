package planner

import (
	"fmt"
	"strings"
	"time"

	"trailmix/internal/travel"
)

// Rain handling is deterministic: the Writer's outline is adjusted so a day
// with a 50%+ chance of rain gets indoor places and its outdoor ones move to
// the driest day that can take them. The model is told about it in the theme.

const rainyPct = 50

// isOutdoor says whether a place is mostly outside: parks, gardens, trails,
// zoos, viewpoints, amphitheatres. Museums, galleries, halls and the like are
// indoor; anything unrecognizable is treated as indoor (no reason to move it).
func isOutdoor(p travel.Place) bool {
	switch activityType(p) {
	case "park", "nature", "zoo", "theme park":
		return true
	case "museum", "nightlife":
		return false
	}
	n := strings.ToLower(p.Name)
	for _, k := range []string{"park", "garden", "trail", "lake", "falls", "canyon", "mountain", "beach", "amphitheat", "overlook", "viewpoint", "reservoir", "creek", "river", "arboretum", "cemetery", "hike", "ski"} {
		if strings.Contains(n, k) {
			return true
		}
	}
	for _, c := range p.Categories {
		lc := strings.ToLower(c)
		if strings.Contains(lc, "park") || strings.Contains(lc, "natural") || strings.Contains(lc, "beach") || strings.Contains(lc, "garden") {
			return true
		}
	}
	return p.Category == "outdoors"
}

// rainPct is the forecast chance of rain for a trip date; -1 when the
// forecast does not cover it (NWS only looks 7 days out).
func rainPct(w travel.Weather, date string) int {
	if d := weatherOn(w, date); d != nil {
		return d.PrecipPct
	}
	return -1
}

// rainAwareAssign moves outdoor activities off rainy days. For each rainy
// middle day it tries, per outdoor place: a swap with an indoor place on a
// dry day (keeping each day's variety rule), else replacing it with an unused
// indoor candidate, else leaving it with a note. Returns what it changed.
func rainAwareAssign(assign []dayAssignment, plan *Plan, start time.Time) []string {
	var notes []string
	n := len(assign)
	date := func(i int) string { return start.AddDate(0, 0, i).Format("2006-01-02") }
	used := map[string]bool{}
	for _, a := range assign {
		for _, p := range a.Activities {
			used[p.Name] = true
		}
	}
	// Indoor candidates the outline did not use, best first.
	var spare []travel.Place
	for _, p := range plan.Activities {
		if !used[p.Name] && !isOutdoor(p) {
			spare = append(spare, p)
		}
	}
	fits := func(day dayAssignment, skip int, cand travel.Place) bool {
		for k, other := range day.Activities {
			if k != skip && !differentNearbyActivities(other, cand) {
				return false
			}
		}
		return true
	}

	for i := 1; i < n-1; i++ {
		pct := rainPct(plan.Weather, date(i))
		if pct < rainyPct {
			continue
		}
		changed := false
		for ai := 0; ai < len(assign[i].Activities); ai++ {
			out := assign[i].Activities[ai]
			if !isOutdoor(out) {
				continue
			}
			// 1. Swap with an indoor place on the driest day that has one.
			bestJ, bestK, bestPct := -1, -1, 101
			for j := 1; j < n-1; j++ {
				if j == i {
					continue
				}
				jp := rainPct(plan.Weather, date(j))
				if jp >= rainyPct {
					continue
				}
				if jp < 0 {
					jp = 40 // unknown: usable, but a known-dry day is better
				}
				for k, in := range assign[j].Activities {
					if isOutdoor(in) || !fits(assign[i], ai, in) || !fits(assign[j], k, out) {
						continue
					}
					if jp < bestPct {
						bestJ, bestK, bestPct = j, k, jp
					}
				}
			}
			if bestJ >= 0 {
				in := assign[bestJ].Activities[bestK]
				assign[i].Activities[ai], assign[bestJ].Activities[bestK] = in, out
				notes = append(notes, fmt.Sprintf("day %d (%d%% rain): %s swapped with %s from day %d", i+1, pct, out.Name, in.Name, bestJ+1))
				changed = true
				continue
			}
			// 2. Replace with an unused indoor candidate.
			for s := 0; s < len(spare); s++ {
				if fits(assign[i], ai, spare[s]) {
					notes = append(notes, fmt.Sprintf("day %d (%d%% rain): %s replaced by %s", i+1, pct, out.Name, spare[s].Name))
					assign[i].Activities[ai] = spare[s]
					spare = append(spare[:s], spare[s+1:]...)
					changed = true
					break
				}
			}
			if !changed {
				notes = append(notes, fmt.Sprintf("day %d (%d%% rain): %s kept, no indoor alternative", i+1, pct, out.Name))
			}
		}
		if changed && !strings.Contains(strings.ToLower(assign[i].Theme), "rain") {
			assign[i].Theme = fmt.Sprintf("Indoor day, %d%% chance of rain: %s", pct, strings.TrimSpace(assign[i].Theme))
		}
	}
	return notes
}
