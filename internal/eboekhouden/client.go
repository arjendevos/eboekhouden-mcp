// Package eboekhouden is a small client for the e-Boekhouden REST API
// (https://api.e-boekhouden.nl). It exchanges the long-lived API token for a
// short-lived session token and renews it transparently.
package eboekhouden

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const DefaultBaseURL = "https://api.e-boekhouden.nl"

// Client calls the e-Boekhouden API. It is safe for concurrent use.
type Client struct {
	BaseURL    string
	APIToken   string
	Source     string
	HTTPClient *http.Client

	mu        sync.Mutex
	session   string
	expiresAt time.Time
}

// APIError is returned for non-2xx responses.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("e-Boekhouden API returned %d: %s", e.Status, e.Body)
}

// Do performs a request against path (e.g. "/v1/mutation") and returns the raw
// response body. body may be nil.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body any) ([]byte, error) {
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
	}

	for attempt := 0; ; attempt++ {
		token, err := c.sessionToken(ctx, attempt > 0)
		if err != nil {
			return nil, err
		}
		status, resp, err := c.send(ctx, method, path, query, payload, token)
		if err != nil {
			return nil, err
		}
		// A session can be revoked or expire early; get a fresh one once.
		if status == http.StatusUnauthorized && attempt == 0 {
			continue
		}
		if status < 200 || status > 299 {
			return nil, &APIError{Status: status, Body: strings.TrimSpace(string(resp))}
		}
		return resp, nil
	}
}

// Close ends the current session, if any.
func (c *Client) Close(ctx context.Context) error {
	c.mu.Lock()
	token := c.session
	c.session = ""
	c.mu.Unlock()
	if token == "" {
		return nil
	}
	_, _, err := c.send(ctx, http.MethodDelete, "/v1/session", nil, nil, token)
	return err
}

func (c *Client) sessionToken(ctx context.Context, forceRenew bool) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !forceRenew && c.session != "" && time.Now().Before(c.expiresAt) {
		return c.session, nil
	}

	payload, _ := json.Marshal(map[string]string{"accessToken": c.APIToken, "source": c.Source})
	status, resp, err := c.send(ctx, http.MethodPost, "/v1/session", nil, payload, "")
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("start session: %w", &APIError{Status: status, Body: strings.TrimSpace(string(resp))})
	}
	var s struct {
		Token     string `json:"token"`
		ExpiresIn int    `json:"expiresIn"`
	}
	if err := json.Unmarshal(resp, &s); err != nil || s.Token == "" {
		return "", fmt.Errorf("start session: unexpected response %q", resp)
	}
	c.session = s.Token
	// Renew a little early so a request never races the expiry.
	c.expiresAt = time.Now().Add(time.Duration(s.ExpiresIn)*time.Second - 30*time.Second)
	return c.session, nil
}

func (c *Client) send(ctx context.Context, method, path string, query url.Values, payload []byte, token string) (int, []byte, error) {
	u := strings.TrimRight(c.baseURL(), "/") + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, data, nil
}

func (c *Client) baseURL() string {
	if c.BaseURL == "" {
		return DefaultBaseURL
	}
	return c.BaseURL
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient == nil {
		return &http.Client{Timeout: 60 * time.Second}
	}
	return c.HTTPClient
}
