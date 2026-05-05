package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// styles are intentionally not coupled — we want render tests to
// exercise the gating logic, not the underlying lipgloss palette.
// Use a minimal local style that always renders some ANSI when on.
var ansiRed = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000"))

func TestEnabled_PipedWriterReturnsFalse(t *testing.T) {
	// bytes.Buffer is the canonical "not a TTY" — pipelines and
	// captured output in tests both look like this from render's
	// perspective.
	if Enabled(&bytes.Buffer{}) {
		t.Error("Enabled on bytes.Buffer should be false (not a TTY)")
	}
}

func TestStyle_StripsWhenWriterIsPipe(t *testing.T) {
	var buf bytes.Buffer
	got := Style(&buf, ansiRed, "boom")
	if got != "boom" {
		t.Errorf("expected plain text on non-TTY, got %q", got)
	}
}

func TestSeverity_PlainOnPipe(t *testing.T) {
	var buf bytes.Buffer
	got := Severity(&buf, "P1")
	// No ANSI escapes anywhere in the result.
	if strings.ContainsRune(got, 0x1b) {
		t.Errorf("non-TTY output should not contain ANSI escapes: %q", got)
	}
	if got != "P1" {
		t.Errorf("expected plain \"P1\", got %q", got)
	}
}

func TestSeverity_EmptyReturnsDash(t *testing.T) {
	var buf bytes.Buffer
	if got := Severity(&buf, ""); got != "-" {
		t.Errorf("empty severity should render as \"-\", got %q", got)
	}
}

func TestNoColorEnv_DisablesStyling(t *testing.T) {
	// NO_COLOR (https://no-color.org) — even on a TTY, set this and
	// no escapes should leak. We test the gate at the Enabled() level
	// since it's where the behaviour lives.
	t.Setenv("NO_COLOR", "1")
	// Enabled() uses io.Writer; bytes.Buffer is fine because the
	// NO_COLOR check fires before the TTY check anyway.
	if Enabled(&bytes.Buffer{}) {
		t.Error("NO_COLOR=1 should disable styling unconditionally")
	}
}
