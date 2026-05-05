package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/srepulse/cli/internal/client"
	"github.com/srepulse/cli/internal/render"
)

var flagRejectReason string

var rejectCmd = &cobra.Command{
	Use:               "reject <id>",
	Short:             "Reject a pending remediation",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeIncidentIDs,
	RunE:              runReject,
}

func init() {
	rejectCmd.Flags().StringVar(&flagRejectReason, "reason", "", "audit reason (recorded with the rejection)")
	_ = rejectCmd.MarkFlagRequired("reason")
}

func runReject(c *cobra.Command, args []string) error {
	id := args[0]
	url, _ := resolveServer()
	cli := client.New(url, flagInsecure)
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()

	if err := cli.RejectIncident(ctx, id, flagRejectReason); err != nil {
		return fmt.Errorf("reject %s: %w", id, err)
	}
	out := c.OutOrStdout()
	fmt.Fprintf(out, "%s rejected %s\n", render.Warn(out, "↳"), render.Pulse(out, id))
	return nil
}
