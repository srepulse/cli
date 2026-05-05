// Package cmd wires the cobra command tree for kubectl-srepulse.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/srepulse/cli/internal/config"
)

// Global flags. Each subcommand reads from this when it builds the
// REST client — server URL precedence: --server > $SREPULSE_URL >
// config file > default localhost.
var (
	flagServer   string
	flagInsecure bool

	versionInfo = struct{ version, commit, date string }{
		version: "dev",
		commit:  "none",
		date:    "unknown",
	}
)

// SetVersionInfo is called from main() to wire ldflags-injected
// values before Execute(). Keeping these as a small struct lets the
// version subcommand render them without main importing internals.
func SetVersionInfo(v, c, d string) {
	versionInfo.version = v
	versionInfo.commit = c
	versionInfo.date = d
}

var rootCmd = &cobra.Command{
	Use:   "kubectl-srepulse",
	Short: "Terminal client for srepulse, the autonomous Kubernetes SRE",
	Long: `kubectl-srepulse is the operator's terminal-side companion for srepulse.

Run with no arguments to launch the interactive TUI. Use the subcommands
for one-shot actions you can pipe, script, or paste into Slack:

  kubectl srepulse list                    # incidents in the active cluster
  kubectl srepulse show <id>               # one incident, no TTY needed
  kubectl srepulse approve <id> --reason "" # apply the proposed remediation
  kubectl srepulse reject  <id> --reason "" # reject; routes to manual playbook
  kubectl srepulse logs    <id> -f         # stream the agent's reasoning

Configuration lives at ~/.config/srepulse/ — server URL, auth token,
client preferences. See "kubectl srepulse config --help" for the
controls.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	// When invoked with no args we delegate to the TUI command. Cobra's
	// RunE on the root keeps us from printing the long help every time
	// someone just types `kubectl srepulse`.
	RunE: func(c *cobra.Command, args []string) error {
		return tuiCmd.RunE(c, args)
	},
}

// Execute dispatches to the matched subcommand. main() returns the
// error so we can keep error-printing in one place.
func Execute() error {
	rootCmd.PersistentFlags().StringVar(
		&flagServer, "server", "",
		"srepulse API URL (default $SREPULSE_URL or ~/.config/srepulse/server.json)",
	)
	rootCmd.PersistentFlags().BoolVar(
		&flagInsecure, "insecure-skip-tls-verify", false,
		"skip TLS verification (dev only)",
	)
	// All incident-scoped subcommands live under `incidents` (with
	// "i" / "inc" aliases for terseness). Fingerprint browse lives
	// under `fingerprints` ("fp"). Top-level slots are reserved for
	// noun-parents and the unique tools (tui / login / config /
	// version).
	rootCmd.AddCommand(versionCmd, incidentsCmd, fingerprintsCmd, loginCmd, tuiCmd, configCmd)
	// Attach our `install` helper + richer help to Cobra's auto-built
	// `completion` command. Must run AFTER all subcommands are
	// registered so InitDefaultCompletionCmd has the full tree to
	// generate scripts for.
	extendCompletionCmd()
	return rootCmd.Execute()
}

// resolveServer picks the API URL using the documented precedence:
// flag > env > config > default. Returns the URL and a redacted
// "where it came from" string for diagnostic messages.
func resolveServer() (string, string) {
	if flagServer != "" {
		return flagServer, "--server flag"
	}
	if env := os.Getenv("SREPULSE_URL"); env != "" {
		return env, "$SREPULSE_URL"
	}
	if cfg, err := config.Load(); err == nil && cfg.ServerURL != "" {
		return cfg.ServerURL, "config file"
	}
	return "http://localhost:8080", "default"
}

// dieIf is shared error-rendering for one-shot subcommands. Keeps
// the top-level red banner consistent across `list`, `show`, etc.
func dieIf(err error, format string, a ...any) {
	if err == nil {
		return
	}
	prefix := fmt.Sprintf(format, a...)
	fmt.Fprintf(os.Stderr, "%s: %v\n", prefix, err)
	os.Exit(1)
}
