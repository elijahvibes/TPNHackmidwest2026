package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"trailmix/internal/auth"
	"trailmix/internal/httpx"
	"trailmix/internal/planner"
	"trailmix/internal/travel"
)

type Trip struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Origin      string          `json:"origin"`
	Destination string          `json:"destination"`
	StartDate   string          `json:"start_date"`
	EndDate     string          `json:"end_date"`
	Travelers   int             `json:"travelers"`
	BudgetUSD   *float64        `json:"budget_usd"`
	Mode        string          `json:"mode"`
	Status      string          `json:"status"`
	Plan        json.RawMessage `json:"plan,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

const tripCols = "id, title, origin, destination, DATE_FORMAT(start_date,'%Y-%m-%d'), DATE_FORMAT(end_date,'%Y-%m-%d'), travelers, budget_usd, mode, status, plan, created_at"

func scanTrip(row scanner) (Trip, error) {
	var t Trip
	var plan []byte
	err := row.Scan(&t.ID, &t.Title, &t.Origin, &t.Destination, &t.StartDate, &t.EndDate, &t.Travelers, &t.BudgetUSD, &t.Mode, &t.Status, &plan, &t.CreatedAt)
	if len(plan) > 0 {
		t.Plan = plan
	}
	return t, err
}

func (d *Deps) tripByID(r *http.Request, id, userID string) (Trip, error) {
	return scanTrip(d.DB.QueryRowContext(r.Context(), "SELECT "+tripCols+" FROM trips WHERE id=? AND user_id=?", id, userID))
}

func (d *Deps) listTrips(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	rows, err := d.DB.QueryContext(r.Context(), "SELECT "+tripCols+" FROM trips WHERE user_id=? ORDER BY created_at DESC", c.UserID)
	if err != nil {
		httpx.Error(w, 500, "query failed")
		return
	}
	defer rows.Close()
	trips := []Trip{}
	for rows.Next() {
		t, err := scanTrip(rows)
		if err != nil {
			httpx.Error(w, 500, "scan failed")
			return
		}
		trips = append(trips, t)
	}
	httpx.JSON(w, 200, trips)
}

type tripInput struct {
	Title       string   `json:"title"`
	Origin      string   `json:"origin"`
	Destination string   `json:"destination"`
	StartDate   string   `json:"start_date"`
	EndDate     string   `json:"end_date"`
	Travelers   int      `json:"travelers"`
	BudgetUSD   *float64 `json:"budget_usd"`
	Mode        string   `json:"mode"`
}

func (d *Deps) createTrip(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var in tripInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, 400, "invalid json")
		return
	}
	if in.Origin == "" || in.Destination == "" || in.StartDate == "" || in.EndDate == "" {
		httpx.Error(w, 400, "origin, destination, start_date and end_date are required")
		return
	}
	if in.Travelers < 1 {
		in.Travelers = 1
	}
	if in.Mode == "" {
		in.Mode = "auto"
	}
	if in.Title == "" {
		in.Title = in.Origin + " to " + in.Destination
	}
	id := uuid.NewString()
	_, err := d.DB.ExecContext(r.Context(),
		`INSERT INTO trips (id, user_id, title, origin, destination, start_date, end_date, travelers, budget_usd, mode)
		 VALUES (?,?,?,?,?,?,?,?,?,?)`,
		id, c.UserID, in.Title, in.Origin, in.Destination, in.StartDate, in.EndDate, in.Travelers, in.BudgetUSD, in.Mode)
	if err != nil {
		httpx.Error(w, 400, "could not create trip: "+err.Error())
		return
	}
	t, err := d.tripByID(r, id, c.UserID)
	if err != nil {
		httpx.Error(w, 500, "could not load trip")
		return
	}
	httpx.JSON(w, 201, t)
}

func (d *Deps) getTrip(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	t, err := d.tripByID(r, chi.URLParam(r, "id"), c.UserID)
	if err != nil {
		httpx.Error(w, 404, "trip not found")
		return
	}
	httpx.JSON(w, 200, t)
}

func (d *Deps) deleteTrip(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	res, err := d.DB.ExecContext(r.Context(), "DELETE FROM trips WHERE id=? AND user_id=?", chi.URLParam(r, "id"), c.UserID)
	if err != nil {
		httpx.Error(w, 500, "delete failed")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		httpx.Error(w, 404, "trip not found")
		return
	}
	w.WriteHeader(204)
}

// planTrip runs the planner for a saved trip and stores the result.
func (d *Deps) planTrip(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	t, err := d.tripByID(r, chi.URLParam(r, "id"), c.UserID)
	if err != nil {
		httpx.Error(w, 404, "trip not found")
		return
	}
	var extra struct {
		VehicleMPG float64        `json:"vehicle_mpg"`
		Interests  []string       `json:"interests"`
		Mode       string         `json:"mode"`       // re-plan as drive/fly/auto; persisted on the trip
		OriginLoc  *travel.LatLng `json:"origin_loc"` // pinned points from the form's autocomplete
		DestLoc    *travel.LatLng `json:"dest_loc"`
	}
	_ = httpx.Decode(r, &extra) // body optional
	if extra.Mode == "drive" || extra.Mode == "fly" || extra.Mode == "auto" {
		t.Mode = extra.Mode
	}
	req := planner.Request{
		Origin: t.Origin, Destination: t.Destination, StartDate: t.StartDate, EndDate: t.EndDate,
		Travelers: t.Travelers, Mode: t.Mode, VehicleMPG: extra.VehicleMPG, Interests: extra.Interests,
		OriginLoc: extra.OriginLoc, DestLoc: extra.DestLoc,
	}
	if t.BudgetUSD != nil {
		req.BudgetUSD = *t.BudgetUSD
	}
	// A re-plan keeps the points the first plan resolved, so a town picked from
	// autocomplete never has to be geocoded again from its label.
	if t.Plan != nil && (req.OriginLoc == nil || req.DestLoc == nil) {
		var prev struct {
			OriginLoc travel.LatLng `json:"origin_loc"`
			DestLoc   travel.LatLng `json:"dest_loc"`
		}
		if json.Unmarshal(t.Plan, &prev) == nil {
			if req.OriginLoc == nil && prev.OriginLoc.Lat != 0 {
				req.OriginLoc = &prev.OriginLoc
			}
			if req.DestLoc == nil && prev.DestLoc.Lat != 0 {
				req.DestLoc = &prev.DestLoc
			}
		}
		if req.VehicleMPG == 0 {
			var drive struct {
				Drive struct {
					VehicleMPG float64 `json:"vehicle_mpg"`
				} `json:"drive"`
			}
			if json.Unmarshal(t.Plan, &drive) == nil {
				req.VehicleMPG = drive.Drive.VehicleMPG
			}
		}
	}
	plan, err := d.Planner.Plan(r.Context(), req)
	if err != nil {
		planError(w, err)
		return
	}
	raw, _ := json.Marshal(plan)
	if _, err := d.DB.ExecContext(r.Context(), "UPDATE trips SET plan=?, status='planned', mode=? WHERE id=?", string(raw), t.Mode, t.ID); err != nil {
		httpx.Error(w, 500, "save failed")
		return
	}
	httpx.JSON(w, 200, plan)
}

// chooseHotel swaps the plan's stay for another quote it already holds and
// re-totals everything. POST /api/trips/{id}/hotel {"name": "..."}.
func (d *Deps) chooseHotel(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	t, err := d.tripByID(r, chi.URLParam(r, "id"), c.UserID)
	if err != nil {
		httpx.Error(w, 404, "trip not found")
		return
	}
	if t.Plan == nil {
		httpx.Error(w, 400, "plan the trip first")
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if err := httpx.Decode(r, &in); err != nil || in.Name == "" {
		httpx.Error(w, 400, "name is required")
		return
	}
	var plan planner.Plan
	if err := json.Unmarshal(t.Plan, &plan); err != nil {
		httpx.Error(w, 500, "stored plan is unreadable")
		return
	}
	budget := 0.0
	if t.BudgetUSD != nil {
		budget = *t.BudgetUSD
	}
	if err := plan.ChooseHotel(in.Name, budget); err != nil {
		httpx.Error(w, 404, err.Error())
		return
	}
	raw, _ := json.Marshal(plan)
	if _, err := d.DB.ExecContext(r.Context(), "UPDATE trips SET plan=? WHERE id=?", string(raw), t.ID); err != nil {
		httpx.Error(w, 500, "save failed")
		return
	}
	httpx.JSON(w, 200, plan)
}

// geoSuggest: GET /api/geo/suggest?q=law → cities/towns/parks matching the prefix.
func (d *Deps) geoSuggest(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if len(q) < 2 {
		httpx.JSON(w, 200, []travel.Placemark{})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	out := d.Planner.Geo.Suggest(ctx, q, 6)
	if out == nil {
		out = []travel.Placemark{}
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	httpx.JSON(w, 200, out)
}

// shareTrip returns a planned trip to anyone holding its id (a UUID), so a
// plan can be sent to friends or judges without a login.
func (d *Deps) shareTrip(w http.ResponseWriter, r *http.Request) {
	t, err := scanTrip(d.DB.QueryRowContext(r.Context(), "SELECT "+tripCols+" FROM trips WHERE id=?", chi.URLParam(r, "id")))
	if err != nil || t.Plan == nil {
		httpx.Error(w, 404, "no shared plan at this link")
		return
	}
	httpx.JSON(w, 200, t)
}

// planPreview is the stateless version used by the landing page.
func (d *Deps) planPreview(w http.ResponseWriter, r *http.Request) {
	var req planner.Request
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, 400, "invalid json")
		return
	}
	plan, err := d.Planner.Plan(r.Context(), req)
	if err != nil {
		planError(w, err)
		return
	}
	httpx.JSON(w, 200, plan)
}

func planError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, travel.ErrProvider) {
		status = http.StatusBadGateway
	}
	httpx.Error(w, status, err.Error())
}

// vehicleMPG: GET /api/vehicles/mpg?year=2019&make=Honda&model=Civic
func (d *Deps) vehicleMPG(w http.ResponseWriter, r *http.Request) {
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	make, model := r.URL.Query().Get("make"), r.URL.Query().Get("model")
	if year == 0 || make == "" || model == "" {
		httpx.Error(w, 400, "year, make and model are required")
		return
	}
	mpg, err := d.Fuel.VehicleMPG(r.Context(), year, make, model)
	if err != nil {
		httpx.JSON(w, 200, map[string]any{"mpg": 28, "source": "fallback", "note": err.Error()})
		return
	}
	httpx.JSON(w, 200, map[string]any{"mpg": mpg, "source": "fueleconomy.gov"})
}
