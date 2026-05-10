package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/srepulse/cli/internal/client"
	"github.com/srepulse/cli/internal/config"
	"golang.org/x/term"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate against a srepulse instance",
	Long: `Prompt for credentials and persist a session token to
~/.config/srepulse/auth.json (mode 0600). Token is sent as a Bearer
header on subsequent requests.

Today this is local-password-only; OIDC flows will land alongside
SSO support.`,
	RunE: runLogin,
}

func runLogin(c *cobra.Command, _ []string) error {
	url, _ := resolveServer()
	fmt.Fprintf(c.OutOrStdout(), "logging in to %s\n", url)

	in := bufio.NewReader(os.Stdin)
	fmt.Fprint(c.OutOrStdout(), "username: ")
	user, _ := in.ReadString('\n')
	user = strings.TrimSpace(user)

	fmt.Fprint(c.OutOrStdout(), "password: ")
	pwd, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(c.OutOrStdout())
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}

	cli := client.New(url, flagInsecure)
	res, err := cli.Login(c.Context(), user, string(pwd))
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	cfg.ServerURL = url
	cfg.AuthToken = res.Token
	cfg.Username = res.User.Email
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}
	fmt.Fprintf(c.OutOrStdout(), "saved session for %s\n", res.User.Email)
	return nil
}
