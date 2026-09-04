package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

// NewSetupCmd creates the `amora setup` command.
//
// This command initializes the Amora directory structure and enables
// systemd lingering so user services persist without an active login session.
func NewSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Initialize the Amora environment on this machine",
		Long: `Sets up the required directory structure under the current user's
home directory and enables systemd lingering for persistent services.

Run this once on a fresh Raspberry Pi after creating the 'amora' user.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			Banner()
			LogInfo("Setting up Amora environment...")

			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("getting home directory: %w", err)
			}

			// Create all required directories.
			dirs := []string{
				filepath.Join(home, "apps"),
				filepath.Join(home, "repos"),
				filepath.Join(home, ".amora", "envs"),
				filepath.Join(home, "caddy"),
				filepath.Join(home, ".config", "systemd", "user"),
			}

			for _, dir := range dirs {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("creating directory %s: %w", dir, err)
				}
				LogSuccess("Created %s", dir)
			}

			// Enable lingering so user services survive logout.
			// This typically requires the current user to have permissions,
			// or the admin to have run `loginctl enable-linger amora` beforehand.
			user := os.Getenv("USER")
			if user == "" {
				user = os.Getenv("LOGNAME")
			}

			if err := exec.Command("loginctl", "enable-linger", user).Run(); err != nil {
				LogError("Could not enable linger for user '%s': %v", user, err)
				fmt.Printf("       Run manually: sudo loginctl enable-linger %s\n", user)
			} else {
				LogSuccess("Enabled systemd linger for user '%s'", user)
			}

			fmt.Println()
			LogInfo("Setup complete! You can now create apps with:")
			fmt.Println("       amora create --app <name>")
			fmt.Println()

			return nil
		},
	}
}
