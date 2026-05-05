// Package main_test exercises the built binary end-to-end. We compile
// once into the test's TempDir, then invoke it for each subtest. This
// catches the wiring failures that unit tests can miss — a broken
// cobra command graph, stdout/stderr crossed wires, the version flag
// gone, etc.
package main_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildBinary compiles the CLI into the test's tempdir and returns
// its path. Each test calls this once; build is fast enough (~1 s)
// that we don't bother sharing across subtests.
func buildBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "kubectl-srepulse")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-trimpath",
		"-ldflags", "-s -w -X main.version=test -X main.commit=testcommit -X main.date=2026-05-05T00:00:00Z",
		"-o", bin, ".")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return bin
}

// run executes the binary with the given args + a clean env (PATH
// only, plus whatever the caller adds). Returns stdout, stderr, and
// the exit code (0 when the command succeeded).
func run(t *testing.T, bin string, env []string, args ...string) (stdout, stderr string, exit int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}, env...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return out.String(), errOut.String(), ee.ExitCode()
		}
		t.Fatalf("exec %s %v: %v", bin, args, err)
	}
	return out.String(), errOut.String(), 0
}

func TestVersion_PrintsBuildInfo(t *testing.T) {
	bin := buildBinary(t)
	stdout, _, code := run(t, bin, nil, "version")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	for _, want := range []string{"kubectl-srepulse", "test", "commit testcommit"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output missing %q: got %q", want, stdout)
		}
	}
}

func TestHelp_ListsExpectedSubcommands(t *testing.T) {
	bin := buildBinary(t)
	stdout, _, _ := run(t, bin, nil, "--help")
	// All incident-scoped commands live under `incidents`. Top-level
	// is just noun-parents (incidents / fingerprints) + the unique
	// tools (tui / login / config / version / completion).
	expected := []string{"incidents", "fingerprints", "login", "tui", "config", "version", "completion"}
	for _, sub := range expected {
		if !strings.Contains(stdout, sub) {
			t.Errorf("--help missing subcommand %q", sub)
		}
	}
}

func TestIncidentsHelp_ListsAllVerbs(t *testing.T) {
	bin := buildBinary(t)
	stdout, _, _ := run(t, bin, nil, "incidents", "--help")
	for _, sub := range []string{"list", "show", "approve", "reject", "logs"} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("`incidents --help` missing %q: got %q", sub, stdout)
		}
	}
}

func TestFingerprintsHelp_ListsListAndShow(t *testing.T) {
	bin := buildBinary(t)
	stdout, _, _ := run(t, bin, nil, "fingerprints", "--help")
	for _, sub := range []string{"list", "show"} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("`fingerprints --help` missing %q: got %q", sub, stdout)
		}
	}
}

func TestList_FailsClearWhenServerUnreachable(t *testing.T) {
	bin := buildBinary(t)
	// `list` lives under `incidents` now; the alias `i` is the
	// short form. Point at a port nothing's bound to so the
	// connection refuses fast.
	_, stderr, code := run(t, bin, nil, "--server", "http://127.0.0.1:1", "incidents", "list")
	if code == 0 {
		t.Error("expected non-zero exit when the server is unreachable")
	}
	if !strings.Contains(stderr, "list incidents") || !strings.Contains(stderr, "127.0.0.1:1") {
		t.Errorf("error should name what failed and where, got: %q", stderr)
	}
}

func TestIncidentsAlias_iWorks(t *testing.T) {
	bin := buildBinary(t)
	// `i` is an alias on the `incidents` parent — the short form
	// most operators will type. Verify it dispatches to the same
	// list runner.
	_, stderr, code := run(t, bin, nil, "--server", "http://127.0.0.1:1", "i", "list")
	if code == 0 {
		t.Error("expected non-zero exit (server unreachable)")
	}
	if !strings.Contains(stderr, "list incidents") {
		t.Errorf("`i list` should hit the same runner as `incidents list`, got: %q", stderr)
	}
}

func TestFingerprintsAlias_fpWorks(t *testing.T) {
	bin := buildBinary(t)
	_, stderr, code := run(t, bin, nil, "--server", "http://127.0.0.1:1", "fp", "list")
	if code == 0 {
		t.Error("expected non-zero exit (server unreachable)")
	}
	if !strings.Contains(stderr, "list fingerprints") {
		t.Errorf("`fp list` should error from the fingerprints runner, got: %q", stderr)
	}
}

func TestRejectMissingReasonFlag(t *testing.T) {
	bin := buildBinary(t)
	// reject lives under `incidents` now.
	_, stderr, code := run(t, bin, nil, "incidents", "reject", "abc")
	if code == 0 {
		t.Error("reject without --reason should fail (the flag is marked required)")
	}
	if !strings.Contains(stderr, "reason") {
		t.Errorf("error should mention the missing reason flag, got: %q", stderr)
	}
}

func TestConfig_ShowEmptyByDefault(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	stdout, _, code := run(t, bin, []string{"SREPULSE_CONFIG_DIR=" + dir}, "config", "show")
	if code != 0 {
		t.Fatalf("config show: exit %d", code)
	}
	if !strings.Contains(stdout, "{") {
		t.Errorf("expected JSON output, got %q", stdout)
	}
}
