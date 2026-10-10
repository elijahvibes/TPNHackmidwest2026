package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"trailmix/internal/ai"
	"trailmix/internal/planner"
	"trailmix/internal/travel"
)

func TestPreviewMissingSerpAPIKey(t *testing.T) {
	d := &Deps{Planner: &planner.Planner{
		AI: ai.New("", "", ""), Geo: travel.NewGeo(""), Fuel: travel.NewFuel(""),
		SerpAPI: travel.NewSerpAPI(""), Places: travel.NewPlaces(""),
	}}
	req := httptest.NewRequest(http.MethodPost, "/api/plan/preview", strings.NewReader(
		`{"origin":"Kansas City","destination":"Chicago","start_date":"2026-11-10","end_date":"2026-11-13","travelers":2}`,
	))
	rec := httptest.NewRecorder()
	d.planPreview(rec, req)
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "SERPAPI_KEY is required") {
		t.Fatalf("expected missing key HTTP 502, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPreviewLiveProviderFailure(t *testing.T) {
	d := &Deps{Planner: &planner.Planner{
		AI: ai.New("", "", ""), Geo: travel.NewGeo(""), Fuel: travel.NewFuel(""),
		SerpAPI: travel.NewSerpAPI("test-secret"), Places: travel.NewPlaces(""),
	}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodPost, "/api/plan/preview", strings.NewReader(
		`{"origin":"Kansas City","destination":"Chicago","start_date":"2026-11-10","end_date":"2026-11-13","travelers":2}`,
	)).WithContext(ctx)
	rec := httptest.NewRecorder()
	d.planPreview(rec, req)
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "flight search") {
		t.Fatalf("expected explicit HTTP 502 provider error, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "test-secret") {
		t.Fatal("response leaked API key")
	}
}

func TestPlanErrorKeepsValidationStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	planError(rec, context.Canceled)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("validation status = %d, want 400", rec.Code)
	}
}
