package cmd

import "github.com/spf13/cobra"

// incidentsCmd hosts every incident-scoped subcommand:
//
//	kubectl srepulse incidents list             ↳ catalog
//	kubectl srepulse incidents show <id>        ↳ detail
//	kubectl srepulse incidents approve <id>     ↳ act
//	kubectl srepulse incidents reject  <id>     ↳ act
//	kubectl srepulse incidents logs    <id>     ↳ stream agent reasoning
//
// Aliased "i" / "inc" so the natural Slack-paste form stays terse:
// `kubectl srepulse i approve abc`.
//
// Same flow is also reachable from the TUI (`kubectl srepulse tui`),
// which uses the same client / audit path so all four surfaces
// (TUI, CLI, Slack, dashboard) share an actor identity.
var incidentsCmd = &cobra.Command{
	Use:     "incidents",
	Aliases: []string{"i", "inc"},
	Short:   "Browse + act on incidents",
	Long: `Browse and act on incidents from the configured srepulse instance.

  list      table of recent incidents (optionally streaming via --watch)
  show      one incident's detail (--watch streams updates via SSE)
  approve   approve a pending remediation
  reject    reject a pending remediation; routes to a manual playbook
  logs      stream the agent's reasoning for an incident (-f to follow)

Aliased "i" / "inc". The same flow lives under the TUI
(kubectl srepulse tui).`,
}

func init() {
	// All incident-scoped subcommands live under this parent. Their
	// package-level vars (listCmd, showCmd, approveCmd, rejectCmd,
	// logsCmd) are defined in their own files; we keep the topology
	// change in one place here.
	incidentsCmd.AddCommand(listCmd, showCmd, approveCmd, rejectCmd, logsCmd)
}
