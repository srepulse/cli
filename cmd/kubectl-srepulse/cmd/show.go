package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/srepulse/cli/internal/client"
)

var flagShowJSON bool

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a single incident's detail",
	Args:  cobra.ExactArgs(1),
	Example: `  kubectl srepulse show 8f6fc09b25ac
  kubectl srepulse show 8f6fc09b25ac -j | jq .hypotheses`,
	RunE: runShow,
}

func init() {
	showCmd.Flags().BoolVarP(&flagShowJSON, "output-json", "j", false, "emit raw JSON")
}

func runShow(c *cobra.Command, args []string) error {
	url, src := resolveServer()
	cli := client.New(url, flagInsecure)
	ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
	defer cancel()

	inc, err := cli.GetIncident(ctx, args[0])
	if err != nil {
		return fmt.Errorf("show incident %s (server: %s, source: %s): %w", args[0], url, src, err)
	}

	if flagShowJSON {
		enc := json.NewEncoder(c.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(inc)
	}

	out := c.OutOrStdout()
	fmt.Fprintf(out, "%s · %s\n", inc.ID, inc.Status)
	fmt.Fprintf(out, "  reason:    %s\n", inc.TriggerEvent.Reason)
	fmt.Fprintf(out, "  ns/res:    %s/%s\n",
		defaultStr(inc.TriggerEvent.Namespace, "-"),
		defaultStr(inc.TriggerEvent.Name, "-"))
	fmt.Fprintf(out, "  severity:  %s\n", defaultStr(inc.TriggerEvent.Severity, "-"))
	fmt.Fprintf(out, "  source:    %s\n", defaultStr(inc.TriggerEvent.Source, "-"))
	fmt.Fprintf(out, "  age:       %s\n", ageString(inc.CreatedAt))
	if inc.Summary != "" {
		fmt.Fprintf(out, "\nsummary:\n  %s\n", inc.Summary)
	}
	if len(inc.Hypotheses) > 0 {
		fmt.Fprintln(out, "\nhypotheses:")
		for _, h := range inc.Hypotheses {
			fmt.Fprintf(out, "  • [%.2f] %s\n", h.Confidence, h.Title)
		}
	}
	if inc.Remediation != nil {
		fmt.Fprintf(out, "\nremediation:\n  %s\n", inc.Remediation.Description)
	}
	return nil
}
