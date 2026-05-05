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
