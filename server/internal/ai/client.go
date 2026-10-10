// Package ai talks to any OpenAI-compatible chat completions endpoint.
// Point AI_BASE_URL at the Akamai workshop inference service
// (https://ws.akamai-workshops.com/v1, model hackathon-chat) or Ollama on a
// Linode GPU box and it just works; with no AI_BASE_URL the planner uses its
// deterministic fallback so the rest of the app stays demoable.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

func New(baseURL, apiKey, model string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		http:    &http.Client{Timeout: 75 * time.Second},
	}
}

func (c *Client) Enabled() bool { return c.baseURL != "" }

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	Temperature    float64         `json:"temperature"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

// maxTokens is the Akamai workshop endpoint's ceiling. Its default is 512,
// which cuts a multi-day itinerary off mid-JSON, so every call asks for the max.
const maxTokens = 4096

type chatResponse struct {
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Chat sends a system + user message and returns the assistant text.
func (c *Client) Chat(ctx context.Context, system, user string, wantJSON bool) (string, error) {
	return c.chat(ctx, []Message{{Role: "system", Content: system}, {Role: "user", Content: user}}, wantJSON)
}

// ChatJSON asks for a JSON object and decodes it into out. If the first reply
// is not valid JSON it sends the broken text back once and asks the model to
// repair it, which fixes most small-model slips (trailing commas, prose
// around the object, unbalanced braces).
func (c *Client) ChatJSON(ctx context.Context, system, user string, out any) error {
	msgs := []Message{{Role: "system", Content: system}, {Role: "user", Content: user}}
	raw, err := c.chat(ctx, msgs, true)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(extractObject(raw)), out); err == nil {
		return nil
	}
	msgs = append(msgs,
		Message{Role: "assistant", Content: raw},
		Message{Role: "user", Content: "That was not valid JSON. Reply with ONLY the corrected JSON object, no prose, no code fences."},
	)
	raw2, err := c.chat(ctx, msgs, true)
	if err != nil {
		return fmt.Errorf("ai repair: %w", err)
	}
	if err := json.Unmarshal([]byte(extractObject(raw2)), out); err != nil {
		return fmt.Errorf("ai returned invalid JSON twice: %w (raw: %.160s)", err, raw2)
	}
	return nil
}

func (c *Client) chat(ctx context.Context, msgs []Message, wantJSON bool) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("ai: AI_BASE_URL not configured")
	}
	req := chatRequest{Model: c.model, Temperature: 0.4, MaxTokens: maxTokens, Messages: msgs}
	if wantJSON {
		req.ResponseFormat = &responseFormat{Type: "json_object"}
	}
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	res, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("ai request: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("ai: status %d: %s", res.StatusCode, truncate(string(raw), 300))
	}
	var out chatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("ai decode: %w", err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("ai: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("ai: empty response")
	}
	if out.Choices[0].FinishReason == "length" {
		return "", fmt.Errorf("ai: reply truncated at %d tokens", maxTokens)
	}
	return stripFence(out.Choices[0].Message.Content), nil
}

// extractObject returns the substring from the first '{' to the last '}',
// which drops any prose a model wraps around its JSON.
func extractObject(s string) string {
	s = stripFence(s)
	i, j := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if i >= 0 && j > i {
		return s[i : j+1]
	}
	return s
}

// stripFence removes ```json fences that smaller models like to add.
func stripFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
