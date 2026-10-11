package planner

import (
	"fmt"
	"sort"
	"strings"
)

// Schedule physics the model (and the rule-based fallback) must respect:
//
//   - Nothing at the destination happens before the traveler arrives.
//   - Dinner is in the evening. Activities happen in daylight hours.
//   - On the last day everything ends in time for the trip home.
//   - Consecutive timed stops are spaced out.
//
// normalizeSchedule enforces these deterministically, moving or dropping
// items, so a 10-hour drive that leaves at 08:00 never checks in at 16:00 and
// "dinner" never lands at 12:30.

const (
	departMin     = 8 * 60 // every trip leaves home at 08:00
	breakfastOpen = 7 * 60 // breakfast window
	breakfastEnd  = 10 * 60
	breakfastDef  = 8*60 + 30
	lunchOpen     = 11*60 + 30 // lunch window
	lunchEnd      = 14 * 60
	lunchDef      = 12*60 + 30
	dinnerEarly   = 17*60 + 30 // dinner window
	dinnerLate    = 20*60 + 30 //
	dinnerDefault = 18*60 + 30 //
	dinnerCutoff  = 21*60 + 30 // arrive later than this → no dinner
	activityOpen  = 9 * 60     // activity window
	activityClose = 20 * 60    //
	activityLast  = 17 * 60    // an arrival-day activity must start by now (done before dinner)
	flyLead       = 3 * 60     // be done this long before a flight home
	driveLead     = 45         // ... or before the drive home
	spacing       = 90         // minutes between consecutive timed stops
	latestFlyHome = 18 * 60    // the trip-home leg can slide this late
	latestDrive   = 16 * 60    //
)

func parseHM(s string) (int, bool) {
	if len(s) != 5 || s[2] != ':' {
		return 0, false
	}
	var h, m int
	if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

func fmtHM(min int) string {
	if min < 0 {
		min = 0
	}
	if min > 23*60+59 {
		min = 23*60 + 59
	}
	return fmt.Sprintf("%02d:%02d", min/60, min%60)
}

// arrivalMinutes is when the traveler can first do anything at the
// destination on day 1 (minutes after midnight), from the real travel data.
func arrivalMinutes(plan *Plan, stops int) int {
	at := departMin
	if plan.RecommendedMode == "drive" || len(plan.Flights) == 0 {
		at += plan.Drive.DurationMin + stops*25
	} else {
		f := plan.Flights[min(plan.SelectedFlight, len(plan.Flights)-1)]
		at += f.DurationMin + 150 // check-in, security, bags
		if a := plan.OriginAirport; a != nil && a.DistanceMiles > 20 {
			at += int(a.DistanceMiles / 45 * 60)
		}
		if a := plan.DestAirport; a != nil && a.DistanceMiles > 20 {
			at += int(a.DistanceMiles / 45 * 60)
		}
	}
	return at
}

func isDinner(it DayItem) bool    { return mealSlot(it) == "dinner" }
func isBreakfast(it DayItem) bool { return mealSlot(it) == "breakfast" }
func isLunch(it DayItem) bool     { return mealSlot(it) == "lunch" }

func isHomeLeg(it DayItem) bool {
	t := strings.ToLower(it.Title)
	return it.Category == "transport" && (strings.HasPrefix(t, "fly back") || strings.HasPrefix(t, "drive back") || strings.HasPrefix(t, "drive home"))
}

func isOutboundLeg(it DayItem) bool {
	t := strings.ToLower(it.Title)
	return it.Category == "transport" && (strings.HasPrefix(t, "fly ") || strings.HasPrefix(t, "depart") || strings.HasPrefix(t, "travel to"))
}

// normalizeSchedule fixes times in place and drops what cannot fit. It
// returns a short note per change, for the Reviewer's audit report.
func normalizeSchedule(plan *Plan, days []Day) []string {
	var notes []string
	n := len(days)
	for di := range days {
		d := &days[di]
		first, last := di == 0, di == n-1
		arrival := 0
		if first {
			stops := 0
			for _, it := range d.Items {
				if it.Category == "stop" {
					stops++
				}
			}
			arrival = arrivalMinutes(plan, stops)
		}

		// Pass 1: windows. Untimed items ("--:--") are left alone.
		kept := d.Items[:0]
		for _, it := range d.Items {
			t, ok := parseHM(it.Time)
			if !ok {
				kept = append(kept, it)
				continue
			}
			move := func(to int, why string) {
				if to != t {
					notes = append(notes, fmt.Sprintf("day %d: %s moved %s → %s (%s)", d.Day, it.Title, fmtHM(t), fmtHM(to), why))
					t = to
				}
			}
			switch {
			case first && isOutboundLeg(it):
				// the outbound legs keep the model's times (08:00 and onward)
			case first && it.Category == "lodging":
				if t < arrival {
					move(arrival, "after arrival")
				}
			case first && it.Category == "stop":
				if t < departMin+30 {
					move(departMin+30+t%7*10, "on the road")
				}
			case first && it.Category == "activity":
				if arrival+60 > activityLast {
					notes = append(notes, fmt.Sprintf("day %d: %s dropped (arrival %s is too late)", d.Day, it.Title, fmtHM(arrival)))
					continue
				}
				if t < arrival+60 {
					move(arrival+60, "after arrival")
				}
			case first && isBreakfast(it):
				notes = append(notes, fmt.Sprintf("day %d: %s dropped (on the road at breakfast time)", d.Day, it.Title))
				continue
			case first && isLunch(it):
				if arrival+30 > lunchEnd {
					notes = append(notes, fmt.Sprintf("day %d: %s dropped (arrival %s is after lunch)", d.Day, it.Title, fmtHM(arrival)))
					continue
				}
				if t < arrival+30 || t > lunchEnd {
					move(max(lunchDef, arrival+30), "lunch after arrival")
				}
			case first && isDinner(it):
				earliest := max(dinnerDefault, arrival+45)
				if earliest > dinnerCutoff {
					notes = append(notes, fmt.Sprintf("day %d: %s dropped (arrival %s is too late for dinner)", d.Day, it.Title, fmtHM(arrival)))
					continue
				}
				if t < earliest || t > dinnerLate {
					move(min(earliest, dinnerLate), "dinner after arrival")
				}
				if t < earliest {
					move(earliest, "dinner after arrival")
				}
			case isDinner(it):
				if t < dinnerEarly || t > dinnerLate {
					move(dinnerDefault, "dinner hours")
				}
			case isBreakfast(it):
				if t < breakfastOpen || t > breakfastEnd {
					move(breakfastDef, "breakfast hours")
				}
			case isLunch(it):
				if t < lunchOpen || t > lunchEnd {
					move(lunchDef, "lunch hours")
				}
			case it.Category == "activity" && !first:
				if t < activityOpen {
					move(activityOpen+60, "opening hours")
				} else if t > activityClose {
					move(activityClose-120, "closing hours")
				}
			}
			it.Time = fmtHM(t)
			kept = append(kept, it)
		}
		d.Items = kept

		// Pass 2: the trip home bounds the last day.
		if last && n > 1 {
			lead, latest := driveLead, latestDrive
			if plan.RecommendedMode == "fly" {
				lead, latest = flyLead, latestFlyHome
				if a := plan.DestAirport; a != nil && a.DistanceMiles > 20 {
					lead += int(a.DistanceMiles / 45 * 60)
				}
			}
			homeIdx, homeAt := -1, 0
			for i, it := range d.Items {
				if isHomeLeg(it) {
					if t, ok := parseHM(it.Time); ok {
						homeIdx, homeAt = i, t
					}
				}
			}
			if plan.RecommendedMode != "fly" && plan.Drive.DurationMin > 6*60 {
				latest = 12 * 60 // a long drive home leaves by noon, not after dinner
			}
			if homeIdx >= 0 && homeAt > latest {
				notes = append(notes, fmt.Sprintf("day %d: trip home moved %s → %s (leave early enough to arrive at a sane hour)", d.Day, fmtHM(homeAt), fmtHM(latest)))
				homeAt = latest
			}
			if homeIdx >= 0 {
				kept := d.Items[:0]
				for i, it := range d.Items {
					t, ok := parseHM(it.Time)
					if i == homeIdx || !ok || it.Category == "transport" {
						kept = append(kept, it)
						continue
					}
					// A stop takes about two hours (a meal one), then the home leg's lead time.
					dur := 120
					if it.Category == "food" {
						dur = 60
					}
					need := t + dur + lead
					if need > homeAt {
						switch {
						case need <= latest:
							notes = append(notes, fmt.Sprintf("day %d: trip home moved %s → %s to fit %s", d.Day, fmtHM(homeAt), fmtHM(need), it.Title))
							homeAt = need
						case latest-lead-dur >= activityOpen-60:
							// Start the stop early enough instead of dropping it.
							early := latest - lead - dur
							notes = append(notes, fmt.Sprintf("day %d: %s moved %s → %s and trip home %s → %s", d.Day, it.Title, fmtHM(t), fmtHM(early), fmtHM(homeAt), fmtHM(latest)))
							it.Time = fmtHM(early)
							homeAt = latest
						default:
							notes = append(notes, fmt.Sprintf("day %d: %s dropped (no time before the trip home)", d.Day, it.Title))
							continue
						}
					}
					kept = append(kept, it)
				}
				d.Items = kept
				for i := range d.Items {
					if isHomeLeg(d.Items[i]) {
						d.Items[i].Time = fmtHM(homeAt)
					}
				}
			}
		}

		// Pass 3: order and spacing. Untimed items sink to the end.
		sort.SliceStable(d.Items, func(a, b int) bool {
			ta, oka := parseHM(d.Items[a].Time)
			tb, okb := parseHM(d.Items[b].Time)
			if !oka || !okb {
				return oka && !okb
			}
			return ta < tb
		})
		// Spacing applies between the stops that take time (activities and
		// meals); check-in and travel legs are not padded. A meal needs an
		// hour, an activity ninety minutes, before the next stop.
		prev, prevFood := -1, false
		for i := range d.Items {
			it := &d.Items[i]
			t, ok := parseHM(it.Time)
			if !ok || (it.Category != "activity" && it.Category != "food") {
				continue
			}
			gap := spacing
			if prevFood {
				gap = 60
			}
			if prev >= 0 && t < prev+gap {
				t = prev + gap
				it.Time = fmtHM(t)
			}
			prev, prevFood = t, it.Category == "food"
		}
		sort.SliceStable(d.Items, func(a, b int) bool {
			ta, oka := parseHM(d.Items[a].Time)
			tb, okb := parseHM(d.Items[b].Time)
			if !oka || !okb {
				return oka && !okb
			}
			return ta < tb
		})
	}
	return notes
}
