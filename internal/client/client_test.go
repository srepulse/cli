package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/srepulse/cli/internal/config"
)

// fakeServer returns a httptest server with simple route mux for
// every endpoint the client touches. Tests assert on the requests
// it received via the captured fields.
type capture struct {
	method string
	path   string
	auth   string
	body   string
}

func newFakeServer(t *testing.T, status int, response string, captured *capture) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method = r.Method
		captured.path = r.URL.Path
		captured.auth = r.Header.Get("Authorization")
		if r.Body != nil {
			b, _ := readAllString(r.Body, 4096)
			captured.body = b
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
}

func readAllString(r interface{ Read(p []byte) (int, error) }, max int) (string, error) {
	buf := make([]byte, max)
	n, _ := r.Read(buf)
	return string(buf[:n]), nil
}

func TestListIncidents_Success(t *testing.T) {
	captured := &capture{}
	srv := newFakeServer(t, 200,
		`[{"id":"a","status":"resolved","triggerEvent":{"reason":"OOMKilled","namespace":"x","name":"p"}}]`,
		captured,
	)
	defer srv.Close()

	c := New(srv.URL, false)
	got, err := c.ListIncidents(context.Background())
	if err != nil {
		t.Fatalf("ListIncidents: %v", err)
	}
	if len(got) != 1 || got[0].ID != "a" || got[0].TriggerEvent.Reason != "OOMKilled" {
		t.Errorf("decode mismatch, got %+v", got)
	}
	if captured.method != http.MethodGet {
		t.Errorf("method = %s, want GET", captured.method)
	}
	if captured.path != "/api/v1/incidents" {
		t.Errorf("path = %s, want /api/v1/incidents", captured.path)
	}
}

func TestListIncidents_HTTPError_BubblesUp(t *testing.T) {
	srv := newFakeServer(t, 503, `{"error":"backend down"}`, &capture{})
	defer srv.Close()

	_, err := New(srv.URL, false).ListIncidents(context.Background())
	if err == nil {
		t.Fatal("expected error on 503")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error should mention HTTP 503, got %v", err)
	}
}

func TestApproveIncident_SendsReason(t *testing.T) {
	captured := &capture{}
	srv := newFakeServer(t, 200, "ok", captured)
	defer srv.Close()

	c := New(srv.URL, false)
	if err := c.ApproveIncident(context.Background(), "abc", "verified in staging"); err != nil {
		t.Fatal(err)
	}
	if captured.method != http.MethodPost {
		t.Errorf("method = %s, want POST", captured.method)
	}
	if captured.path != "/api/v1/incidents/abc/approve" {
		t.Errorf("path = %s", captured.path)
	}
	var body map[string]string
	if err := json.Unmarshal([]byte(captured.body), &body); err != nil {
		t.Fatalf("body not JSON: %v (raw=%s)", err, captured.body)
	}
	if body["reason"] != "verified in staging" {
		t.Errorf("reason in body = %q, want \"verified in staging\"", body["reason"])
	}
}

func TestRejectIncident_SendsReason(t *testing.T) {
	captured := &capture{}
	srv := newFakeServer(t, 200, "ok", captured)
	defer srv.Close()

	if err := New(srv.URL, false).RejectIncident(context.Background(), "abc", "off-hours, defer"); err != nil {
		t.Fatal(err)
	}
	if captured.method != http.MethodPost || captured.path != "/api/v1/incidents/abc/reject" {
		t.Errorf("got %s %s", captured.method, captured.path)
	}
}

func TestAuth_AppliesBearerWhenTokenCached(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SREPULSE_CONFIG_DIR", dir)
	if err := config.Save(&config.File{AuthToken: "tk-abc"}); err != nil {
		t.Fatal(err)
	}

	captured := &capture{}
	srv := newFakeServer(t, 200, "[]", captured)
	defer srv.Close()

	if _, err := New(srv.URL, false).ListIncidents(context.Background()); err != nil {
		t.Fatal(err)
	}
	if captured.auth != "Bearer tk-abc" {
		t.Errorf("Authorization = %q, want \"Bearer tk-abc\"", captured.auth)
	}
}

func TestAuth_NoHeaderWhenTokenAbsent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SREPULSE_CONFIG_DIR", dir)
	// No config — no token.

	captured := &capture{}
	srv := newFakeServer(t, 200, "[]", captured)
	defer srv.Close()

	if _, err := New(srv.URL, false).ListIncidents(context.Background()); err != nil {
		t.Fatal(err)
	}
	if captured.auth != "" {
		t.Errorf("expected no Authorization header, got %q", captured.auth)
	}
}

func TestNew_TrimsTrailingSlash(t *testing.T) {
	c := New("https://srepulse.example.com/", false)
	if !strings.HasSuffix(c.baseURL, "com") {
		t.Errorf("baseURL = %q, want trailing slash trimmed", c.baseURL)
	}
}

func TestStreamThoughts_ParsesSSEEvents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Two events separated by blank lines, plus one comment + one
		// non-data line that the parser should ignore.
		_, _ = w.Write([]byte("event: trace\n"))
		_, _ = w.Write([]byte(`data: {"node":"detect","kind":"node_enter","message":"firing","at":"2026-05-05T00:00:00Z"}` + "\n\n"))
		_, _ = w.Write([]byte("data: " + `{"node":"diagnose","kind":"node_enter","message":"matching","at":"2026-05-05T00:00:01Z"}` + "\n\n"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	events, errs := New(srv.URL, false).StreamThoughts(ctx, "abc")
	var got []TraceStep
	for ev := range events {
		got = append(got, ev)
	}
	if err := <-errs; err != nil {
		t.Fatalf("stream error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2 (raw=%+v)", len(got), got)
	}
	if got[0].Node != "detect" || got[1].Node != "diagnose" {
		t.Errorf("event parsing wrong, got %+v", got)
	}
}
