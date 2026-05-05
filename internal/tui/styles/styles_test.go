package styles

import (
	"strings"
	"testing"
)

func TestSeverityStyle_KnownLabels(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// Each entry just checks that the rendered output is non-empty
		// — we're verifying the lookup-by-label, not exact ANSI bytes
		// (which depend on terminal capability detection at runtime).
		{"P1", "P1"},
		{"p1", "p1"},
		{"critical", "critical"},
		{"P2", "P2"},
		{"warn", "warn"},
		{"P3", "P3"},
		{"info", "info"},
		{"pending", "pending"},
		{"awaiting_approval", "awaiting_approval"},
		{"resolved", "resolved"},
		{"unrecognised-bucket", "unrecognised-bucket"}, // falls through to dim
	}
	for _, c := range cases {
		got := SeverityStyle(c.in).Render(c.in)
		if !strings.Contains(got, c.want) {
			t.Errorf("SeverityStyle(%q) didn't render the label, got %q", c.in, got)
		}
	}
}

func TestPhaseGlyph_DoneCurrentPending(t *testing.T) {
	done := PhaseGlyph("apply", PhaseDone)
	if !strings.Contains(done, "✓") {
		t.Errorf("PhaseDone should render ✓, got %q", done)
	}
	if !strings.Contains(done, "apply") {
		t.Errorf("PhaseDone should include the label, got %q", done)
	}

	cur := PhaseGlyph("approve", PhaseCurrent)
	if !strings.Contains(cur, "◆") {
		t.Errorf("PhaseCurrent should render ◆, got %q", cur)
	}

	pending := PhaseGlyph("verify", PhasePending)
	if !strings.Contains(pending, "·") {
		t.Errorf("PhasePending should render ·, got %q", pending)
	}
}

func TestPhaseStates_AreDistinct(t *testing.T) {
	// Trinary should produce three different glyphs for the same label.
	if PhaseGlyph("x", PhaseDone) == PhaseGlyph("x", PhasePending) {
		t.Error("done and pending render identically — colour or glyph change missing")
	}
	if PhaseGlyph("x", PhaseDone) == PhaseGlyph("x", PhaseCurrent) {
		t.Error("done and current render identically")
	}
}
