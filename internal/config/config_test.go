package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SREPULSE_CONFIG_DIR", dir)

	in := &File{
		ServerURL: "https://srepulse.example.com",
		AuthToken: "secret-token-123",
		Username:  "alice",
	}
	if err := Save(in); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if *out != *in {
		t.Errorf("round-trip mismatch:\n got: %+v\nwant: %+v", out, in)
	}
}

func TestLoad_MissingFile_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SREPULSE_CONFIG_DIR", dir)

	got, err := Load()
	if err != nil {
		t.Fatalf("missing file should not error, got %v", err)
	}
	if got == nil {
		t.Fatal("expected zero-value File, got nil")
	}
	if *got != (File{}) {
		t.Errorf("expected zero-value, got %+v", got)
	}
}

func TestLoad_BadJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SREPULSE_CONFIG_DIR", dir)

	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Error("bad JSON should error so the operator notices a corrupt file")
	}
}

func TestSave_FileMode_IsRestrictive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes don't translate cleanly on windows")
	}
	dir := t.TempDir()
	t.Setenv("SREPULSE_CONFIG_DIR", dir)
	if err := Save(&File{AuthToken: "t"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("file mode = %#o, want 0600 (the file holds an auth token)", got)
	}
}

func TestSave_AtomicViaRename(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SREPULSE_CONFIG_DIR", dir)
	if err := Save(&File{ServerURL: "v1"}); err != nil {
		t.Fatal(err)
	}
	if err := Save(&File{ServerURL: "v2"}); err != nil {
		t.Fatal(err)
	}
	// The temp file shouldn't be left behind after a successful save.
	if _, err := os.Stat(filepath.Join(dir, "config.json.tmp")); !os.IsNotExist(err) {
		t.Errorf("config.json.tmp should not exist after save, got err=%v", err)
	}
	got, _ := Load()
	if got.ServerURL != "v2" {
		t.Errorf("second save didn't replace, got %q", got.ServerURL)
	}
}

func TestDir_HonoursEnv(t *testing.T) {
	t.Setenv("SREPULSE_CONFIG_DIR", "/some/where/explicit")
	got, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/some/where/explicit" {
		t.Errorf("Dir() = %q, want /some/where/explicit", got)
	}
}
