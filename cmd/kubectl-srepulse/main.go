// Command kubectl-srepulse is the srepulse operator's terminal client.
// Bare invocation launches the interactive TUI; subcommands cover the
// one-shot CLI surface (list, show, approve, reject, login, etc.).
//
// The binary is named kubectl-srepulse so kubectl plugin dispatch
// resolves "kubectl srepulse <args>" to it; it also runs standalone
// for users who prefer not to type the kubectl prefix.
package main

import (
	"fmt"
	"os"

	"github.com/srepulse/cli/cmd/kubectl-srepulse/cmd"
)

// version is set at build time via -ldflags. Leave as "dev" for local
// builds so `kubectl srepulse version` reports something useful.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.SetVersionInfo(version, commit, date)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
