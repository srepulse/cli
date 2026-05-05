package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/srepulse/cli/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Inspect / edit the local config",
}

func init() {
	configCmd.AddCommand(
		&cobra.Command{
			Use:   "show",
			Short: "Print the resolved config",
			RunE: func(c *cobra.Command, _ []string) error {
				cfg, err := config.Load()
				if err != nil {
					return err
				}
				// Auth token is sensitive — redact in printed output but
				// indicate whether it's present so the user can tell why
				// auth might be failing.
				view := *cfg
				if view.AuthToken != "" {
					view.AuthToken = "[redacted, " + fmt.Sprint(len(cfg.AuthToken)) + " bytes]"
				}
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(view)
			},
		},
		&cobra.Command{
			Use:   "set-server <url>",
			Short: "Persist the API URL",
			Args:  cobra.ExactArgs(1),
			RunE: func(c *cobra.Command, args []string) error {
				cfg, _ := config.Load()
				if cfg == nil {
					cfg = &config.File{}
				}
				cfg.ServerURL = args[0]
				return config.Save(cfg)
			},
		},
		&cobra.Command{
			Use:   "logout",
			Short: "Clear the cached auth token",
			RunE: func(c *cobra.Command, _ []string) error {
				cfg, err := config.Load()
				if err != nil {
					return err
				}
				cfg.AuthToken = ""
				cfg.Username = ""
				return config.Save(cfg)
			},
		},
	)
}
