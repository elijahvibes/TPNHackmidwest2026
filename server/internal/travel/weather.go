package travel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// DayWeather is one calendar day of forecast at a place.
type DayWeather struct {
	Date      string `json:"date"`
	Label     string `json:"label"` // "Friday"
	HighF     int    `json:"high_f"`
	LowF      int    `json:"low_f"`
	Short     string `json:"short"`      // "Chance Rain Showers"
	PrecipPct int    `json:"precip_pct"` // 0-100
	Icon      string `json:"icon"`       // sun | cloud | rain | snow | storm | wind | fog
}

type Weather struct {
	Place  string       `json:"place"`
	Days   []DayWeather `json:"days"`
	Note   string       `json:"note,omitempty"`
	Source string       `json:"source"` // nws | mock
}

// NWS wraps the National Weather Service API (api.weather.gov): free, no key,
// US only, 7-day horizon. A User-Agent header is required by their policy.
type NWS struct{ http *http.Client }

func NewNWS() *NWS { return &NWS{http: &http.Client{Timeout: 10 * time.Second}} }

func (n *NWS) get(ctx context.Context, url string, v any) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("User-Agent", "TPN trip planner (hackmidwest 2026; contact via github.com/elijahvibes)")
	req.Header.Set("Accept", "application/geo+json")
	res, err := n.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("nws %d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(v)
}

// Forecast returns daily weather at a point for the trip dates. Days beyond the
// 7-day NWS horizon are not invented; the Note explains the gap. Nil receiver or
// any failure falls back to deterministic mock weather so the UI never goes blank.
func (n *NWS) Forecast(ctx context.Context, at LatLng, place, startDate, endDate string) Weather {
	start, err1 := time.Parse("2006-01-02", startDate)
	end, err2 := time.Parse("2006-01-02", endDate)
	if err1 != nil || err2 != nil {
		return mockWeather(at, place, startDate, 1)
	}
	tripDays := int(end.Sub(start).Hours()/24) + 1
	if n == nil {
		return mockWeather(at, place, startDate, tripDays)
	}

	var pt struct {
		Properties struct {
			Forecast string `json:"forecast"`
		} `json:"properties"`
	}
	if err := n.get(ctx, fmt.Sprintf("https://api.weather.gov/points/%.4f,%.4f", at.Lat, at.Lng), &pt); err != nil || pt.Properties.Forecast == "" {
		w := mockWeather(at, place, startDate, tripDays)
		w.Note = "NWS unavailable right now; showing typical conditions"
		return w
	}
	var fc struct {
		Properties struct {
			Periods []struct {
				Name          string `json:"name"`
				StartTime     string `json:"startTime"`
				IsDaytime     bool   `json:"isDaytime"`
				Temperature   int    `json:"temperature"`
				ShortForecast string `json:"shortForecast"`
				Precip        struct {
					Value *int `json:"value"`
				} `json:"probabilityOfPrecipitation"`
			} `json:"periods"`
		} `json:"properties"`
	}
	if err := n.get(ctx, pt.Properties.Forecast, &fc); err != nil || len(fc.Properties.Periods) == 0 {
		w := mockWeather(at, place, startDate, tripDays)
		w.Note = "NWS forecast unavailable right now; showing typical conditions"
		return w
	}

	// Fold day/night periods into calendar days.
	byDate := map[string]*DayWeather{}
	var order []string
	for _, p := range fc.Properties.Periods {
		if len(p.StartTime) < 10 {
			continue
		}
		date := p.StartTime[:10]
		d, ok := byDate[date]
		if !ok {
			d = &DayWeather{Date: date, HighF: -999, LowF: 999}
			byDate[date] = d
			order = append(order, date)
		}
		if p.IsDaytime {
			d.Label, d.Short, d.Icon = p.Name, p.ShortForecast, iconFor(p.ShortForecast)
			if p.Temperature > d.HighF {
				d.HighF = p.Temperature
			}
		} else {
			if p.Temperature < d.LowF {
				d.LowF = p.Temperature
			}
			if d.Short == "" {
				d.Label, d.Short, d.Icon = strings.TrimSuffix(p.Name, " Night"), p.ShortForecast, iconFor(p.ShortForecast)
			}
		}
		if p.Precip.Value != nil && *p.Precip.Value > d.PrecipPct {
			d.PrecipPct = *p.Precip.Value
		}
	}
	w := Weather{Place: place, Source: "nws"}
	covered := 0
	for _, date := range order {
		d := byDate[date]
		if d.HighF == -999 {
			d.HighF = d.LowF + 12
		}
		if d.LowF == 999 {
			d.LowF = d.HighF - 12
		}
		if date >= startDate && date <= endDate {
			w.Days = append(w.Days, *d)
			covered++
		}
	}
	switch {
	case covered == 0:
		// Trip is past the horizon: show the outlook anyway so the panel is useful.
		for _, date := range order[:min(5, len(order))] {
			w.Days = append(w.Days, *byDate[date])
		}
		w.Note = fmt.Sprintf("trip starts %s, beyond the 7-day NWS forecast; showing the current outlook for %s", startDate, place)
	case covered < tripDays:
		w.Note = fmt.Sprintf("NWS covers the first %d of %d trip days", covered, tripDays)
	}
	return w
}

func iconFor(short string) string {
	s := strings.ToLower(short)
	switch {
	case strings.Contains(s, "thunder"):
		return "storm"
	case strings.Contains(s, "snow"), strings.Contains(s, "flurr"), strings.Contains(s, "sleet"):
		return "snow"
	case strings.Contains(s, "rain"), strings.Contains(s, "shower"), strings.Contains(s, "drizzle"):
		return "rain"
	case strings.Contains(s, "fog"), strings.Contains(s, "haze"):
		return "fog"
	case strings.Contains(s, "wind"), strings.Contains(s, "breezy"):
		return "wind"
	case strings.Contains(s, "cloud"), strings.Contains(s, "overcast"):
		return "cloud"
	}
	return "sun"
}

func mockWeather(at LatLng, place, startDate string, days int) Weather {
	start, _ := time.Parse("2006-01-02", startDate)
	s := seed(fmt.Sprintf("%.1f,%.1f", at.Lat, at.Lng), startDate)
	// Rough seasonal baseline from latitude + month, then jitter.
	month := int(start.Month())
	season := 1 - float64(abs(month-7))/6 // 1 in July, 0 in January
	base := 30 + season*55 - (at.Lat-30)*0.9
	kinds := []string{"sun", "cloud", "sun", "rain", "sun", "cloud", "storm"}
	shorts := map[string]string{"sun": "Sunny", "cloud": "Partly Cloudy", "rain": "Chance Rain Showers", "storm": "Scattered Thunderstorms"}
	w := Weather{Place: place, Source: "mock"}
	for i := 0; i < days && i < 7; i++ {
		d := start.AddDate(0, 0, i)
		k := kinds[(int(s)+i)%len(kinds)]
		hi := int(base + float64(int(s>>uint(i))%12) - 4)
		pp := 5
		if k == "rain" {
			pp = 55
		} else if k == "storm" {
			pp = 70
		}
		w.Days = append(w.Days, DayWeather{Date: d.Format("2006-01-02"), Label: d.Weekday().String(), HighF: hi, LowF: hi - 14, Short: shorts[k], PrecipPct: pp, Icon: k})
	}
	return w
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
