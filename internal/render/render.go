// Package render is the shared output helper for the one-shot CLI
// subcommands. It applies the srepulse design tokens (via the
// styles package) when stdout is a real TTY, and falls back to
// plain text when output is piped — `kubectl srepulse list | grep`
// and `kubectl srepulse list -j | jq` should both stay clean.
//
// All styling for non-interactive subcommands routes through this
// file. Anything in cmd/* that hand-rolls colour escapes is a bug.
package render

import (
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/srepulse/cli/internal/tui/styles"
	"golang.org/x/term"
)

// Enabled reports whether colour should be emitted to the given
// writer. Honours NO_COLOR (https://no-color.org) and falls back to
// term.IsTerminal on the underlying fd. Anything that's not an
// *os.File (bytes.Buffer in tests, pipes) gets plain text.
func Enabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// Style applies the given lipgloss style only when colour is
// enabled for w; otherwise it returns s untouched. Keeps callsites
// terse: render.Style(out, styles.Pulse, "inc-4291").
func Style(w io.Writer, st lipgloss.Style, s string) string {
	if !Enabled(w) {
		return s
	}
	return st.Render(s)
}

// Severity renders a severity label in its design colour (P1 red,
// P2 amber, P3 dim, etc.) when colour is enabled.
func Severity(w io.Writer, sev string) string {
	if sev == "" {
		return "-"
	}
	return Style(w, styles.SeverityStyle(sev), sev)
}

// Status renders an incident status with the same lookup as severity
// — `awaiting_approval` becomes pending-purple, `resolved` becomes
// green, etc. Plain string when colour is off.
func Status(w io.Writer, status string) string {
	return Style(w, styles.SeverityStyle(status), status)
}

// Dim is the canonical "label / separator" style — dim foreground,
// no bold. Used for column headers and field captions.
func Dim(w io.Writer, s string) string { return Style(w, styles.Dim, s) }

// Pulse is the "this matters" highlight. Reserve for incident IDs,
// fingerprint names, token-savings chips, and the active phase.
func Pulse(w io.Writer, s string) string { return Style(w, styles.Pulse, s) }

// OK / Warn / Err are the three short functional styles.
func OK(w io.Writer, s string) string   { return Style(w, styles.OK, s) }
func Warn(w io.Writer, s string) string { return Style(w, styles.Warn, s) }
func Err(w io.Writer, s string) string  { return Style(w, styles.Err, s) }
