// Package tui hosts the Bubble Tea program. v0.1 is a minimal welcome
// screen so the entrypoint runs end-to-end and the shared styles
// package is exercised against a real terminal — the three-pane layout
// (incident list / detail / thought-stream) lands in v0.2.
//
// Visual conventions match srepulse/design/ui_kits/cli/index.html:
//   - JetBrains Mono everywhere
//   - pulse teal for the prompt, the active phase, the live indicator
//   - a 3-dot terminal chrome at the top
//   - dim Fg3 for labels and separators
package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/srepulse/cli/internal/tui/styles"
)

// Options carries everything the TUI needs from the parent command.
type Options struct {
	ServerURL       string
	InsecureSkipTLS bool
	Cluster         string // optional cluster context label for the header chrome
}

// Run blocks until the user quits the TUI or ctx fires.
func Run(ctx context.Context, opts Options) error {
	if opts.Cluster == "" {
		opts.Cluster = "default"
	}
	p := tea.NewProgram(initialModel(opts), tea.WithAltScreen(), tea.WithContext(ctx))
	_, err := p.Run()
	return err
}

type model struct {
	opts Options
	w, h int
}

func initialModel(opts Options) model { return model{opts: opts} }

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	}
	return m, nil
}

// View renders the v0.1 welcome screen — a faithful translation of
// the CLI mock's terminal-window chrome + a placeholder body that
// names the v0.2 deliverable. Quitting on q/esc/ctrl+c.
func (m model) View() string {
	width := m.w
	if width == 0 {
		width = 80
	}

	// 3-dot terminal chrome + cluster label, mirroring the design mock.
	chrome := chrome(m.opts.Cluster, width)

	prompt := styles.Prompt.Render("~") + " " + styles.Dim.Render("$") + " " + styles.Pulse.Render("srepulse")
	cursor := lipgloss.NewStyle().Background(styles.BrandPulse).Render(" ")

	body := lipgloss.JoinVertical(lipgloss.Left,
		"",
		styles.HeaderHint.Render("connected to ")+styles.Pulse.Render(m.opts.ServerURL),
		"",
		prompt+cursor,
		"",
		styles.PanelBox.Width(width-4).Render(
			lipgloss.JoinVertical(lipgloss.Left,
				styles.Header.Render("srepulse")+" "+styles.HeaderHint.Render("· terminal client"),
				"",
				styles.Dim.Render("v0.1 placeholder. Three-pane TUI lands in v0.2 with the live"),
				styles.Dim.Render("incident feed, expandable detail, and the streaming"),
				styles.Dim.Render("thought-stream — same backend as the dashboard."),
				"",
				styles.Dim.Render("meanwhile try the one-shot subcommands:"),
				"  "+styles.Pulse.Render("kubectl srepulse list"),
				"  "+styles.Pulse.Render("kubectl srepulse show <id>"),
				"  "+styles.Pulse.Render("kubectl srepulse approve <id> --reason \"...\""),
				"  "+styles.Pulse.Render("kubectl srepulse logs <id> -f"),
				"",
				styles.Dim.Render("press q to quit"),
			),
		),
	)

	return lipgloss.JoinVertical(lipgloss.Left, chrome, body)
}

// chrome renders the design mock's three-dot terminal title bar with
// a right-aligned cluster context label. Width-aware so resizes don't
// break the visual.
func chrome(cluster string, width int) string {
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
		Render(dots + spaces(pad) + right)
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, n)
	for i := range out {
		out[i] = ' '
	}
	return string(out)
}
