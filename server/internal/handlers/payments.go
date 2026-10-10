package handlers

import (
	"net/http"
	"time"

	"trailmix/internal/auth"
	"trailmix/internal/httpx"
)

type Payment struct {
	ID          string    `json:"id"`
	TripID      *string   `json:"trip_id"`
	Provider    string    `json:"provider"`
	ProviderRef *string   `json:"provider_ref"`
	AmountUSD   float64   `json:"amount_usd"`
	Status      string    `json:"status"`
	Memo        *string   `json:"memo"`
	CreatedAt   time.Time `json:"created_at"`
}

const paymentCols = "id, trip_id, provider, provider_ref, amount_usd, status, memo, created_at"

func (d *Deps) listPayments(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	rows, err := d.DB.Query(r.Context(), "SELECT "+paymentCols+" FROM payments WHERE user_id=$1 ORDER BY created_at DESC", c.UserID)
	if err != nil {
		httpx.Error(w, 500, "query failed")
		return
	}
	defer rows.Close()
	out := []Payment{}
	for rows.Next() {
		var p Payment
		if err := rows.Scan(&p.ID, &p.TripID, &p.Provider, &p.ProviderRef, &p.AmountUSD, &p.Status, &p.Memo, &p.CreatedAt); err != nil {
			httpx.Error(w, 500, "scan failed")
			return
		}
		out = append(out, p)
	}
	httpx.JSON(w, 200, out)
}

// createPayment charges the trip total through Jack Henry and marks the trip booked.
func (d *Deps) createPayment(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var in struct {
		TripID    *string `json:"trip_id"`
		AmountUSD float64 `json:"amount_usd"`
		Memo      string  `json:"memo"`
	}
	if err := httpx.Decode(r, &in); err != nil || in.AmountUSD <= 0 {
		httpx.Error(w, 400, "amount_usd must be positive")
		return
	}
	res, err := d.Payments.Pay(r.Context(), c.UserID, in.AmountUSD, in.Memo)
	if err != nil {
		httpx.Error(w, 502, err.Error())
		return
	}
	var p Payment
	err = d.DB.QueryRow(r.Context(),
		`INSERT INTO payments (user_id, trip_id, provider_ref, amount_usd, status, memo)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING `+paymentCols,
		c.UserID, in.TripID, res.ProviderRef, in.AmountUSD, res.Status, in.Memo).
		Scan(&p.ID, &p.TripID, &p.Provider, &p.ProviderRef, &p.AmountUSD, &p.Status, &p.Memo, &p.CreatedAt)
	if err != nil {
		httpx.Error(w, 500, "could not record payment")
		return
	}
	if in.TripID != nil && res.Status == "settled" {
		_, _ = d.DB.Exec(r.Context(), "UPDATE trips SET status='booked', updated_at=now() WHERE id=$1 AND user_id=$2", *in.TripID, c.UserID)
	}
	httpx.JSON(w, 201, map[string]any{"payment": p, "mock": res.Mock})
}
