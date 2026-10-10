// Package payments wraps the Jack Henry payments integration.
//
// The exact endpoint and payload depend on which Jack Henry sandbox the
// hackathon provides (e.g. the Banno Digital Toolkit or a Symitar/JXchange
// test environment). The HTTP shape below is a reasonable generic REST call;
// adjust Path and the request body once the sandbox docs are in hand. With no
// JACKHENRY_BASE_URL configured, payments settle instantly in mock mode so the
// checkout flow is always demoable.
package payments

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewJackHenry(baseURL, apiKey string) *Client {
	return &Client{baseURL: baseURL, apiKey: apiKey, http: &http.Client{Timeout: 20 * time.Second}}
}

func (c *Client) Live() bool { return c.baseURL != "" && c.apiKey != "" }

type Result struct {
	ProviderRef string `json:"provider_ref"`
	Status      string `json:"status"` // settled | pending | failed
	Mock        bool   `json:"mock"`
}

// Pay moves amountUSD from the user's linked account for the given memo.
func (c *Client) Pay(ctx context.Context, userID string, amountUSD float64, memo string) (Result, error) {
	if !c.Live() {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		return Result{ProviderRef: "JH-MOCK-" + hex.EncodeToString(b), Status: "settled", Mock: true}, nil
	}
	body, _ := json.Marshal(map[string]any{
		"externalUserId": userID,
		"amount":         fmt.Sprintf("%.2f", amountUSD),
		"currency":       "USD",
		"memo":           memo,
	})
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/payments", bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	res, err := c.http.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("jack henry: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return Result{Status: "failed"}, fmt.Errorf("jack henry: status %d", res.StatusCode)
	}
	var out struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	_ = json.NewDecoder(res.Body).Decode(&out)
	if out.Status == "" {
		out.Status = "pending"
	}
	return Result{ProviderRef: out.ID, Status: out.Status}, nil
}
