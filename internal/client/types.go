package client

import "time"

// Mirror of the agent's JSON shape — intentionally kept thin. When
// the agent extracts pkg/incidentapi, this file becomes a re-export
// or gets deleted entirely. Field-set is whatever the CLI surface
// actually reads; expand as commands grow.

type Incident struct {
	ID            string       `json:"id"`
	CreatedAt     string       `json:"createdAt"`
	UpdatedAt     string       `json:"updatedAt"`
	Status        string       `json:"status"`
	TriggerEvent  EventInfo    `json:"triggerEvent"`
	Summary       string       `json:"summary,omitempty"`
	Trace         []TraceStep  `json:"trace"`
	Hypotheses    []Hypothesis `json:"hypotheses,omitempty"`
	Remediation   *Remediation `json:"remediation,omitempty"`
	Risk          *Risk        `json:"risk,omitempty"`
	TokensUsed    int          `json:"tokensUsed,omitempty"`
	EstimatedCost float64      `json:"estimatedCost,omitempty"`
	RunbookName   string       `json:"runbookName,omitempty"`
	Recurrences   int          `json:"recurrences,omitempty"`
}

type EventInfo struct {
	Reason      string `json:"reason"`
	Message     string `json:"message"`
	Type        string `json:"type"`
	Namespace   string `json:"namespace"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Source      string `json:"source,omitempty"`
	Severity    string `json:"severity,omitempty"`
	ExternalURL string `json:"externalUrl,omitempty"`
}

type Hypothesis struct {
	Title      string  `json:"title"`
	Reasoning  string  `json:"reasoning"`
	Confidence float64 `json:"confidence"`
}

type Remediation struct {
	Kind        string `json:"kind"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	PatchType   string `json:"patchType"`
	Patch       string `json:"patch"`
	Description string `json:"description"`
}

type Risk struct {
	Level   string   `json:"level"`
	Score   int      `json:"score"`
	Reasons []string `json:"reasons"`
	Blocked bool     `json:"blocked"`
}

type TraceStep struct {
	At      time.Time      `json:"at"`
	Node    string         `json:"node"`
	Kind    string         `json:"kind"`
	Message string         `json:"message,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

type AuthUser struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name,omitempty"`
	Role     string `json:"role"`
	Disabled bool   `json:"disabled"`
}

type LoginResponse struct {
	Token     string   `json:"token"`
	User      AuthUser `json:"user"`
	ExpiresIn int64    `json:"expiresIn"`
}

// ── Fingerprints ─────────────────────────────────────────────────
//
// The catalog of recognisers the agent matches incidents against.
// Built-ins are code-shipped and read-only; learned ones are drafts
// captured from approved LLM remediations and reviewable in the
// catalog UI.

type Fingerprint struct {
	ID          string   `json:"id"`       // stable kebab-case slug
	Name        string   `json:"name"`     // engine-side identifier (PascalCase)
	Category    string   `json:"category"` // crash / image / scheduling / observability / learned / …
	Source      string   `json:"source"`   // k8s / prometheus / alertmanager / scanner / argocd / pagerduty
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Fix         string   `json:"fix"`
	AutoPatch   bool     `json:"autoPatch"`
	Confidence  string   `json:"confidence"` // calibration string, e.g. "0.92 (n=47)" or "0.55-0.75"
	RiskLevel   string   `json:"riskLevel"`  // low / medium / high
	Triggers    []string `json:"triggers,omitempty"`
	Definition  string   `json:"definition"` // YAML body
	Status      string   `json:"status,omitempty"`
	Builtin     bool     `json:"builtin,omitempty"`
	CreatedAt   string   `json:"createdAt,omitempty"`
	UpdatedAt   string   `json:"updatedAt,omitempty"`
}

// FingerprintCatalog mirrors the top-level shape of GET /fingerprints:
// total + per-category counts + the items array + a map of pending
// refinement counts (fingerprint id → count of unreviewed plan
// suggestions).
type FingerprintCatalog struct {
	Total              int            `json:"total"`
	Categories         map[string]int `json:"categories"`
	Items              []Fingerprint  `json:"items"`
	PendingRefinements map[string]int `json:"pendingRefinements,omitempty"`
}
