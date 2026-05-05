package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"
	"github.com/srepulse/cli/internal/client"
	"github.com/srepulse/cli/internal/render"
)

var (
	flagShowJSON  bool
	flagShowWatch bool
)

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a single incident's detail",
	Args:  cobra.ExactArgs(1),
	Example: `  kubectl srepulse incidents show 8f6fc09b25ac
  kubectl srepulse i show 8f6fc09b25ac -w
  kubectl srepulse i show 8f6fc09b25ac -j | jq .hypotheses`,
	ValidArgsFunction: completeIncidentIDs,
	RunE:              runShow,
}

func init() {
	showCmd.Flags().BoolVarP(&flagShowJSON, "output-json", "j", false, "emit raw JSON")
	showCmd.Flags().BoolVarP(&flagShowWatch, "watch", "w", false, "stream live updates via SSE; Ctrl+C to stop")
}

func runShow(c *cobra.Command, args []string) error {
	id := args[0]
	url, src := resolveServer()
	cli := client.New(url, flagInsecure)

	if flagShowWatch {
		return runShowWatch(c, cli, url, id)
	}

	ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
	defer cancel()

	inc, err := cli.GetIncident(ctx, id)
	if err != nil {
		return fmt.Errorf("show incident %s (server: %s, source: %s): %w", id, url, src, err)
	}

	if flagShowJSON {
		enc := json.NewEncoder(c.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(inc)
	}

	out := c.OutOrStdout()
	// Header: pulse-coloured ID + status. The dim middle dot mirrors
	// the design mock's "pulse · dim · pulse" cadence for chip rows.
	fmt.Fprintf(out, "%s %s %s\n",
		render.Pulse(out, inc.ID),
		render.Dim(out, "·"),
		render.Status(out, inc.Status),
	)
	field := func(label, value string) {
		fmt.Fprintf(out, "  %s %s\n",
			render.Dim(out, fmt.Sprintf("%-9s", label)),
			value,
		)
	}
	field("reason:", inc.TriggerEvent.Reason)
	field("ns/res:", fmt.Sprintf("%s/%s",
		defaultStr(inc.TriggerEvent.Namespace, "-"),
		defaultStr(inc.TriggerEvent.Name, "-")))
	field("severity:", render.Severity(out, defaultStr(inc.TriggerEvent.Severity, "-")))
	field("source:", defaultStr(inc.TriggerEvent.Source, "-"))
	field("age:", render.Dim(out, ageString(inc.CreatedAt)))

	if inc.Summary != "" {
		fmt.Fprintf(out, "\n%s\n  %s\n", render.Dim(out, "summary:"), inc.Summary)
	}
	if len(inc.Hypotheses) > 0 {
		fmt.Fprintf(out, "\n%s\n", render.Dim(out, "hypotheses:"))
		for _, h := range inc.Hypotheses {
			// Confidence is the value the operator's eye is drawn
			// to — pulse it so it reads first.
			fmt.Fprintf(out, "  • [%s] %s\n",
				render.Pulse(out, fmt.Sprintf("%.2f", h.Confidence)),
				h.Title,
			)
		}
	}
	if inc.Remediation != nil {
		fmt.Fprintf(out, "\n%s\n  %s\n", render.Dim(out, "remediation:"), inc.Remediation.Description)
	}
	return nil
}

// runShowWatch streams updates for one incident. Initial GET, then
// SSE-filtered to ev.ID == id. Re-renders the detail (clear + paint)
// each time the agent emits a new state for this incident. Ctrl+C
// exits cleanly.
func runShowWatch(c *cobra.Command, cli *client.Client, url, id string) error {
	ctx, stop := signal.NotifyContext(c.Context(), os.Interrupt)
	defer stop()

	initCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	inc, err := cli.GetIncident(initCtx, id)
	cancel()
	if err != nil {
		return fmt.Errorf("show incident %s (server: %s): %w", id, url, err)
	}

	out := c.OutOrStdout()
	paint := func(i client.Incident) {
		if flagShowJSON {
			enc := json.NewEncoder(out)
			_ = enc.Encode(map[string]any{
				"at":       time.Now().UTC().Format(time.RFC3339),
				"incident": i,
			})
			return
		}
		// TTY: clear + paint, so the detail stays in place.
		// Pipe: emit a separator + re-render so each snapshot is a
		// readable block in a log.
		if renderEnabled(out) {
			fmt.Fprint(out, "\033[H\033[2J")
		} else {
			fmt.Fprintln(out, "─── update @ "+time.Now().UTC().Format(time.RFC3339)+" ──────────")
		}
		printShow(out, i)
		fmt.Fprintln(out)
		fmt.Fprintln(out, render.Dim(out, fmt.Sprintf("watching %s · %s · Ctrl+C to stop",
			id, time.Now().UTC().Format("15:04:05"))))
	}

	paint(*inc)

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
			// Filter to the requested incident.
			if ev.ID == id {
				paint(ev)
			}
		}
	}
}

// printShow is the body of the non-watch render path, factored out so
// runShowWatch can re-use it on every refresh.
func printShow(out interface{ Write([]byte) (int, error) }, inc client.Incident) {
	fmt.Fprintf(out, "%s %s %s\n",
		render.Pulse(out, inc.ID),
		render.Dim(out, "·"),
		render.Status(out, inc.Status),
	)
	field := func(label, value string) {
		fmt.Fprintf(out, "  %s %s\n",
			render.Dim(out, fmt.Sprintf("%-9s", label)),
			value,
		)
	}
	field("reason:", inc.TriggerEvent.Reason)
	field("ns/res:", fmt.Sprintf("%s/%s",
		defaultStr(inc.TriggerEvent.Namespace, "-"),
		defaultStr(inc.TriggerEvent.Name, "-")))
	field("severity:", render.Severity(out, defaultStr(inc.TriggerEvent.Severity, "-")))
	field("source:", defaultStr(inc.TriggerEvent.Source, "-"))
	field("age:", render.Dim(out, ageString(inc.CreatedAt)))
	if inc.Summary != "" {
		fmt.Fprintf(out, "\n%s\n  %s\n", render.Dim(out, "summary:"), inc.Summary)
	}
	if len(inc.Hypotheses) > 0 {
		fmt.Fprintf(out, "\n%s\n", render.Dim(out, "hypotheses:"))
		for _, h := range inc.Hypotheses {
			fmt.Fprintf(out, "  • [%s] %s\n",
				render.Pulse(out, fmt.Sprintf("%.2f", h.Confidence)),
				h.Title,
			)
		}
	}
	if inc.Remediation != nil {
		fmt.Fprintf(out, "\n%s\n  %s\n", render.Dim(out, "remediation:"), inc.Remediation.Description)
	}
}

// renderEnabled is a thin alias so this file doesn't have to import
// the render package as a function namespace.
func renderEnabled(w interface{ Write([]byte) (int, error) }) bool {
	if f, ok := w.(*os.File); ok {
		return render.Enabled(f)
	}
	return false
}
