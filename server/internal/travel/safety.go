package travel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
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
// recent published year, rounded. The national rate is about 380. Cities are
// more specific than the live state figure, so a listed city keeps its rate.
var violentCrimeRates = map[string]float64{
	"kansas city": 1430, "st. louis": 1850, "st louis": 1850, "chicago": 870, "denver": 920,
	"nashville": 1120, "minneapolis": 1090, "omaha": 560, "dallas": 860, "austin": 430,
	"new orleans": 1320, "las vegas": 750, "los angeles": 770, "new york": 580, "miami": 650,
	"orlando": 820, "seattle": 740, "san francisco": 660, "phoenix": 740, "atlanta": 900,
	"yellowstone": 120, "grand canyon": 120,
}

const nationalViolentRate = 380.0

// FBI Crime Data Explorer, state-level violent crime. api.data.gov's public
// DEMO_KEY works (30 requests/hour), so no registration is needed; set
// FBI_API_KEY for a higher limit. Results are cached per state for a day.
var (
	fbiMu    sync.Mutex
	fbiCache = map[string]struct {
		at   time.Time
		rate float64
		year int
	}{}
	fbiHTTP = &http.Client{Timeout: 12 * time.Second}
)

// SafetyFor rates a destination by name ("Denver, CO"). A listed city uses its
// city-proper rate; otherwise the state's live FBI rate; otherwise the national
// average with a note. It never errors.
func SafetyFor(ctx context.Context, city string) Safety {
	key := strings.ToLower(strings.TrimSpace(city))
	rate, known := 0.0, false
	for name, r := range violentCrimeRates {
		if strings.Contains(key, name) {
			rate, known = r, true
			break
		}
	}
	state := ""
	if i := strings.LastIndex(city, ","); i >= 0 {
		state = StateCode(city[i+1:])
	}
	src := "fbi-ucr (built-in city table)"
	liveRate, liveYear, live := fbiStateRate(ctx, state)
	switch {
	case known && live:
		src = fmt.Sprintf("fbi-ucr city table · fbi-cde live: %s %d, %.0f per 100k statewide", state, liveYear, liveRate)
	case !known && live:
		rate, src = liveRate, fmt.Sprintf("fbi-cde (live, %s statewide %d)", state, liveYear)
	case !known:
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

// fbiStateRate returns the most recent full year's violent-crime rate per 100k
// for a state from the FBI CDE API (sum of the monthly rates).
func fbiStateRate(ctx context.Context, state string) (rate float64, year int, ok bool) {
	if state == "" || ctx == nil || Offline {
		return 0, 0, false
	}
	fbiMu.Lock()
	if c, hit := fbiCache[state]; hit && time.Since(c.at) < 24*time.Hour {
		fbiMu.Unlock()
		return c.rate, c.year, c.rate > 0
	}
	fbiMu.Unlock()

	apiKey := os.Getenv("FBI_API_KEY")
	if apiKey == "" {
		apiKey = "DEMO_KEY"
	}
	year = time.Now().Year() - 1
	for tries := 0; tries < 2; tries++ { // last year may not be published yet
		u := fmt.Sprintf("https://api.usa.gov/crime/fbi/cde/summarized/state/%s/violent-crime?from=01-%d&to=12-%d&API_KEY=%s", state, year, year, apiKey)
		cctx, cancel := context.WithTimeout(ctx, 12*time.Second)
		req, _ := http.NewRequestWithContext(cctx, "GET", u, nil)
		res, err := fbiHTTP.Do(req)
		if err != nil {
			cancel()
			return 0, 0, false
		}
		var out struct {
			Offenses struct {
				Rates map[string]map[string]float64 `json:"rates"`
			} `json:"offenses"`
		}
		err = json.NewDecoder(res.Body).Decode(&out)
		res.Body.Close()
		cancel()
		if err != nil || res.StatusCode != 200 {
			return 0, 0, false
		}
		for series, months := range out.Offenses.Rates {
			if !strings.HasSuffix(series, "Offenses") || len(months) < 6 {
				continue
			}
			sum := 0.0
			for _, v := range months {
				sum += v
			}
			sum = sum * 12 / float64(len(months)) // annualize a partial year
			fbiMu.Lock()
			fbiCache[state] = struct {
				at   time.Time
				rate float64
				year int
			}{time.Now(), sum, year}
			fbiMu.Unlock()
			return sum, year, true
		}
		year--
	}
	return 0, 0, false
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
