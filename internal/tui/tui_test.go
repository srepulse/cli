package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/srepulse/cli/internal/client"
)

// fakeIncidents returns a stable 3-row corpus for cursor tests.
func fakeIncidents() []client.Incident {
	return []client.Incident{
		{ID: "a-id", Status: "awaiting_approval", TriggerEvent: client.EventInfo{Reason: "OOM", Severity: "critical"}},
		{ID: "b-id", Status: "blocked", TriggerEvent: client.EventInfo{Reason: "ImagePullBackOff", Severity: "warning"}},
		{ID: "c-id", Status: "resolved", TriggerEvent: client.EventInfo{Reason: "FailedScheduling", Severity: "info"}},
	}
}

func key(s string) tea.KeyMsg {
	switch s {
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func TestKey_DownUp_MovesCursorWithBounds(t *testing.T) {
	m := newModel(Options{}, nil)
	m.incidents = fakeIncidents()

	// down twice → cursor at last row
	m, _ = applyKey(t, m, "down")
	m, _ = applyKey(t, m, "down")
	if m.cursor != 2 {
		t.Fatalf("cursor after 2 down = %d, want 2", m.cursor)
	}
	// down again → still at last (clamp)
	m, _ = applyKey(t, m, "down")
	if m.cursor != 2 {
		t.Errorf("cursor should clamp at len-1, got %d", m.cursor)
	}
	// up x3 → cursor at 0
	for i := 0; i < 3; i++ {
		m, _ = applyKey(t, m, "up")
	}
	if m.cursor != 0 {
		t.Errorf("cursor after 3 up = %d, want 0 (clamped)", m.cursor)
	}
}

func TestKey_GG_HomeAndEnd(t *testing.T) {
	m := newModel(Options{}, nil)
	m.incidents = fakeIncidents()
	m.cursor = 1

	m, _ = applyKey(t, m, "g")
	if m.cursor != 0 {
		t.Errorf("g should jump to 0, got %d", m.cursor)
	}
	m, _ = applyKey(t, m, "G")
	if m.cursor != 2 {
		t.Errorf("G should jump to last, got %d", m.cursor)
	}
}

func TestKey_QuitKeys(t *testing.T) {
	for _, k := range []string{"q", "esc", "ctrl+c"} {
		m := newModel(Options{}, nil)
		var msg tea.KeyMsg
		switch k {
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "ctrl+c":
			msg = tea.KeyMsg{Type: tea.KeyCtrlC}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		_, cmd := m.Update(msg)
		if cmd == nil {
			t.Errorf("%q should return tea.Quit cmd, got nil", k)
		}
	}
}

func TestUpdate_LoadedClearsLoading(t *testing.T) {
	m := newModel(Options{}, nil)
	if !m.loading {
		t.Fatal("initial state should be loading")
	}
	mAny, _ := m.Update(incidentsLoadedMsg{incidents: fakeIncidents()})
	m = mAny.(model)
	if m.loading {
		t.Error("loading should clear after data arrives")
	}
	if m.err != nil {
		t.Errorf("err should clear on success, got %v", m.err)
	}
	if len(m.incidents) != 3 {
		t.Errorf("incidents = %d, want 3", len(m.incidents))
	}
}

func TestUpdate_LoadedClampsCursor(t *testing.T) {
	m := newModel(Options{}, nil)
	m.incidents = fakeIncidents()
	m.cursor = 2 // pointing at the last row
	// Backend returns only 1 incident now — cursor must clamp.
	mAny, _ := m.Update(incidentsLoadedMsg{incidents: fakeIncidents()[:1]})
	m = mAny.(model)
	if m.cursor != 0 {
		t.Errorf("cursor should clamp to len-1=0, got %d", m.cursor)
	}
}

func TestUpdate_LoadErr_SetsError(t *testing.T) {
	m := newModel(Options{}, nil)
	mAny, _ := m.Update(loadErrMsg{err: testErr("backend down")})
	m = mAny.(model)
	if m.err == nil || !strings.Contains(m.err.Error(), "backend down") {
		t.Errorf("err = %v, want containing \"backend down\"", m.err)
	}
	if m.loading {
		t.Error("loading should clear even on error")
	}
}

func TestUpdate_ActionDone_SetsBanner(t *testing.T) {
	m := newModel(Options{}, nil)
	m.incidents = fakeIncidents()
	mAny, _ := m.Update(actionDoneMsg{id: "a-id", verb: "approved", err: nil})
	m = mAny.(model)
	if !strings.Contains(m.banner, "approved") {
		t.Errorf("banner = %q, want containing \"approved\"", m.banner)
	}
	if m.bannerExpiry.IsZero() {
		t.Error("banner expiry should be set so the message clears later")
	}
}

func TestUpdate_ActionDone_ErrorBanner(t *testing.T) {
	m := newModel(Options{}, nil)
	mAny, _ := m.Update(actionDoneMsg{id: "a-id", verb: "approved", err: testErr("HTTP 403")})
	m = mAny.(model)
	if !strings.Contains(m.banner, "✕") || !strings.Contains(m.banner, "403") {
		t.Errorf("banner should mark failure + show error, got %q", m.banner)
	}
}

func TestSelectedID_OutOfRangeReturnsEmpty(t *testing.T) {
	m := newModel(Options{}, nil)
	if got := m.selectedID(); got != "" {
		t.Errorf("empty list selectedID = %q, want \"\"", got)
	}
	m.incidents = fakeIncidents()
	m.cursor = 99
	if got := m.selectedID(); got != "" {
		t.Errorf("cursor past end selectedID = %q, want \"\"", got)
	}
}

func TestRenderRow_PlainAndSelected(t *testing.T) {
	m := newModel(Options{}, nil)
	m.width, m.height = 100, 24
	inc := fakeIncidents()[0]

	plain := m.renderRow(inc, false, 60)
	sel := m.renderRow(inc, true, 60)
	// Selected row should render the cursor rail glyph.
	if !strings.Contains(sel, "▸") {
		t.Errorf("selected row should include ▸ rail, got %q", sel)
	}
	if strings.Contains(plain, "▸") {
		t.Errorf("plain row should not include ▸, got %q", plain)
	}
}

// ── Helpers ──────────────────────────────────────────────────────

type testErr string

func (e testErr) Error() string { return string(e) }

func applyKey(t *testing.T, m model, s string) (model, tea.Cmd) {
	t.Helper()
	mAny, cmd := m.Update(key(s))
	return mAny.(model), cmd
}
