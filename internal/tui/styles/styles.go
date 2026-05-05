// Package styles is the canonical translation of the srepulse design
// system's colors_and_type.css into Lipgloss styles. Every other TUI
// component imports its colours from here so palette changes ripple
// through the whole binary by editing one file.
//
// Source: srepulse-design-system/project/colors_and_type.css and the
// CLI mock at ui_kits/cli/index.html. Hex values are copied verbatim
// — when the design ships an update we re-port here, not in callers.
package styles

import "github.com/charmbracelet/lipgloss"

// ── Brand ────────────────────────────────────────────────────────
//
// `pulse` is the primary teal — used for the prompt char, the active
// phase indicator, the live-agent dot, fingerprint ids, confidence
// numbers, and approve actions. Used sparingly: if something is
// pulse, it matters.
var (
	BrandPulse    = lipgloss.Color("#14B8A6")
	BrandPulseDim = lipgloss.Color("#0EA5A4")
	BrandTide     = lipgloss.Color("#1E40AF") // gradient endpoint
)

// ── Foreground (4-step text scale) ───────────────────────────────
var (
	Fg1 = lipgloss.Color("#e8edf2") // primary text
	Fg2 = lipgloss.Color("#b0bac5") // secondary
	Fg3 = lipgloss.Color("#7a8694") // tertiary, labels, separators
	Fg4 = lipgloss.Color("#4a5563") // disabled, placeholders
)

// ── Backgrounds (5-step surface scale, dark theme only) ──────────
//
// Lipgloss happily renders these as fills; on terminals that don't
// support truecolor backgrounds the styles degrade to the closest
// 256-colour and finally to no background at all. The CLI mock keeps
// these subtle — never wash a panel in pulse.
var (
	BgCanvas  = lipgloss.Color("#0a0d10") // page
	BgBase    = lipgloss.Color("#0e1216") // default panel
	BgRaised  = lipgloss.Color("#141a20") // elevated card
	BgOverlay = lipgloss.Color("#1a2128") // modal / popover
	BgInset   = lipgloss.Color("#06080a") // code block / inset well
)

// ── Status (functional, not decorative) ──────────────────────────
//
// Severity dots, phase outcome glyphs, calibration chips. Map P1 →
// Critical, P2 → Warn, P3 → Muted in CLI tables to mirror the mock.
var (
	StatusOk       = lipgloss.Color("#10b981")
	StatusInfo     = lipgloss.Color("#3b82f6")
	StatusWarn     = lipgloss.Color("#f59e0b")
	StatusError    = lipgloss.Color("#ef4444")
	StatusCritical = lipgloss.Color("#dc2626")
	StatusPending  = lipgloss.Color("#a855f7") // awaiting-approval (human-in-loop)
	StatusMuted    = lipgloss.Color("#6b7280")
)

// ── Reusable text styles ─────────────────────────────────────────
//
// These map onto the CSS classes from the CLI mock (.p, .dim, .ok,
// .warn, .err, .pulse) so anyone reading the design + the code can
// see the correspondence at a glance.

// Prompt is the leading `~` character before each command line.
var Prompt = lipgloss.NewStyle().Foreground(BrandPulse)

// Dim is `$` markers, separators, secondary labels.
var Dim = lipgloss.NewStyle().Foreground(Fg3)

// OK / Warn / Err are the three functional inline styles.
var (
	OK   = lipgloss.NewStyle().Foreground(StatusOk)
	Warn = lipgloss.NewStyle().Foreground(StatusWarn)
	Err  = lipgloss.NewStyle().Foreground(StatusError)
)

// Pulse is the teal-emphasis style used for the active phase,
// confidence numbers, token savings, and the cursor.
var Pulse = lipgloss.NewStyle().Foreground(BrandPulse).Bold(true)

// PulseBox renders the highlighted incident-detail block — pulse
// border + pulse-tinted background. Mock equivalent: .box.pulse-box.
var PulseBox = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder()).
	BorderForeground(BrandPulse).
	Padding(0, 1).
	Foreground(Fg2)

// PanelBox is the unhighlighted variant (.box from the mock) — used
// for non-active sections, with a hairline border in border-2.
var PanelBox = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("#262f38")). // border-2 from tokens
	Padding(0, 1).
	Foreground(Fg2)

// Header is the chrome at the top of the TUI — bold pulse for the
// product name, dim mono for the cluster context.
var (
	Header     = lipgloss.NewStyle().Foreground(BrandPulse).Bold(true)
	HeaderHint = lipgloss.NewStyle().Foreground(Fg3)
)

// SeverityStyle picks a Lipgloss style for an alert severity label
// using the design's P1=critical / P2=warn / P3=muted convention.
// Unknown values fall back to muted so we never stylelessly render.
func SeverityStyle(sev string) lipgloss.Style {
	switch sev {
	case "P1", "p1", "critical", "Critical":
		return lipgloss.NewStyle().Foreground(StatusError).Bold(true)
	case "P2", "p2", "warning", "Warning", "warn":
		return lipgloss.NewStyle().Foreground(StatusWarn)
	case "P3", "p3", "info", "Info":
		return lipgloss.NewStyle().Foreground(Fg3)
	case "pending", "awaiting_approval":
		return lipgloss.NewStyle().Foreground(StatusPending)
	case "ok", "resolved", "applied":
		return OK
	default:
		return Dim
	}
}

// PhaseGlyph renders one segment of the seven-step phase strip with
// the design's iconography: ✓ for done, ◆ in pulse for current, ·
// for pending. Mock pattern lifted verbatim from the inspect block.
func PhaseGlyph(label string, state PhaseState) string {
	switch state {
	case PhaseDone:
		return Dim.Render(label) + " " + OK.Render("✓")
	case PhaseCurrent:
		return Pulse.Render(label+" ◆")
	case PhasePending:
		return Dim.Render(label) + " " + Dim.Render("·")
	default:
		return Dim.Render(label)
	}
}

// PhaseState is the trinary state for each segment of the phase strip.
type PhaseState int

const (
	PhasePending PhaseState = iota
	PhaseCurrent
	PhaseDone
)
