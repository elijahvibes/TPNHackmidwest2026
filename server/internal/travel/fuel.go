package travel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Fuel estimates driving cost. Vehicle MPG comes from fueleconomy.gov (no key
// required); the regional gas price comes from the EIA weekly series when
// EIA_KEY is set, else a national fallback.
type Fuel struct {
	eiaKey string
	http   *http.Client
}

func NewFuel(eiaKey string) *Fuel {
	return &Fuel{eiaKey: eiaKey, http: &http.Client{Timeout: 15 * time.Second}}
}

const fallbackGasPrice = 3.19

// GasPrice returns $/gallon regular. EIA series EMM_EPMR_PTE_NUS_DPG = US regular, weekly.
func (f *Fuel) GasPrice(ctx context.Context) (float64, string) {
	if f.eiaKey == "" {
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
		return fallbackGasPrice, "fallback"
	}
	return v, "eia"
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
