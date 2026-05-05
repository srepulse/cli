package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/srepulse/cli/internal/client"
)

// getenvFromOS is the real os.Getenv. The package-level `getenv` var
// at the bottom of this file points at this; tests can swap it.
func getenvFromOS(k string) string { return os.Getenv(k) }

// Cobra installs a default `completion` command automatically — the
// one that prints bash/zsh/fish/powershell scripts. This file extends
// that with three small things:
//
//  1. A friendlier long help that includes copy-pasteable per-shell
//     install instructions (Cobra's default explains which command
//     to run but not where to write the output).
//  2. Dynamic ValidArgsFunctions for the subcommands that take an
//     incident or fingerprint id, so `kubectl srepulse show <TAB>`
//     suggests live IDs from the configured backend.
//  3. A short timeout on the backend probe so completion never
//     hangs the user's shell when the server is slow / down.

// completionCustomHelp replaces Cobra's auto-generated completion-
// command help. We keep the same hidden install sub-tree (bash/zsh/
// fish/powershell) — Cobra's defaults are fine — but add per-shell
// install snippets to the long description.
func init() {
	cobra.OnInitialize() // touch the package so init order is stable
}

// ── Dynamic argument completion ──────────────────────────────────

// completeIncidentIDs suggests live incident IDs against the backend
// when the user tabs after `show`, `approve`, `reject`, or `logs`.
// Returns IDs in Cobra's "value\tdescription" format so shells that
// support per-suggestion descriptions (zsh, fish) show the reason
// next to the id. Bash drops the description.
func completeIncidentIDs(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Each id is positional + only one. After we have one, return
	// nothing so the shell falls back to filename completion off.
	if len(args) >= 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	url, _ := resolveServer()
	cli := client.New(url, flagInsecure)
	// Tight timeout — completion runs as a synchronous subprocess
	// of the user's shell. If the backend is slow we'd rather emit
	// nothing than hang the prompt.
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	incs, err := cli.ListIncidents(ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError | cobra.ShellCompDirectiveNoFileComp
	}

	out := make([]string, 0, len(incs))
	for _, i := range incs {
		if !strings.HasPrefix(i.ID, toComplete) {
			continue
		}
		desc := strings.TrimSpace(i.TriggerEvent.Reason)
		if desc == "" {
			desc = i.Status
		}
		// "id\tdescription" — Cobra recognises this and surfaces the
		// description in shells that support it.
		out = append(out, i.ID+"\t"+desc)
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeFingerprintIDs is the same shape for fingerprint slugs.
// Cached briefly per process so a tab-bursty session doesn't issue
// N requests; the cache TTL is intentionally short (10 s) so newly
// promoted learned fingerprints surface quickly.
var (
	fpCacheItems   []client.Fingerprint
	fpCacheUpdated time.Time
)

const fpCompletionCacheTTL = 10 * time.Second

func completeFingerprintIDs(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) >= 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	if time.Since(fpCacheUpdated) > fpCompletionCacheTTL {
		url, _ := resolveServer()
		cli := client.New(url, flagInsecure)
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		defer cancel()
		cat, err := cli.ListFingerprints(ctx)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError | cobra.ShellCompDirectiveNoFileComp
		}
		fpCacheItems = cat.Items
		fpCacheUpdated = time.Now()
	}

	out := make([]string, 0, len(fpCacheItems))
	for _, fp := range fpCacheItems {
		if !strings.HasPrefix(fp.ID, toComplete) {
			continue
		}
		desc := fp.Title
		if desc == "" {
			desc = fp.Category
		}
		out = append(out, fp.ID+"\t"+desc)
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// extendCompletionCmd attaches our `install` helper to Cobra's auto-
// generated `completion` subcommand and patches its long help text
// with per-shell install snippets. Called from Execute() before
// rootCmd.Execute() so the registration is in place before Cobra
// dispatches.
func extendCompletionCmd() {
	// Force Cobra to materialise its default completion command. It
	// creates this lazily on first execution; we want it earlier so
	// we can hang an install subcommand off it.
	rootCmd.InitDefaultCompletionCmd()

	comp, _, err := rootCmd.Find([]string{"completion"})
	if err != nil || comp == nil {
		return
	}
	for _, s := range comp.Commands() {
		if s.Name() == "install" {
			return // already added (Execute called twice in tests)
		}
	}
	comp.AddCommand(completionInstallCmd)
	comp.Long = completionLongHelp
}

const completionLongHelp = `Generate shell completion scripts for kubectl-srepulse.

The CLI supports tab-completion of subcommands, flags, AND live values:
incident IDs in show/approve/reject/logs and fingerprint slugs in
fingerprints show all auto-suggest from the configured backend.

Pick the snippet for your shell and source it once in your shell
profile (or run "kubectl srepulse completion install" for the same
hint with paths checked):

  # bash (linux)
  kubectl srepulse completion bash > /etc/bash_completion.d/kubectl-srepulse

  # bash (macOS, with bash-completion@2 from brew)
  kubectl srepulse completion bash > $(brew --prefix)/etc/bash_completion.d/kubectl-srepulse

  # zsh
  kubectl srepulse completion zsh > "${fpath[1]}/_kubectl-srepulse"

  # fish
  kubectl srepulse completion fish > ~/.config/fish/completions/kubectl-srepulse.fish

  # powershell
  kubectl srepulse completion powershell | Out-String | Invoke-Expression`

var completionInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Print shell-specific install instructions",
	Long: `Print install instructions for the configured shell.

Detects $SHELL and prints the right one-liner; pass an explicit shell
name (bash/zsh/fish/powershell) to override.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		shell := ""
		if len(args) == 1 {
			shell = args[0]
		} else {
			// $SHELL is reliable on unix, less so on Windows.
			env := splitShell(getenv("SHELL"))
			shell = env
		}
		switch strings.ToLower(shell) {
		case "bash":
			fmt.Fprintln(c.OutOrStdout(),
				"# bash — add to ~/.bashrc (linux) or sourced once (macos):\n"+
					"  source <(kubectl-srepulse completion bash)\n"+
					"# or, persistently:\n"+
					"  kubectl-srepulse completion bash > /etc/bash_completion.d/kubectl-srepulse")
		case "zsh":
			fmt.Fprintln(c.OutOrStdout(),
				"# zsh — add to ~/.zshrc:\n"+
					"  autoload -U compinit && compinit\n"+
					"  source <(kubectl-srepulse completion zsh)\n"+
					"# or, persistently:\n"+
					"  kubectl-srepulse completion zsh > \"${fpath[1]}/_kubectl-srepulse\"")
		case "fish":
			fmt.Fprintln(c.OutOrStdout(),
				"# fish — install once:\n"+
					"  kubectl-srepulse completion fish > ~/.config/fish/completions/kubectl-srepulse.fish")
		case "powershell", "pwsh":
			fmt.Fprintln(c.OutOrStdout(),
				"# powershell — add to $PROFILE:\n"+
					"  kubectl-srepulse completion powershell | Out-String | Invoke-Expression")
		default:
			return fmt.Errorf("unknown shell %q (try: bash | zsh | fish | powershell)", shell)
		}
		return nil
	},
}

// splitShell extracts the basename of $SHELL ("/bin/zsh" → "zsh").
func splitShell(s string) string {
	if s == "" {
		return ""
	}
	if i := strings.LastIndex(s, "/"); i >= 0 && i+1 < len(s) {
		return s[i+1:]
	}
	return s
}

// getenv is a tiny indirection so tests can stub.
var getenv = func(k string) string {
	return getenvFromOS(k)
}
