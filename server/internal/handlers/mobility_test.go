package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"trailmix/internal/planner"
)

func TestPreviewRejectsInvalidMobilityBeforeProviderCalls(t *testing.T) {
	for _, body := range []string{
		`{"local_transport":"helicopter"}`,
		`{"activity_radius_miles":0.5}`,
		`{"activity_radius_miles":151}`,
	} {
		d := &Deps{Planner: &planner.Planner{}}
		r := httptest.NewRequest(http.MethodPost, "/api/plan/preview", strings.NewReader(body))
		w := httptest.NewRecorder()
		d.planPreview(w, r)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "must be") {
			t.Fatalf("invalid mobility did not produce explicit 400: %d %s", w.Code, w.Body.String())
		}
	}
}
