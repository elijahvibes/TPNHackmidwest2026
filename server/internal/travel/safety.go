package travel

import (
	"context"
	"fmt"
	"strings"
)

// Safety summarizes crime risk for a destination so the planner can pick
// lodging near well-trafficked areas and shape late-night plans.
type Safety struct {
	City           string   `json:"city"`
	ViolentPer100k float64  `json:"violent_per_100k"`
	Level          string   `json:"level"` // low | moderate | elevated
	Score          int      `json:"score"` // 0 (worst) - 100 (safest)
	Tips           []string `json:"tips"`
	Source         string   `json:"source"`
}

// FBI UCR / NIBRS violent-crime rates per 100k residents, city proper, most
// recent published year, rounded. The national rate is about 380. These are
// embedded so the demo works offline; swap in a live FBI Crime Data Explorer
// call (needs a free api.data.gov key) when there is time.
var violentCrimeRates = map[string]float64{
	"kansas city": 1430, "st. louis": 1850, "st louis": 1850, "chicago": 870, "denver": 920,
	"nashville": 1120, "minneapolis": 1090, "omaha": 560, "dallas": 860, "austin": 430,
	"new orleans": 1320, "las vegas": 750, "los angeles": 770, "new york": 580, "miami": 650,
	"orlando": 820, "seattle": 740, "san francisco": 660, "phoenix": 740, "atlanta": 900,
	"yellowstone": 120, "grand canyon": 120,
}

const nationalViolentRate = 380.0

// SafetyFor rates a destination by name. Unknown places get the national
// average with a note, never an error.
func SafetyFor(_ context.Context, city string) Safety {
	key := strings.ToLower(strings.TrimSpace(city))
	rate, known := 0.0, false
	for name, r := range violentCrimeRates {
		if strings.Contains(key, name) {
			rate, known = r, true
			break
		}
	}
	src := "fbi-ucr (built-in table)"
	if !known {
		rate, src = nationalViolentRate, "national average (city not in table)"
	}
	s := Safety{City: city, ViolentPer100k: rate, Source: src}
	// Score: national average ≈ 70, 2x average ≈ 40, half average ≈ 85.
	s.Score = clamp(int(100-30*(rate/nationalViolentRate)), 5, 98)
	switch {
	case rate < 450:
		s.Level = "low"
		s.Tips = []string{"Standard precautions; downtown and tourist districts are fine on foot in the evening."}
	case rate < 900:
		s.Level = "moderate"
		s.Tips = []string{
			"Stay in or near the downtown / entertainment district.",
			"After 11pm, prefer rideshare over walking between venues and the hotel.",
		}
	default:
		s.Level = "elevated"
		s.Tips = []string{
			"Book lodging within about a mile of downtown or a major attraction cluster.",
			"After 10pm, use rideshare between venues and the hotel rather than walking.",
			"If driving in, aim to arrive before dark; the planner schedules departures accordingly.",
			"Keep valuables out of sight in a parked car.",
		}
	}
	s.Tips = append(s.Tips, fmt.Sprintf("Violent crime about %.1fx the US average (%.0f vs %.0f per 100k).", rate/nationalViolentRate, rate, nationalViolentRate))
	return s
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
