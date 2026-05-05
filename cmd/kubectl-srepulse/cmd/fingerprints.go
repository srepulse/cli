package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/srepulse/cli/internal/client"
	"github.com/srepulse/cli/internal/render"
)

// fingerprintsCmd is a parent that hosts list/show under "kubectl
// srepulse fingerprints …". Aliased to "fp" for terseness — typing
// the full word every time gets old when investigating in tmux.
var fingerprintsCmd = &cobra.Command{
	Use:     "fingerprints",
	Aliases: []string{"fp"},
	Short:   "Browse the fingerprint catalog",
	Long: `Fingerprints are the agent's recognisers — small rules that match
specific failure shapes (OOMKilled bursts, ImagePullBackOff, FailedScheduling,
…) and ship a tested remediation. High-confidence matches skip the LLM
entirely.

  list    show the catalog with calibration confidence per row
  show    a single fingerprint's definition + status

Aliased "fp" for short.`,
}

func init() {
	fingerprintsCmd.AddCommand(fpListCmd, fpShowCmd)
}

// ── fingerprints list ────────────────────────────────────────────

var (
	flagFpListJSON     bool
	flagFpListCategory string
	flagFpListSource   string
	flagFpListLearned  bool
)

var fpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List fingerprints",
	Long: `List the fingerprint catalog. Without flags, prints a tabular view
sorted by category. -j emits the raw catalog object (incl. category counts
+ pending-refinement map) for jq pipelines.`,
	Example: `  kubectl srepulse fingerprints list
  kubectl srepulse fp list --category crash
  kubectl srepulse fp list --source prometheus --learned
  kubectl srepulse fp list -j | jq '.categories'`,
	RunE: runFpList,
}

func init() {
	fpListCmd.Flags().BoolVarP(&flagFpListJSON, "output-json", "j", false, "emit raw catalog JSON")
	fpListCmd.Flags().StringVar(&flagFpListCategory, "category", "", "filter by category (crash / image / scheduling / observability / learned / …)")
	fpListCmd.Flags().StringVar(&flagFpListSource, "source", "", "filter by source (k8s / prometheus / alertmanager / scanner / argocd / pagerduty)")
	fpListCmd.Flags().BoolVar(&flagFpListLearned, "learned", false, "show only learned (drafted from approved remediations) fingerprints")
}

func runFpList(c *cobra.Command, _ []string) error {
	url, src := resolveServer()
	cli := client.New(url, flagInsecure)
	ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
	defer cancel()

	cat, err := cli.ListFingerprints(ctx)
	if err != nil {
		return fmt.Errorf("list fingerprints (server: %s, source: %s): %w", url, src, err)
	}
	out := c.OutOrStdout()

	if flagFpListJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(cat)
	}

	// Filter pass: category / source / learned-only.
	items := cat.Items
	if flagFpListCategory != "" {
		want := strings.ToLower(flagFpListCategory)
		filtered := items[:0]
		for _, fp := range items {
			if strings.ToLower(fp.Category) == want {
				filtered = append(filtered, fp)
			}
		}
		items = filtered
	}
	if flagFpListSource != "" {
		want := strings.ToLower(flagFpListSource)
		filtered := items[:0]
		for _, fp := range items {
			if strings.ToLower(fp.Source) == want {
				filtered = append(filtered, fp)
			}
		}
		items = filtered
	}
	if flagFpListLearned {
		filtered := items[:0]
		for _, fp := range items {
			// Learned fingerprints either have category=learned or are
			// non-builtin — both are produced by the LLM-remediation
			// capture path, so we accept either signal.
			if strings.EqualFold(fp.Category, "learned") || !fp.Builtin {
				filtered = append(filtered, fp)
			}
		}
		items = filtered
	}

	// Stable sort: category asc, then id asc — keeps related rows
	// together (all "kubernetes" fingerprints adjacent, etc.) which
	// matches the design's CLI mock pattern.
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Category != items[j].Category {
			return items[i].Category < items[j].Category
		}
		return items[i].ID < items[j].ID
	})

	// Headline strip — total count + per-category breakdown — only
	// when no filter is active (otherwise the numbers are misleading).
	if flagFpListCategory == "" && flagFpListSource == "" && !flagFpListLearned {
		printHeadline(out, cat)
		fmt.Fprintln(out)
	}

	if len(items) == 0 {
		fmt.Fprintln(out, render.Dim(out, "no fingerprints match"))
		return nil
	}

	headers := []string{"ID", "CATEGORY", "SOURCE", "RISK", "AUTO", "STATUS", "CONF", "TITLE"}
	rows := [][]string{}
	for _, fp := range items {
		rows = append(rows, []string{
			render.Pulse(out, fp.ID),
			render.Dim(out, fp.Category),
			render.Dim(out, fp.Source),
			render.Severity(out, fp.RiskLevel),
			autoChip(out, fp.AutoPatch),
			statusChip(out, fp.Status),
			fp.Confidence,
			truncate(fp.Title, 50),
		})
	}
	fmt.Fprintln(out, formatTable(out, headers, rows))
	return nil
}

// printHeadline renders the count + per-category counts as a quiet
// dim row so the operator gets a sense of scale before scanning the
// table. Mirrors the marketing/web view's stat band.
func printHeadline(out interface{ Write([]byte) (int, error) }, cat *client.FingerprintCatalog) {
	keys := make([]string, 0, len(cat.Categories))
	for k := range cat.Categories {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts,
			render.Pulse(out, fmt.Sprint(cat.Categories[k]))+" "+render.Dim(out, k))
	}
	fmt.Fprintf(out, "%s %s   %s\n",
		render.Pulse(out, fmt.Sprint(cat.Total)),
		render.Dim(out, "fingerprints"),
		strings.Join(parts, "  "+render.Dim(out, "·")+"  "),
	)
}

func autoChip(out interface{ Write([]byte) (int, error) }, on bool) string {
	if on {
		return render.OK(out, "yes")
	}
	return render.Dim(out, "no")
}

func statusChip(out interface{ Write([]byte) (int, error) }, st string) string {
	if st == "" {
		return render.Dim(out, "-")
	}
	switch strings.ToLower(st) {
	case "stable":
		return render.OK(out, st)
	case "experimental":
		return render.Warn(out, st)
	case "draft":
		return render.Dim(out, st)
	default:
		return st
	}
}

// ── fingerprints show ────────────────────────────────────────────

var flagFpShowJSON bool

var fpShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a single fingerprint",
	Args:  cobra.ExactArgs(1),
	Long: `Show one fingerprint's full definition: title, description, fix,
calibration confidence, triggers, and the YAML body.

The id is the kebab-case slug — the leftmost column in the list table.`,
	Example: `  kubectl srepulse fingerprints show crash-loop-back-off
  kubectl srepulse fp show oomkilled-restart-spike -j | yq .definition`,
	ValidArgsFunction: completeFingerprintIDs,
	RunE:              runFpShow,
}

func init() {
	fpShowCmd.Flags().BoolVarP(&flagFpShowJSON, "output-json", "j", false, "emit raw fingerprint JSON")
}

func runFpShow(c *cobra.Command, args []string) error {
	id := args[0]
	url, src := resolveServer()
	cli := client.New(url, flagInsecure)
	ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
	defer cancel()

	fp, err := cli.GetFingerprint(ctx, id)
	if err != nil {
		return fmt.Errorf("show fingerprint %s (server: %s, source: %s): %w", id, url, src, err)
	}

	out := c.OutOrStdout()

	if flagFpShowJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(fp)
	}

	// Header — same cadence as `incidents show`: pulse identifier,
	// dim middle dot, status chip on the right.
	fmt.Fprintf(out, "%s %s %s\n",
		render.Pulse(out, fp.ID),
		render.Dim(out, "·"),
		statusChip(out, fp.Status),
	)
	field := func(label, value string) {
		fmt.Fprintf(out, "  %s %s\n",
			render.Dim(out, fmt.Sprintf("%-12s", label)),
			value,
		)
	}
	field("name:", fp.Name)
	field("title:", fp.Title)
	field("category:", fp.Category)
	field("source:", fp.Source)
	field("risk:", render.Severity(out, fp.RiskLevel))
	field("auto-patch:", autoChip(out, fp.AutoPatch))
	field("confidence:", render.Pulse(out, fp.Confidence))
	field("builtin:", boolStr(fp.Builtin))
	if fp.UpdatedAt != "" {
		field("updated:", render.Dim(out, fp.UpdatedAt))
	}

	if fp.Description != "" {
		fmt.Fprintf(out, "\n%s\n%s\n",
			render.Dim(out, "description:"),
			indent(fp.Description, "  "))
	}
	if fp.Fix != "" {
		fmt.Fprintf(out, "\n%s\n%s\n",
			render.Dim(out, "fix:"),
			indent(fp.Fix, "  "))
	}
	if len(fp.Triggers) > 0 {
		fmt.Fprintf(out, "\n%s\n", render.Dim(out, "triggers:"))
		for _, t := range fp.Triggers {
			fmt.Fprintf(out, "  %s %s\n", render.Dim(out, "·"), t)
		}
	}
	if fp.Definition != "" {
		fmt.Fprintf(out, "\n%s\n", render.Dim(out, "definition:"))
		// Render the YAML body inside a panel-bordered box per the
		// design (.box style — hairline border in fg-3-ish). Lipgloss
		// gives us the box trivially.
		box := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#262f38")).
			Padding(0, 1).
			Render(strings.TrimRight(fp.Definition, "\n"))
		fmt.Fprintln(out, box)
	}
	return nil
}

func boolStr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}
