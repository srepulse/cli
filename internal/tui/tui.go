// Package tui hosts the interactive Bubble Tea program for
// kubectl-srepulse. Layout matches the design system's CLI mock + the
// issue-#5 RFC:
//
//	┌─────────────────────────────────────────────────────────────────┐
//	│ ● ● ●                                          prod-us-east-1   │  chrome
//	├──────────────────┬──────────────────────────────────────────────┤
//	│ INCIDENTS (4)    │ <id> · <status>                              │
//	│ ▸ id  sev  reason│  reason / ns / severity / source / age        │
//	│   id  sev  reason│  summary                                      │
//	│                  │  hypotheses                                   │
//	│                  │  remediation                                  │
//	├──────────────────┴──────────────────────────────────────────────┤
//	│ ↑↓ navigate · r refresh · a approve · R reject · q quit         │
//	└─────────────────────────────────────────────────────────────────┘
//
// State updates flow through typed Bubble Tea messages; the model is
// pure. Network I/O is wrapped in tea.Cmd factories so the runtime
// schedules + handles cancellation. Auto-refresh fires every 5 s.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/srepulse/cli/internal/client"
	"github.com/srepulse/cli/internal/tui/styles"
)

// Options carries everything the TUI needs from the parent command.
type Options struct {
	ServerURL       string
	InsecureSkipTLS bool
	Cluster         string // optional context label rendered in the chrome
}

// Run blocks until the user quits the TUI or ctx fires.
func Run(ctx context.Context, opts Options) error {
	if opts.Cluster == "" {
		opts.Cluster = "default"
	}
	cli := client.New(opts.ServerURL, opts.InsecureSkipTLS)
	p := tea.NewProgram(
		newModel(opts, cli),
		tea.WithAltScreen(),
		tea.WithContext(ctx),
	)
	_, err := p.Run()
	return err
}

// ── Messages ─────────────────────────────────────────────────────

type incidentsLoadedMsg struct{ incidents []client.Incident }
type loadErrMsg struct{ err error }
type actionDoneMsg struct {
	id   string
	verb string
	err  error
}
type tickMsg time.Time

const refreshInterval = 5 * time.Second

// ── Model ────────────────────────────────────────────────────────

type model struct {
	opts Options
	cli  *client.Client

	width, height int
	incidents     []client.Incident
	cursor        int
	loading       bool
	err           error

	// Action result banner — surfaces approval / rejection success or
	// failure for ~3 s before clearing.
	banner       string
	bannerStyle  lipgloss.Style
	bannerExpiry time.Time
}

func newModel(opts Options, cli *client.Client) model {
	return model{opts: opts, cli: cli, loading: true}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.fetchCmd(), tickEvery(refreshInterval))
}

// fetchCmd issues a List against the backend and produces either a
// loaded message or an error message — Bubble Tea dispatches whichever
// fires through Update().
func (m model) fetchCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		incs, err := m.cli.ListIncidents(ctx)
		if err != nil {
			return loadErrMsg{err}
		}
		return incidentsLoadedMsg{incs}
	}
}

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// approveCmd / rejectCmd encode the side-effecting actions as Cmds so
// they run on the Bubble Tea worker without blocking the View.
func (m model) approveCmd(id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := m.cli.ApproveIncident(ctx, id, "approved from TUI")
		return actionDoneMsg{id: id, verb: "approved", err: err}
	}
}

func (m model) rejectCmd(id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := m.cli.RejectIncident(ctx, id, "rejected from TUI")
		return actionDoneMsg{id: id, verb: "rejected", err: err}
	}
}

// ── Update ───────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.bannerExpiry.IsZero() && time.Now().After(m.bannerExpiry) {
		m.banner = ""
		m.bannerExpiry = time.Time{}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tickMsg:
		// Re-fetch on every tick. Don't cancel any in-flight fetch —
		// last-write-wins via the next loadedMsg.
		return m, tea.Batch(m.fetchCmd(), tickEvery(refreshInterval))

	case incidentsLoadedMsg:
		m.incidents = msg.incidents
		m.loading = false
		m.err = nil
		// Clamp cursor when the list shrinks (incidents can age out).
		if m.cursor >= len(m.incidents) {
			m.cursor = maxInt(0, len(m.incidents)-1)
		}
		return m, nil

	case loadErrMsg:
		m.err = msg.err
		m.loading = false
		return m, nil

	case actionDoneMsg:
		if msg.err != nil {
			m.banner = fmt.Sprintf("✕ %s %s: %v", msg.verb, short(msg.id, 12), msg.err)
			m.bannerStyle = lipgloss.NewStyle().Foreground(styles.StatusError)
		} else {
			m.banner = fmt.Sprintf("✓ %s %s", msg.verb, short(msg.id, 12))
			m.bannerStyle = lipgloss.NewStyle().Foreground(styles.StatusOk)
		}
		m.bannerExpiry = time.Now().Add(3 * time.Second)
		// Reload list so the row's status refreshes immediately.
		return m, m.fetchCmd()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// handleKey is its own method so the keymap reads top-to-bottom in
// one place. Keep this in sync with renderHelp.
func (m model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		if m.cursor < len(m.incidents)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		m.cursor = maxInt(0, len(m.incidents)-1)
	case "r":
		// Lowercase r — manual refresh. Capital R below triggers
		// reject; the design system recommends capitals for
		// destructive keybinds (issue #5 RFC).
		return m, m.fetchCmd()
	case "a":
		if id := m.selectedID(); id != "" {
			return m, m.approveCmd(id)
		}
	case "R":
		if id := m.selectedID(); id != "" {
			return m, m.rejectCmd(id)
		}
	}
	return m, nil
}

func (m model) selectedID() string {
	if m.cursor < 0 || m.cursor >= len(m.incidents) {
		return ""
	}
	return m.incidents[m.cursor].ID
}

// ── View ─────────────────────────────────────────────────────────

func (m model) View() string {
	width := m.width
	if width == 0 {
		width = 100
	}
	height := m.height
	if height == 0 {
		height = 30
	}

	chrome := renderChrome(m.opts.Cluster, width)

	bodyH := height - 2
	if m.banner != "" {
		bodyH--
	}
	if bodyH < 6 {
		bodyH = 6
	}

	listW := minInt(40, width/3)
	if listW < 28 {
		listW = 28
	}
	detailW := width - listW - 1

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderList(listW, bodyH),
		divider(bodyH),
		m.renderDetail(detailW, bodyH),
	)

	parts := []string{chrome, body}
	if m.banner != "" {
		parts = append(parts, m.bannerStyle.Render(m.banner))
	}
	parts = append(parts, renderHelp(width, len(m.incidents) > 0))
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// renderList draws the left pane. Heading shows the count; rows are
// short-id + severity + reason (truncated to fit), with the cursor
// row pulse-railed.
func (m model) renderList(width, height int) string {
	header := styles.Dim.Bold(true).Render(fmt.Sprintf("INCIDENTS (%d)", len(m.incidents)))
	if m.loading {
		return lipgloss.NewStyle().Width(width).Height(height).Render(
			lipgloss.JoinVertical(lipgloss.Left, header, "", styles.Dim.Render("loading…")),
		)
	}
	if m.err != nil {
		return lipgloss.NewStyle().Width(width).Height(height).Render(
			lipgloss.JoinVertical(lipgloss.Left,
				header,
				"",
				styles.Err.Render("error:"),
				styles.Dim.Render(wrapTo(m.err.Error(), width-2)),
				"",
				styles.Dim.Render("press r to retry"),
			),
		)
	}
	if len(m.incidents) == 0 {
		return lipgloss.NewStyle().Width(width).Height(height).Render(
			lipgloss.JoinVertical(lipgloss.Left, header, "", styles.Dim.Render("no incidents")),
		)
	}

	rows := []string{header, ""}
	avail := height - 2
	if avail < 1 {
		avail = 1
	}
	// Window scroll: keep cursor visible. Centre when possible.
	start := 0
	if m.cursor >= avail {
		start = m.cursor - avail/2
	}
	end := minInt(len(m.incidents), start+avail)
	if end-start < avail {
		start = maxInt(0, end-avail)
	}
	for i := start; i < end; i++ {
		rows = append(rows, m.renderRow(m.incidents[i], i == m.cursor, width))
	}
	return lipgloss.NewStyle().Width(width).Height(height).Render(
		lipgloss.JoinVertical(lipgloss.Left, rows...),
	)
}

// renderRow formats one incident. Selected row gets a pulse left-rail
// and bold; unselected rows are plain.
func (m model) renderRow(inc client.Incident, selected bool, width int) string {
	id := short(inc.ID, 12)
	sev := defaultStr(inc.TriggerEvent.Severity, "-")
	// Reserve room for: 2-char rail + space + id + 2 spaces + sev + 1 space.
	avail := width - 2 - len(id) - 2 - lipgloss.Width(sev) - 2
	reason := truncateRune(defaultStr(inc.TriggerEvent.Reason, "-"), maxInt(0, avail))

	idStyled := styles.Pulse.Render(id)
	sevStyled := styles.SeverityStyle(sev).Render(sev)
	row := fmt.Sprintf("%s  %s  %s", idStyled, sevStyled, reason)

	if selected {
		rail := lipgloss.NewStyle().Foreground(styles.BrandPulse).Bold(true).Render("▸ ")
		return lipgloss.NewStyle().
			Width(width).
			Background(lipgloss.Color("#0e1216")).
			Render(rail + row)
	}
	return lipgloss.NewStyle().Width(width).Render("  " + row)
}

// renderDetail draws the right pane.
func (m model) renderDetail(width, height int) string {
	if width < 20 {
		return ""
	}
	if m.loading {
		return blankPane(width, height, styles.Dim.Render("loading…"))
	}
	if m.cursor < 0 || m.cursor >= len(m.incidents) {
		return blankPane(width, height, styles.Dim.Render("no incident selected"))
	}
	inc := m.incidents[m.cursor]

	header := lipgloss.JoinHorizontal(lipgloss.Top,
		styles.Pulse.Render(inc.ID),
		" "+styles.Dim.Render("·")+" ",
		styles.SeverityStyle(inc.Status).Render(inc.Status),
	)

	field := func(label, value string) string {
		return styles.Dim.Render(fmt.Sprintf("%-9s ", label)) + value
	}
	fields := []string{
		field("reason:", inc.TriggerEvent.Reason),
		field("ns/res:", fmt.Sprintf("%s/%s",
			defaultStr(inc.TriggerEvent.Namespace, "-"),
			defaultStr(inc.TriggerEvent.Name, "-"))),
		field("severity:", styles.SeverityStyle(inc.TriggerEvent.Severity).Render(defaultStr(inc.TriggerEvent.Severity, "-"))),
		field("source:", defaultStr(inc.TriggerEvent.Source, "-")),
		field("age:", styles.Dim.Render(ageString(inc.CreatedAt))),
	}

	body := []string{header, ""}
	body = append(body, fields...)
	if inc.Summary != "" {
		body = append(body, "", styles.Dim.Render("summary:"), wrapTo(inc.Summary, width-2))
	}
	if len(inc.Hypotheses) > 0 {
		body = append(body, "", styles.Dim.Render("hypotheses:"))
		for _, h := range inc.Hypotheses {
			conf := styles.Pulse.Render(fmt.Sprintf("%.2f", h.Confidence))
			body = append(body, "  "+styles.Dim.Render("•")+" ["+conf+"] "+h.Title)
		}
	}
	if inc.Remediation != nil && inc.Remediation.Description != "" {
		body = append(body, "", styles.Dim.Render("remediation:"), wrapTo(inc.Remediation.Description, width-2))
	}

	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(0, 1).
		Render(lipgloss.JoinVertical(lipgloss.Left, body...))
}

// renderChrome lays out the 3-dot terminal title bar + cluster label.
func renderChrome(cluster string, width int) string {
	dot := func(c string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render("●")
	}
	dots := dot("#ff5f56") + " " + dot("#ffbd2e") + " " + dot("#27c93f")
	right := styles.HeaderHint.Render(fmt.Sprintf("srepulse — %s", cluster))
	pad := width - lipgloss.Width(dots) - lipgloss.Width(right) - 4
	if pad < 1 {
		pad = 1
	}
	return lipgloss.NewStyle().
		Background(styles.BgBase).
		Padding(0, 2).
		Width(width).
		Render(dots + strings.Repeat(" ", pad) + right)
}

// renderHelp is the keybinding hint bar.
func renderHelp(width int, hasIncidents bool) string {
	keys := []string{
		styles.Pulse.Render("↑↓") + " " + styles.Dim.Render("navigate"),
		styles.Pulse.Render("r") + " " + styles.Dim.Render("refresh"),
	}
	if hasIncidents {
		keys = append(keys,
			styles.Pulse.Render("a")+" "+styles.Dim.Render("approve"),
			styles.Pulse.Render("R")+" "+styles.Dim.Render("reject"),
		)
	}
	keys = append(keys, styles.Pulse.Render("q")+" "+styles.Dim.Render("quit"))
	bar := strings.Join(keys, "  "+styles.Dim.Render("·")+"  ")
	return lipgloss.NewStyle().Width(width).Padding(0, 1).Render(bar)
}

// ── Helpers ──────────────────────────────────────────────────────

func divider(height int) string {
	col := styles.Dim.Render("│")
	rows := make([]string, height)
	for i := range rows {
		rows[i] = col
	}
	return strings.Join(rows, "\n")
}

func blankPane(width, height int, msg string) string {
	return lipgloss.NewStyle().Width(width).Height(height).Padding(0, 1).Render(msg)
}

func short(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func defaultStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func truncateRune(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}

func wrapTo(s string, width int) string {
	if width <= 0 {
		return s
	}
	var lines []string
	for _, paragraph := range strings.Split(s, "\n") {
		words := strings.Fields(paragraph)
		var cur string
		for _, w := range words {
			if cur == "" {
				cur = w
				continue
			}
			if len(cur)+1+len(w) > width {
				lines = append(lines, cur)
				cur = w
			} else {
				cur += " " + w
			}
		}
		if cur != "" {
			lines = append(lines, cur)
		}
	}
	return strings.Join(lines, "\n")
}

func ageString(iso string) string {
	t, err := time.Parse(time.RFC3339Nano, iso)
	if err != nil {
		t, err = time.Parse(time.RFC3339, iso)
		if err != nil {
			return "?"
		}
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
