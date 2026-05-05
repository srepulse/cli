package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/srepulse/cli/internal/client"
	"github.com/srepulse/cli/internal/render"
)

var (
	flagApproveReason string
	flagApproveYes    bool
)

var approveCmd = &cobra.Command{
	Use:               "approve <id>",
	Short:             "Approve a pending remediation",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeIncidentIDs,
	RunE:              runApprove,
}

func init() {
	approveCmd.Flags().StringVar(&flagApproveReason, "reason", "", "audit reason (recorded with the approval)")
	approveCmd.Flags().BoolVarP(&flagApproveYes, "yes", "y", false, "skip the confirmation prompt")
}

func runApprove(c *cobra.Command, args []string) error {
	id := args[0]
	if !flagApproveYes {
		fmt.Fprintf(c.ErrOrStderr(), "approve %s? this will trigger an apply against the cluster. (--yes to skip)\n", id)
		var s string
		if _, err := fmt.Fscanln(c.InOrStdin(), &s); err != nil {
			return fmt.Errorf("approval cancelled (no confirmation): %w", err)
		}
		if s != "y" && s != "yes" {
			return fmt.Errorf("approval cancelled")
		}
	}

	url, _ := resolveServer()
	cli := client.New(url, flagInsecure)
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()

	if err := cli.ApproveIncident(ctx, id, flagApproveReason); err != nil {
		return fmt.Errorf("approve %s: %w", id, err)
	}
	out := c.OutOrStdout()
	fmt.Fprintf(out, "%s approved %s\n", render.OK(out, "✓"), render.Pulse(out, id))
	return nil
}
