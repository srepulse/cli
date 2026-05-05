// Package config persists the CLI's runtime preferences — server URL,
// auth token, current user — under ~/.config/srepulse/config.json.
//
// Honours $SREPULSE_CONFIG_DIR for users who want to override (XDG-style
// or per-cluster profiles). File is mode 0600 since it carries the
// auth token.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// File is the on-disk shape. New fields land at the bottom; old
// agents tolerate unknown keys via json.Decoder.DisallowUnknownFields
// being off by default.
type File struct {
	ServerURL string `json:"serverUrl,omitempty"`
	AuthToken string `json:"authToken,omitempty"`
	Username  string `json:"username,omitempty"`
}

// Dir returns the config directory, respecting $SREPULSE_CONFIG_DIR
// before falling back to ~/.config/srepulse.
func Dir() (string, error) {
	if d := os.Getenv("SREPULSE_CONFIG_DIR"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home dir: %w", err)
	}
	return filepath.Join(home, ".config", "srepulse"), nil
}

func path() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.json"), nil
}

// Load returns the persisted config. A missing file is not an error
// — callers get a zero-value File. Bad JSON is an error so the
// operator notices a corrupt file before it silently overwrites.
func Load() (*File, error) {
	p, err := path()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &File{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", p, err)
	}
	var f File
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", p, err)
	}
	return &f, nil
}

// Save writes the file at mode 0600. Creates parent directories with
// mode 0700. Atomic-via-rename so a crashed process leaves the prior
// config intact.
func Save(f *File) error {
	d, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", d, err)
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(d, "config.json.tmp")
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	final := filepath.Join(d, "config.json")
	if err := os.Rename(tmp, final); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
