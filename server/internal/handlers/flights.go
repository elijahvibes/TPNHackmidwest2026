package handlers

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"trailmix/internal/auth"
	"trailmix/internal/httpx"
	"trailmix/internal/planner"
	"trailmix/internal/travel"
)

func (d *Deps) returnFlights(w http.ResponseWriter, r *http.Request) {
	var in travel.FlightSearch
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, 400, "invalid flight search")
		return
	}
	flights, err := d.Planner.SerpAPI.ReturnFlights(r.Context(), in)
	if err != nil {
		planError(w, err)
		return
	}
	travel.RankFlights(flights)
	httpx.JSON(w, 200, flights)
}

func (d *Deps) flightBookings(w http.ResponseWriter, r *http.Request) {
	var in travel.FlightSearch
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, 400, "invalid flight search")
		return
	}
	options, err := d.Planner.SerpAPI.BookingOptions(r.Context(), in)
	if err != nil {
		planError(w, err)
		return
	}
	httpx.JSON(w, 200, options)
}

type flightSelection struct {
	Plan        *planner.Plan `json:"plan"`
	FlightIndex int           `json:"flight_index"`
	ReturnToken string        `json:"return_token,omitempty"`
}

func (d *Deps) selectPreviewFlight(w http.ResponseWriter, r *http.Request) {
	var in flightSelection
	if err := httpx.Decode(r, &in); err != nil || in.Plan == nil {
		httpx.Error(w, 400, "plan and flight selection are required")
		return
	}
	if err := d.Planner.SelectFlight(r.Context(), in.Plan, in.FlightIndex, in.ReturnToken); err != nil {
		planError(w, err)
		return
	}
	httpx.JSON(w, 200, in.Plan)
}

func (d *Deps) selectTripFlight(w http.ResponseWriter, r *http.Request) {
	var in flightSelection
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, 400, "invalid flight selection")
		return
	}
	user := auth.FromContext(r.Context())
	tx, err := d.DB.Begin(r.Context())
	if err != nil {
		httpx.Error(w, 500, "could not start flight selection")
		return
	}
	defer tx.Rollback(r.Context())
	t, err := scanTrip(tx.QueryRow(r.Context(),
		"SELECT "+tripCols+" FROM trips WHERE id=$1 AND user_id=$2 FOR UPDATE",
		chi.URLParam(r, "id"), user.UserID))
	if err != nil {
		httpx.Error(w, 404, "trip not found")
		return
	}
	if t.Status == "booked" {
		httpx.Error(w, 409, "booked trips cannot change flights")
		return
	}
	var plan planner.Plan
	if len(t.Plan) == 0 || json.Unmarshal(t.Plan, &plan) != nil {
		httpx.Error(w, 400, "re-plan this trip before selecting a flight")
		return
	}
	if err := d.Planner.SelectFlight(r.Context(), &plan, in.FlightIndex, in.ReturnToken); err != nil {
		planError(w, err)
		return
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		httpx.Error(w, 500, "could not encode flight selection")
		return
	}
	if _, err := tx.Exec(r.Context(), "UPDATE trips SET plan=$1, updated_at=now() WHERE id=$2 AND user_id=$3", raw, t.ID, user.UserID); err != nil {
		httpx.Error(w, 500, "could not save flight selection")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Error(w, 500, "could not commit flight selection")
		return
	}
	httpx.JSON(w, 200, plan)
}

// Google booking actions may require an opaque POST body, not a normal hyperlink.
func resolveFlightBooking(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		httpx.Error(w, 400, "invalid booking action")
		return
	}
	target := r.PostForm.Get("url")
	payload := r.PostForm.Get("post_data")
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "https" || u.Host != "www.google.com" || u.Path != "/travel/clk/f" || u.User != nil || payload == "" {
		httpx.Error(w, 400, "invalid Google booking action")
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, strings.NewReader(payload))
	if err != nil {
		httpx.Error(w, 400, "invalid booking request")
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		httpx.Error(w, 502, "booking link could not be resolved; refresh booking options")
		return
	}
	defer res.Body.Close()
	redirect, err := bookingRedirect(res)
	if err != nil || redirect.Scheme != "https" || redirect.Host == "" || redirect.User != nil {
		httpx.Error(w, 502, fmt.Sprintf("booking provider did not return a usable link (HTTP %d); refresh booking options", res.StatusCode))
		return
	}
	http.Redirect(w, r, redirect.String(), http.StatusSeeOther)
}

func bookingRedirect(res *http.Response) (*url.URL, error) {
	if res.StatusCode >= 300 && res.StatusCode < 400 {
		return res.Location()
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("booking action failed")
	}
	// Google can return an HTML meta refresh instead of an HTTP redirect.
	decoder := xml.NewDecoder(io.LimitReader(res.Body, 64<<10))
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Entity = xml.HTMLEntity
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("no booking redirect in response")
		}
		tag, ok := token.(xml.StartElement)
		if !ok || !strings.EqualFold(tag.Name.Local, "meta") {
			continue
		}
		var kind, content string
		for _, attr := range tag.Attr {
			if strings.EqualFold(attr.Name.Local, "http-equiv") {
				kind = attr.Value
			}
			if strings.EqualFold(attr.Name.Local, "content") {
				content = attr.Value
			}
		}
		if !strings.EqualFold(kind, "refresh") {
			continue
		}
		_, rest, ok := strings.Cut(content, ";")
		if !ok {
			continue
		}
		name, target, ok := strings.Cut(strings.TrimSpace(rest), "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(name), "url") {
			continue
		}
		return url.Parse(strings.Trim(strings.TrimSpace(target), "'\""))
	}
}
