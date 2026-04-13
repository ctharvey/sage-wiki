package scheduler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client polls the yawn.scheduler /acquire endpoint to ensure the
// llama-server is running before LLM requests.
type Client struct {
	baseURL string
	client  http.Client
}

// AcquireResponse is the JSON body returned by POST /acquire.
type AcquireResponse struct {
	URL   string `json:"url"`
	State string `json:"state"`
}

// NewClient creates a scheduler client. baseURL is the scheduler's
// HTTP address, e.g. "http://localhost:8082".
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		client:  http.Client{Timeout: 30 * time.Second},
	}
}

// Acquire calls POST /acquire on the scheduler and returns the llama
// endpoint URL (e.g. "http://localhost:8082/v1/t/abc"). The scheduler
// starts or wakes the llama-server as needed.
func (c *Client) Acquire() (string, error) {
	resp, err := c.client.Post(c.baseURL+"/acquire", "application/json", nil)
	if err != nil {
		return "", fmt.Errorf("scheduler: POST acquire: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("scheduler: read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("scheduler: HTTP %d: %s", resp.StatusCode, body)
	}

	var r AcquireResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("scheduler: decode: %w", err)
	}
	if r.State != "running" {
		return "", fmt.Errorf("scheduler: state=%q, expected running", r.State)
	}
	return r.URL, nil
}
