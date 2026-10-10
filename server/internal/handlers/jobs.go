package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"trailmix/internal/httpx"
	"trailmix/internal/planner"
	"trailmix/internal/travel"
)

// A plan takes 60-120 s because of the AI agents. Instead of holding one HTTP
// request open for that long, planning runs as a background job: the browser
// starts it, polls, shows the rule-based draft as soon as it exists, and swaps
// in the AI version when the job finishes.

type jobEvent struct {
	At      time.Time `json:"at"`
	Stage   string    `json:"stage"`
	Message string    `json:"message"`
	Agent   string    `json:"agent,omitempty"`
	Status  string    `json:"status,omitempty"`
}

// jobState is what the browser polls.
type jobState struct {
	ID      string        `json:"id"`
	Status  string        `json:"status"` // running | done | error
	Stage   string        `json:"stage"`
	Events  []jobEvent    `json:"events"`
	Plan    *planner.Plan `json:"plan,omitempty"` // latest draft, or the final plan when done
	Final   bool          `json:"final"`
	Error   string        `json:"error,omitempty"`
	TripID  string        `json:"trip_id,omitempty"`
	Created time.Time     `json:"created"`
}

type planJob struct {
	mu sync.Mutex
	jobState
}

func (j *planJob) snapshot() jobState {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := j.jobState
	out.Events = append([]jobEvent(nil), j.Events...)
	return out
}

var (
	jobsMu sync.Mutex
	jobs   = map[string]*planJob{}
)

const jobTTL = 20 * time.Minute

func getJob(id string) *planJob {
	jobsMu.Lock()
	defer jobsMu.Unlock()
	for k, j := range jobs { // opportunistic cleanup
		if time.Since(j.Created) > jobTTL {
			delete(jobs, k)
		}
	}
	return jobs[id]
}

// startPlanJob runs the planner in the background. onDone runs with the final
// plan (for saving to the trip); it may be nil.
func (d *Deps) startPlanJob(req planner.Request, tripID string, onDone func(context.Context, *planner.Plan) error) *planJob {
	job := &planJob{jobState: jobState{ID: uuid.NewString(), Status: "running", Stage: "search", TripID: tripID, Created: time.Now()}}
	jobsMu.Lock()
	jobs[job.ID] = job
	jobsMu.Unlock()

	req.Progress = func(ev planner.Event) {
		job.mu.Lock()
		defer job.mu.Unlock()
		e := jobEvent{At: time.Now(), Stage: ev.Stage, Message: ev.Message}
		if ev.Report != nil {
			e.Agent, e.Status = ev.Report.Agent, ev.Report.Status
		}
		job.Events = append(job.Events, e)
		job.Stage = ev.Stage
		if ev.Plan != nil {
			job.Plan = ev.Plan
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
		defer cancel()
		plan, err := d.Planner.Plan(ctx, req)
		if err == nil && onDone != nil {
			err = onDone(ctx, plan)
		}
		job.mu.Lock()
		defer job.mu.Unlock()
		if err != nil {
			job.Status, job.Error = "error", err.Error()
			if errors.Is(err, travel.ErrProvider) {
				job.Error = "provider: " + err.Error()
			}
			job.Events = append(job.Events, jobEvent{At: time.Now(), Stage: "error", Message: err.Error()})
			return
		}
		job.Plan, job.Status, job.Final, job.Stage = plan, "done", true, "done"
	}()
	return job
}

// POST /api/plan/preview/start {planner.Request} → {job_id}
func (d *Deps) startPreview(w http.ResponseWriter, r *http.Request) {
	var req planner.Request
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, 400, "invalid json")
		return
	}
	job := d.startPlanJob(req, "", nil)
	httpx.JSON(w, 202, map[string]string{"job_id": job.ID})
}

// GET /api/plan/jobs/{id} → job snapshot
func (d *Deps) planJobStatus(w http.ResponseWriter, r *http.Request) {
	job := getJob(chi.URLParam(r, "id"))
	if job == nil {
		httpx.Error(w, 404, "no such planning job (it may have expired)")
		return
	}
	httpx.JSON(w, 200, job.snapshot())
}

// POST /api/trips/{id}/plan/start → {job_id}; the plan is saved on the trip
// when the job finishes.
func (d *Deps) startTripPlan(w http.ResponseWriter, r *http.Request) {
	t, req, ok := d.tripPlanRequest(w, r)
	if !ok {
		return
	}
	job := d.startPlanJob(req, t.ID, func(ctx context.Context, plan *planner.Plan) error {
		raw, _ := json.Marshal(plan)
		_, err := d.DB.ExecContext(ctx, "UPDATE trips SET plan=?, status='planned', mode=? WHERE id=?", string(raw), t.Mode, t.ID)
		return err
	})
	httpx.JSON(w, 202, map[string]string{"job_id": job.ID, "trip_id": t.ID})
}
