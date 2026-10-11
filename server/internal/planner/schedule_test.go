package planner

import (
	"strings"
	"testing"

	"trailmix/internal/travel"
)

func drivePlan(durationMin int) *Plan {
	return &Plan{RecommendedMode: "drive", Nights: 3, Drive: travel.DriveEstimate{DurationMin: durationMin}}
}

func times(d Day) string {
	var out []string
	for _, it := range d.Items {
		out = append(out, it.Time+" "+it.Title)
	}
	return strings.Join(out, " | ")
}

func TestNormalizeArrivalDay(t *testing.T) {
	plan := drivePlan(636) // 10h36m: arrives 18:36
	days := []Day{{Day: 1, Items: []DayItem{
		{Time: "08:00", Title: "Depart Kansas City", Category: "transport"},
		{Time: "16:00", Title: "Check in: Test hotel", Category: "lodging"},
		{Time: "12:30", Title: "Dinner at Cafe Gio", Category: "food"},
		{Time: "14:00", Title: "Civic Center Park", Category: "activity"},
	}}, {Day: 2}, {Day: 3}, {Day: 4, Items: []DayItem{{Time: "12:00", Title: "Drive back to Kansas City", Category: "transport"}}}}
	notes := normalizeSchedule(plan, days)
	got := times(days[0])
	// Evening arrival: the park is dropped (no daylight left), dinner follows check-in.
	want := "08:00 Depart Kansas City | 18:36 Check in: Test hotel | 19:21 Dinner at Cafe Gio"
	if got != want {
		t.Errorf("day 1:\n got %s\nwant %s\nnotes %v", got, want, notes)
	}
}

func TestNormalizeDropsWhatCannotFit(t *testing.T) {
	plan := drivePlan(13 * 60) // arrives 21:00: too late for a park or dinner
	days := []Day{{Day: 1, Items: []DayItem{
		{Time: "08:00", Title: "Depart", Category: "transport"},
		{Time: "16:00", Title: "Check in: Test hotel", Category: "lodging"},
		{Time: "19:00", Title: "Dinner at Cafe Gio", Category: "food"},
		{Time: "14:00", Title: "Civic Center Park", Category: "activity"},
	}}, {Day: 2}}
	normalizeSchedule(plan, days)
	if got := times(days[0]); got != "08:00 Depart | 21:00 Check in: Test hotel" {
		t.Errorf("day 1: %s", got)
	}
}

func TestNormalizeDinnerHoursAndSpacing(t *testing.T) {
	plan := drivePlan(120)
	days := []Day{{Day: 1}, {Day: 2, Items: []DayItem{
		{Time: "10:00", Title: "Denver Art Museum", Category: "activity"},
		{Time: "10:30", Title: "Civic Center Park", Category: "activity"},
		{Time: "13:00", Title: "Dinner at Pasque", Category: "food"},
		{Time: "07:00", Title: "Buffalo Bill Museum", Category: "activity"},
	}}, {Day: 3}}
	normalizeSchedule(plan, days)
	if got := times(days[1]); got != "10:00 Denver Art Museum | 11:30 Buffalo Bill Museum | 13:00 Civic Center Park | 18:30 Dinner at Pasque" {
		t.Errorf("day 2: %s", got)
	}
}

func TestNormalizeLongDriveHomeLeavesByNoon(t *testing.T) {
	plan := drivePlan(621) // 10h21m home
	days := []Day{{Day: 1}, {Day: 2, Items: []DayItem{
		{Time: "10:00", Title: "Colorado State Capitol", Category: "activity"},
		{Time: "17:30", Title: "Drive back to Kansas City", Category: "transport"},
	}}}
	notes := normalizeSchedule(plan, days)
	// 17:30 would arrive at 04:00; the leg is pulled to noon and the visit is
	// started early enough to still fit (12:00 - 45 min - 2 h = 09:15).
	if got := times(days[1]); got != "09:15 Colorado State Capitol | 12:00 Drive back to Kansas City" {
		t.Errorf("day 2: %s (%v)", got, notes)
	}
	// A home leg the writer put at 08:00 gets the same treatment.
	days = []Day{{Day: 1}, {Day: 2, Items: []DayItem{
		{Time: "10:00", Title: "Colorado State Capitol", Category: "activity"},
		{Time: "08:00", Title: "Drive back to Kansas City", Category: "transport"},
	}}}
	normalizeSchedule(plan, days)
	if got := times(days[1]); got != "09:15 Colorado State Capitol | 12:00 Drive back to Kansas City" {
		t.Errorf("early home leg: %s", got)
	}
}

func TestNormalizeLastDayBeforeFlightHome(t *testing.T) {
	plan := &Plan{RecommendedMode: "fly", Nights: 1, Flights: []travel.FlightQuote{{DurationMin: 120}}}
	days := []Day{{Day: 1}, {Day: 2, Items: []DayItem{
		{Time: "09:00", Title: "MacIntosh Park", Category: "activity"},
		{Time: "11:00", Title: "Fly back to MCI", Category: "transport"},
	}}}
	notes := normalizeSchedule(plan, days)
	// park 09:00 + 2 h visit + 3 h lead = 14:00 → the flight slides later rather than dropping the park
	if got := times(days[1]); got != "09:00 MacIntosh Park | 14:00 Fly back to MCI" {
		t.Errorf("day 2: %s (%v)", got, notes)
	}
	days = []Day{{Day: 1}, {Day: 2, Items: []DayItem{
		{Time: "16:00", Title: "MacIntosh Park", Category: "activity"},
		{Time: "11:00", Title: "Fly back to MCI", Category: "transport"},
	}}}
	normalizeSchedule(plan, days)
	// 16:00 cannot work before any flight; the park is pulled to 13:00 and the
	// flight slides to the 18:00 limit rather than losing the stop.
	if got := times(days[1]); got != "13:00 MacIntosh Park | 18:00 Fly back to MCI" {
		t.Errorf("late activity should be pulled earlier: %s", got)
	}
}
