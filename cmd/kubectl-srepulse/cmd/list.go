package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/srepulse/cli/internal/client"
	"github.com/srepulse/cli/internal/render"
)

var (
	flagListJSON   bool
	flagListLimit  int
	flagListStatus string
	flagListWatch  bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List incidents",
	Long: `List incidents from the configured srepulse instance.

Without flags, prints a tabular view ordered newest-first. Use -o json
to emit the raw incident objects (useful for jq pipelines, scripts, or
sourcing into other commands). Use --watch to stream live updates via
the agent's SSE feed (Ctrl+C to stop).`,
	Example: `  kubectl srepulse list
  kubectl srepulse list --status awaiting_approval
  kubectl srepulse list -w
  kubectl srepulse list -o json | jq '.[] | select(.status == "blocked") | .id'`,
	RunE: runList,
}

func init() {
	listCmd.Flags().BoolVarP(&flagListJSON, "output-json", "j", false, "emit raw JSON instead of a table")
	listCmd.Flags().IntVarP(&flagListLimit, "limit", "n", 50, "max rows to print")
	listCmd.Flags().StringVar(&flagListStatus, "status", "", "filter by status (e.g. awaiting_approval, blocked, resolved)")
	listCmd.Flags().BoolVarP(&flagListWatch, "watch", "w", false, "stream live updates via SSE; Ctrl+C to stop")
}

func runList(c *cobra.Command, _ []string) error {
	url, src := resolveServer()
	cli := client.New(url, flagInsecure)

	if flagListWatch {
		return runListWatch(c, cli, url)
	}

	ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
	defer cancel()

	incs, err := cli.ListIncidents(ctx)
	if err != nil {
		return fmt.Errorf("list incidents (server: %s, source: %s): %w", url, src, err)
	}

	// Optional client-side status filter — keeps the API surface small
	// while still letting operators slice by approval state without
	// piping through jq.
	if flagListStatus != "" {
		want := strings.ToLower(flagListStatus)
		filtered := incs[:0]
		for _, i := range incs {
			if strings.ToLower(i.Status) == want {
				filtered = append(filtered, i)
			}
		}
		incs = filtered
	}
	if len(incs) > flagListLimit {
		incs = incs[:flagListLimit]
	}

	if flagListJSON {
		enc := json.NewEncoder(c.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(incs)
	}

	out := c.OutOrStdout()
	if len(incs) == 0 {
		fmt.Fprintln(out, render.Dim(out, "no incidents"))
		return nil
	}

	// Build the rows as colour-styled strings, then pad each column
	// to its widest entry using lipgloss.Width — that helper measures
	// the *visible* width (it strips ANSI escapes), which is what we
	// need for clean alignment when the cells are coloured. The
	// stdlib tabwriter counts raw bytes and so over-pads the columns
	// that came back styled.
	headers := []string{"ID", "STATUS", "SEVERITY", "NAMESPACE", "REASON", "AGE"}
	rows := [][]string{}
	for _, i := range incs {
		rows = append(rows, []string{
			render.Pulse(out, short(i.ID, 12)),
			render.Status(out, i.Status),
			render.Severity(out, defaultStr(i.TriggerEvent.Severity, "-")),
			defaultStr(i.TriggerEvent.Namespace, "-"),
			truncate(i.TriggerEvent.Reason, 40),
			render.Dim(out, ageString(i.CreatedAt)),
		})
	}
	fmt.Fprintln(out, formatTable(out, headers, rows))
	return nil
}

// runListWatch is the --watch path. Initial REST fetch + SSE stream;
// re-renders the table on every incident update. Filters and limit
// flags apply to each render. Ctrl+C exits cleanly.
func runListWatch(c *cobra.Command, cli *client.Client, url string) error {
	ctx, stop := signal.NotifyContext(c.Context(), os.Interrupt)
	defer stop()

	// Initial snapshot via REST so the first render isn't blank
	// while we wait for the first SSE event.
	initCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	initial, err := cli.ListIncidents(initCtx)
	cancel()
	if err != nil {
		return fmt.Errorf("list incidents (server: %s): %w", url, err)
	}

	// Reconcile-by-id state. Map keeps O(1) update + lookup; we
	// re-sort newest-first on every render. The agent emits one
	// incident per SSE event whenever it transitions.
	state := make(map[string]client.Incident, len(initial))
	for _, inc := range initial {
		state[inc.ID] = inc
	}

	out := c.OutOrStdout()
	paint := func() {
		incs := snapshotSorted(state)
		if flagListStatus != "" {
			want := strings.ToLower(flagListStatus)
			f := incs[:0]
			for _, i := range incs {
				if strings.ToLower(i.Status) == want {
					f = append(f, i)
				}
			}
			incs = f
		}
		if len(incs) > flagListLimit {
			incs = incs[:flagListLimit]
		}

		if flagListJSON {
			// JSON-watch mode: one snapshot per event, NDJSON-friendly
			// — wrap each render in {"at": ..., "incidents": [...]} so
			// downstream tools can join them on `at`.
			enc := json.NewEncoder(out)
			_ = enc.Encode(map[string]any{
				"at":        time.Now().UTC().Format(time.RFC3339),
				"incidents": incs,
			})
			return
		}

		// Tabular watch mode — clear screen + render so the table
		// stays in place when a TTY is attached. On a pipe we just
		// write the table back-to-back (no clear) so the output is
		// log-friendly.
		if render.Enabled(out) {
			fmt.Fprint(out, "\033[H\033[2J") // ANSI clear + home
		} else {
			fmt.Fprintln(out, render.Dim(out, "─── update @ "+time.Now().UTC().Format(time.RFC3339)+" ──────────"))
		}

		if len(incs) == 0 {
			fmt.Fprintln(out, render.Dim(out, "no incidents"))
			return
		}
		headers := []string{"ID", "STATUS", "SEVERITY", "NAMESPACE", "REASON", "AGE"}
		rows := [][]string{}
		for _, i := range incs {
			rows = append(rows, []string{
				render.Pulse(out, short(i.ID, 12)),
				render.Status(out, i.Status),
				render.Severity(out, defaultStr(i.TriggerEvent.Severity, "-")),
				defaultStr(i.TriggerEvent.Namespace, "-"),
				truncate(i.TriggerEvent.Reason, 40),
				render.Dim(out, ageString(i.CreatedAt)),
			})
		}
		fmt.Fprintln(out, formatTable(out, headers, rows))
		fmt.Fprintln(out, render.Dim(out, fmt.Sprintf("watching %s · %s · Ctrl+C to stop",
			url, time.Now().UTC().Format("15:04:05"))))
	}

	// First paint.
	paint()

	// Stream updates. On any error we exit so the operator can
	// retry; SSE reconnect with backoff is a v0.2 polish.
	events, errCh := cli.StreamIncidents(ctx)
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errCh:
			if err != nil {
				return fmt.Errorf("watch stream: %w", err)
			}
			return nil
		case ev, ok := <-events:
			if !ok {
				return nil
			}
			state[ev.ID] = ev
			paint()
		}
	}
}

// snapshotSorted returns the map values as a slice ordered newest-
// first by UpdatedAt (falls back to CreatedAt). Stable sort means
// repeat renders don't shuffle equal-timestamp rows.
func snapshotSorted(m map[string]client.Incident) []client.Incident {
	out := make([]client.Incident, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool {
		ti := out[i].UpdatedAt
		if ti == "" {
			ti = out[i].CreatedAt
		}
		tj := out[j].UpdatedAt
		if tj == "" {
			tj = out[j].CreatedAt
		}
		return ti > tj
	})
	return out
}

// formatTable lays out a table where individual cells may already be
// ANSI-coloured. Column widths come from the *visible* width
// (lipgloss.Width) so coloured and plain cells line up. Header row is
// dim per the design's CLI mock convention.
func formatTable(out interface{ Write([]byte) (int, error) }, headers []string, rows [][]string) string {
	cols := len(headers)
	widths := make([]int, cols)
	// Headers are coloured before measuring, so the dim escapes get
	// stripped by lipgloss.Width too.
	rendered := make([][]string, 0, len(rows)+1)
	headerRow := make([]string, cols)
	for i, h := range headers {
		headerRow[i] = render.Dim(out, h)
		widths[i] = lipgloss.Width(headerRow[i])
	}
	rendered = append(rendered, headerRow)
	for _, r := range rows {
		row := make([]string, cols)
		for i := 0; i < cols && i < len(r); i++ {
			row[i] = r[i]
			if w := lipgloss.Width(r[i]); w > widths[i] {
				widths[i] = w
			}
		}
		rendered = append(rendered, row)
	}

	// Pad every cell to its column's max width. The last column
	// doesn't get padded — saves trailing whitespace on every row.
	const sep = "  "
	var b strings.Builder
	for ri, r := range rendered {
		for i, cell := range r {
			pad := widths[i] - lipgloss.Width(cell)
			if pad < 0 {
				pad = 0
			}
			b.WriteString(cell)
			if i < cols-1 {
				b.WriteString(strings.Repeat(" ", pad))
				b.WriteString(sep)
			}
		}
		if ri < len(rendered)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func short(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func defaultStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func ageString(iso string) string {
	t, err := time.Parse(time.RFC3339Nano, iso)
	if err != nil {
		// Fall back to the looser RFC3339 — server emits both.
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
