package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print client version",
	RunE: func(c *cobra.Command, _ []string) error {
		fmt.Fprintf(c.OutOrStdout(),
			"kubectl-srepulse %s (commit %s, built %s)\n",
			versionInfo.version, versionInfo.commit, versionInfo.date,
		)
		return nil
	},
}
