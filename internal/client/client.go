// Package client is the thin srepulse REST + SSE client used by the
// CLI subcommands and the TUI. Types here intentionally mirror the
// agent's JSON shape — when the agent extracts pkg/incidentapi (see
// the linked tracking issue), this package switches to importing
// from there. Until then we keep enough of the shape to drive the
// list/show/approve/reject/logs surface.
package client

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/srepulse/cli/internal/config"
)

// Client is the API client. Construct with New(...). Concurrent use
// is safe — net/http.Client is the only mutable thing inside.
type Client struct {
	baseURL string
	http    *http.Client
}

// New builds a client pointed at the given base URL. Insecure flips
// TLS verification off — strictly for dev clusters with self-signed
// certs; production should use a real cert + DNS.
func New(baseURL string, insecure bool) *Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if insecure {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402 — opt-in dev escape
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Transport: tr,
			Timeout:   30 * time.Second,
		},
	}
}

// authHeader reads the cached token from config and returns the
// Authorization header value, or "" when no token is set.
func authHeader() string {
	cfg, err := config.Load()
	if err != nil || cfg.AuthToken == "" {
		return ""
	}
	return "Bearer " + cfg.AuthToken
}

func (c *Client) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode body: %w", err)
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if h := authHeader(); h != "" {
		req.Header.Set("Authorization", h)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return resp, nil
}

// ListIncidents returns the incident catalog. Order is server-side
// (newest first today). Filter / paginate client-side via list flags.
func (c *Client) ListIncidents(ctx context.Context) ([]Incident, error) {
	resp, err := c.do(ctx, http.MethodGet, "/api/v1/incidents", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out []Incident
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return out, nil
}

func (c *Client) GetIncident(ctx context.Context, id string) (*Incident, error) {
	resp, err := c.do(ctx, http.MethodGet, "/api/v1/incidents/"+id, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out Incident
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &out, nil
}

func (c *Client) ApproveIncident(ctx context.Context, id, reason string) error {
	body := map[string]string{"reason": reason}
	resp, err := c.do(ctx, http.MethodPost, "/api/v1/incidents/"+id+"/approve", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) RejectIncident(ctx context.Context, id, reason string) error {
	body := map[string]string{"reason": reason}
	resp, err := c.do(ctx, http.MethodPost, "/api/v1/incidents/"+id+"/reject", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// StreamThoughts opens an SSE connection to the agent's thought-
// stream for one incident and returns channels delivering parsed
// events + a terminal error. The receiver should drain `events`
// until it closes (ctx cancellation or transport EOF).
//
// Implementation is intentionally tiny — no retry, no reconnect —
// the operator typically runs this for a single incident's lifetime
// and Ctrl+C exits. v0.2 will add reconnect with backoff.
func (c *Client) StreamThoughts(ctx context.Context, id string) (<-chan TraceStep, <-chan error) {
	events := make(chan TraceStep, 32)
	errs := make(chan error, 1)

	go func() {
		defer close(events)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/sse/incidents/"+id+"/thoughts", nil)
		if err != nil {
			errs <- err
			return
		}
		req.Header.Set("Accept", "text/event-stream")
		if h := authHeader(); h != "" {
			req.Header.Set("Authorization", h)
		}
		// SSE wants no transport timeout — the http.Client.Timeout we
		// set in New() would kill the stream after 30 s. Use a per-
		// request client with no deadline.
		streamClient := &http.Client{Transport: c.http.Transport}
		resp, err := streamClient.Do(req)
		if err != nil {
			errs <- err
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			errs <- fmt.Errorf("stream HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		// SSE lines can be up to several KB for thought-stream events
		// that include tool result JSON; bump the default 64 KB buffer.
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		var dataBuf strings.Builder
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case line == "":
				if dataBuf.Len() > 0 {
					var ev TraceStep
					if err := json.Unmarshal([]byte(dataBuf.String()), &ev); err == nil {
						select {
						case events <- ev:
						case <-ctx.Done():
							return
						}
					}
					dataBuf.Reset()
				}
			case strings.HasPrefix(line, "data:"):
				dataBuf.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			default:
				// Ignore comments / event-name lines for now.
			}
		}
		if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) {
			errs <- fmt.Errorf("read SSE: %w", err)
			return
		}
		errs <- nil
	}()

	return events, errs
}
