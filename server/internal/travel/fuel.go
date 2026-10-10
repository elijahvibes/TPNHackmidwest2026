package travel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Fuel estimates driving cost. Vehicle MPG comes from fueleconomy.gov (no key
// required); the gas price comes from EIA's weekly series via the API when
// EIA_KEY is set, else from EIA's public RSS feed, else a national fallback.
type Fuel struct {
	eiaKey string
	http   *http.Client
}

func NewFuel(eiaKey string) *Fuel {
	return &Fuel{eiaKey: eiaKey, http: &http.Client{Timeout: 15 * time.Second}}
}

const fallbackGasPrice = 3.19

// GasPrice returns $/gallon regular, US average. With EIA_KEY it uses the EIA
// API; without one it parses EIA's public weekly RSS feed (same numbers, no
// key); only if both fail does it fall back to a constant.
func (f *Fuel) GasPrice(ctx context.Context) (float64, string) {
	if f.eiaKey == "" {
		if v, ok := f.gasPriceRSS(ctx); ok {
			return v, "eia"
		}
		return fallbackGasPrice, "fallback"
	}
	u := "https://api.eia.gov/v2/petroleum/pri/gnd/data/?frequency=weekly&data[0]=value&facets[series][]=EMM_EPMR_PTE_NUS_DPG&sort[0][column]=period&sort[0][direction]=desc&length=1&api_key=" + url.QueryEscape(f.eiaKey)
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	res, err := f.http.Do(req)
	if err != nil {
		return fallbackGasPrice, "fallback"
	}
	defer res.Body.Close()
	var out struct {
		Response struct {
			Data []struct {
				Value json.Number `json:"value"`
			} `json:"data"`
		} `json:"response"`
	}
	if json.NewDecoder(res.Body).Decode(&out) != nil || len(out.Response.Data) == 0 {
		return fallbackGasPrice, "fallback"
	}
	v, err := out.Response.Data[0].Value.Float64()
	if err != nil || v <= 0 {
		if v, ok := f.gasPriceRSS(ctx); ok {
			return v, "eia"
		}
		return fallbackGasPrice, "fallback"
	}
	return v, "eia"
}

// EIA publishes the same weekly retail prices in an RSS item whose description
// reads "4.354  .. U.S.", "4.020 .... Colorado", etc. It needs no API key.
const eiaRSS = "https://www.eia.gov/petroleum/gasdiesel/includes/gas_diesel_rss.xml"

var eiaUSLine = regexp.MustCompile(`(\d\.\d{3})\s+\.+\s*U\.S\.`)

var (
	gasRSSMu   sync.Mutex
	gasRSSAt   time.Time
	gasRSSUSD  float64
	gasRSSBody string
)

func (f *Fuel) gasPriceRSS(ctx context.Context) (float64, bool) {
	if Offline {
		return 0, false
	}
	gasRSSMu.Lock()
	defer gasRSSMu.Unlock()
	if time.Since(gasRSSAt) < 6*time.Hour && gasRSSUSD > 0 {
		return gasRSSUSD, true
	}
	req, err := http.NewRequestWithContext(ctx, "GET", eiaRSS, nil)
	if err != nil {
		return 0, false
	}
	req.Header.Set("User-Agent", "TrailMix-TPN/1.0 (HackMidwest 2026)")
	res, err := f.http.Do(req)
	if err != nil {
		return 0, false
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	m := eiaUSLine.FindSubmatch(body)
	if m == nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(string(m[1]), 64)
	if err != nil || v < 1 || v > 10 {
		return 0, false
	}
	gasRSSAt, gasRSSUSD, gasRSSBody = time.Now(), v, string(body)
	return v, true
}

// StateGasPrice returns the latest EIA weekly price for a state when the feed
// lists it (only about a dozen states are), else the US price. The feed is
// cached by gasPriceRSS, so this makes no extra request when called after it.
func (f *Fuel) StateGasPrice(ctx context.Context, state string) (float64, string, bool) {
	us, src := f.GasPrice(ctx)
	name := StateName(state)
	gasRSSMu.Lock()
	body := gasRSSBody
	gasRSSMu.Unlock()
	if name == "" || body == "" {
		return us, src, false
	}
	re := regexp.MustCompile(`(\d\.\d{3})\s+\.+\s*` + regexp.QuoteMeta(name) + `\b`)
	if m := re.FindStringSubmatch(body); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil && v > 1 {
			return v, "eia", true
		}
	}
	return us, src, false
}

// VehicleMPG looks up combined MPG for a year/make/model on fueleconomy.gov.
// Returns the first matching trim.
func (f *Fuel) VehicleMPG(ctx context.Context, year int, make, model string) (float64, error) {
	get := func(u string, v any) error {
		req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
		req.Header.Set("Accept", "application/json")
		res, err := f.http.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			return fmt.Errorf("fueleconomy.gov status %d", res.StatusCode)
		}
		return json.NewDecoder(res.Body).Decode(v)
	}
	base := "https://www.fueleconomy.gov/ws/rest/vehicle"
	// menuItem is an object for one result, an array for many, or null for none.
	type item struct {
		Text  string `json:"text"`
		Value string `json:"value"`
	}
	menu := func(u string) ([]item, error) {
		var raw struct {
			MenuItem json.RawMessage `json:"menuItem"`
		}
		if err := get(u, &raw); err != nil {
			return nil, err
		}
		var items []item
		switch {
		case len(raw.MenuItem) == 0 || string(raw.MenuItem) == "null":
		case raw.MenuItem[0] == '[':
			_ = json.Unmarshal(raw.MenuItem, &items)
		default:
			var one item
			_ = json.Unmarshal(raw.MenuItem, &one)
			items = []item{one}
		}
		return items, nil
	}

	options := func(m string) ([]item, error) {
		return menu(fmt.Sprintf("%s/menu/options?year=%d&make=%s&model=%s", base, year, url.QueryEscape(make), url.QueryEscape(m)))
	}
	items, err := options(model)
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		// Names are exact on fueleconomy.gov ("Civic 4Dr"); find the closest model for the make.
		models, err := menu(fmt.Sprintf("%s/menu/model?year=%d&make=%s", base, year, url.QueryEscape(make)))
		if err != nil {
			return 0, err
		}
		want := strings.ToLower(strings.TrimSpace(model))
		for _, m := range models {
			if strings.HasPrefix(strings.ToLower(m.Value), want) {
				if items, err = options(m.Value); err == nil && len(items) > 0 {
					break
				}
			}
		}
	}
	if len(items) == 0 || items[0].Value == "" {
		return 0, fmt.Errorf("no vehicle match for %d %s %s", year, make, model)
	}
	var veh struct {
		Comb08 string `json:"comb08"`
	}
	if err := get(base+"/"+items[0].Value, &veh); err != nil {
		return 0, err
	}
	mpg, _ := strconv.ParseFloat(veh.Comb08, 64)
	return mpg, nil
}

func (f *Fuel) Drive(ctx context.Context, miles float64, minutes int, mpg float64) DriveEstimate {
	if mpg <= 0 {
		mpg = 28 // typical sedan
	}
	price, src := f.GasPrice(ctx)
	gallons := miles / mpg
	cost := gallons * price
	return DriveEstimate{
		DistanceMiles: round1(miles),
		DurationMin:   minutes,
		GallonsNeeded: round1(gallons),
		GasPriceUSD:   round2(price),
		FuelCostUSD:   round2(cost),
		RoundTripUSD:  round2(cost * 2),
		VehicleMPG:    mpg,
		Source:        src,
	}
}

func round1(v float64) float64 { return float64(int(v*10+0.5)) / 10 }
func round2(v float64) float64 { return float64(int(v*100+0.5)) / 100 }
