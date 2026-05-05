package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
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

	// TODO: call /api/v1/auth/login when wired. For now, persist the
	// raw values so the rest of the flow can be exercised against a
	// dev backend; the loginer hits the real endpoint as soon as
	// auth lands in the agent.
	if err := config.Save(&config.File{
		ServerURL: url,
		AuthToken: string(pwd), // placeholder — swap for the returned JWT
		Username:  user,
	}); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}
	fmt.Fprintf(c.OutOrStdout(), "saved session for %s\n", user)
	return nil
}
