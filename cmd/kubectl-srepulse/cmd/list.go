package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/srepulse/cli/internal/client"
)

var (
	flagListJSON   bool
	flagListLimit  int
	flagListStatus string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List incidents",
	Long: `List incidents from the configured srepulse instance.

Without flags, prints a tabular view ordered newest-first. Use -o json
to emit the raw incident objects (useful for jq pipelines, scripts, or
sourcing into other commands).`,
	Example: `  kubectl srepulse list
  kubectl srepulse list --status awaiting_approval
  kubectl srepulse list -o json | jq '.[] | select(.status == "blocked") | .id'`,
	RunE: runList,
}

func init() {
	listCmd.Flags().BoolVarP(&flagListJSON, "output-json", "j", false, "emit raw JSON instead of a table")
	listCmd.Flags().IntVarP(&flagListLimit, "limit", "n", 50, "max rows to print")
	listCmd.Flags().StringVar(&flagListStatus, "status", "", "filter by status (e.g. awaiting_approval, blocked, resolved)")
}

func runList(c *cobra.Command, _ []string) error {
	url, src := resolveServer()
	cli := client.New(url, flagInsecure)
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

	if len(incs) == 0 {
		fmt.Fprintln(c.OutOrStdout(), "no incidents")
		return nil
	}

	w := tabwriter.NewWriter(c.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tSEVERITY\tNAMESPACE\tREASON\tAGE")
	for _, i := range incs {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			short(i.ID, 12),
			i.Status,
			defaultStr(i.TriggerEvent.Severity, "-"),
			defaultStr(i.TriggerEvent.Namespace, "-"),
			truncate(i.TriggerEvent.Reason, 40),
			ageString(i.CreatedAt),
		)
	}
	return w.Flush()
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
