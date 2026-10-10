package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"trailmix/internal/planner"
)

func TestSelectPreviewRejectsInvalidSelection(t *testing.T) {
	d := &Deps{Planner: &planner.Planner{}}
	for _, body := range []string{`{}`, `{"plan":null}`, `{"plan":{"nights":-1},"flight_index":0}`} {
		w := httptest.NewRecorder()
		d.selectPreviewFlight(w, httptest.NewRequest(http.MethodPost, "/api/plan/flight", strings.NewReader(body)))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("invalid selection must return 400, got %d: %s", w.Code, w.Body.String())
		}
	}
}

func TestBookingRedirectRejectsUnsafeTargets(t *testing.T) {
	for _, target := range []string{
		"http://www.google.com/travel/clk/f",
		"https://www.google.com.evil.example/travel/clk/f",
		"https://127.0.0.1/travel/clk/f",
		"https://www.google.com/other-path",
		"https://user:password@www.google.com/travel/clk/f",
		"https://www.google.com:444/travel/clk/f",
	} {
		w := httptest.NewRecorder()
		body := url.Values{"url": {target}, "post_data": {"u=opaque"}}
		r := httptest.NewRequest(http.MethodPost, "/api/flights/booking-redirect", strings.NewReader(body.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resolveFlightBooking(w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("unsafe target accepted: %s (%d)", target, w.Code)
		}
	}
}

func TestBookingRedirectHandlesGoogleMetaRefresh(t *testing.T) {
	res := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(
		`<html><head><meta content="0;url='https://www.united.com/booking?id=123&amp;Name=Google-Web'" http-equiv="refresh"></head><body></body></html>`,
	))}
	redirect, err := bookingRedirect(res)
	if err != nil || redirect.String() != "https://www.united.com/booking?id=123&Name=Google-Web" {
		t.Fatalf("meta redirect = %v, %v", redirect, err)
	}
}

func TestBookingRedirectRejectsNonRedirectHTML(t *testing.T) {
	res := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`<html><body>Link expired</body></html>`))}
	if _, err := bookingRedirect(res); err == nil {
		t.Fatal("non-redirect HTML must fail")
	}
}
