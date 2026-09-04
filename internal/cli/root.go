package cli

import (
	"github.com/spf13/cobra"
)

// NewRootCmd creates the root Cobra command for the Amora CLI.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "amora",
		Short: "🍇 Amora — Micro-PaaS for Raspberry Pi",
		Long: `Amora is a tiny PaaS that turns your Raspberry Pi into a
Heroku-like deployment target. Deploy apps with git push,
get automatic mDNS discovery, and zero-config reverse proxy.`,
	}

	cmd.AddCommand(
		NewSetupCmd(),
		NewCreateCmd(),
		NewDestroyCmd(),
		NewHookCmd(),
		NewEnvCmd(),
		NewLogsCmd(),
		NewProvisionCmd(),
	)

	return cmd
}
