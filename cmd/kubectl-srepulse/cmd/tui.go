package cmd

import (
	"github.com/spf13/cobra"
	"github.com/srepulse/cli/internal/tui"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive TUI",
	Long: `Open the three-pane Bubble Tea TUI: incident list (left), incident
detail (centre), live thought-stream (right). Bare invocation of
"kubectl srepulse" enters this command by default.

The TUI reads from the same configured backend as the one-shot
subcommands and respects the same approval gates and namespace
policies — Slack/dashboard/CLI/TUI are all the same audit actors.`,
	RunE: func(c *cobra.Command, _ []string) error {
		url, _ := resolveServer()
		return tui.Run(c.Context(), tui.Options{ServerURL: url, InsecureSkipTLS: flagInsecure})
	},
}
