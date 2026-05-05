package cmd

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/srepulse/cli/internal/client"
)

var flagLogsFollow bool

var logsCmd = &cobra.Command{
	Use:   "logs <id>",
	Short: "Stream the agent's reasoning for an incident",
	Args:  cobra.ExactArgs(1),
	Long: `Print the agent's thought-stream events for an incident — tool calls,
hypotheses, evidence, and node transitions. Use -f to follow the
stream live (Ctrl+C to stop).`,
	RunE: runLogs,
}

func init() {
	logsCmd.Flags().BoolVarP(&flagLogsFollow, "follow", "f", false, "follow the stream until Ctrl+C")
}

func runLogs(c *cobra.Command, args []string) error {
	id := args[0]
	url, _ := resolveServer()
	cli := client.New(url, flagInsecure)

	// Cancel on Ctrl+C so the SSE goroutine drains cleanly.
	ctx, stop := signal.NotifyContext(c.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if !flagLogsFollow {
		// One-shot: dump the trace already on the incident, no SSE.
		shortCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		inc, err := cli.GetIncident(shortCtx, id)
		if err != nil {
			return fmt.Errorf("fetch incident %s: %w", id, err)
		}
		for _, t := range inc.Trace {
			printTrace(c, t)
		}
		return nil
	}

	// Follow mode — open SSE and print every event as it lands.
	events, errCh := cli.StreamThoughts(ctx, id)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return nil
			}
			printTrace(c, ev)
		case err := <-errCh:
			if err != nil {
				return fmt.Errorf("stream: %w", err)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func printTrace(c *cobra.Command, t client.TraceStep) {
	fmt.Fprintf(c.OutOrStdout(), "[%s] %s · %s · %s\n",
		t.At.Format(time.RFC3339),
		defaultStr(t.Node, "?"),
		t.Kind,
		t.Message,
	)
}
