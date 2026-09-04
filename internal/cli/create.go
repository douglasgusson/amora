package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/douglasgusson/amora/internal/deploy"
	"github.com/douglasgusson/amora/internal/env"
	"github.com/spf13/cobra"
)

// NewCreateCmd creates the `amora create --app <name>` command.
//
// This command initializes a new application by:
//  1. Creating a bare git repository at ~/repos/<app>.git
//  2. Installing a post-receive hook that triggers the deploy pipeline
//  3. Creating the app working directory at ~/apps/<app>
//  4. Assigning a unique PORT in the app's .env file
func NewCreateCmd() *cobra.Command {
	var appName string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new application",
		Long:  "Creates a bare git repository, installs the deploy hook, and assigns a port.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if appName == "" {
				return fmt.Errorf("--app flag is required")
			}

			Banner()
			LogInfo("Creating app '%s'...", appName)

			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("getting home directory: %w", err)
			}

			// 1. Create and initialize bare git repository.
			repoPath := filepath.Join(home, "repos", appName+".git")
			if err := os.MkdirAll(repoPath, 0755); err != nil {
				return fmt.Errorf("creating repo directory: %w", err)
			}

			if err := deploy.StreamCommand("git", "init", "--bare", repoPath); err != nil {
				return fmt.Errorf("initializing bare repo: %w", err)
			}
			LogSuccess("Bare repository created at %s", repoPath)

			// 2. Install post-receive hook.
			hookPath := filepath.Join(repoPath, "hooks", "post-receive")

			// Resolve the absolute path of the amora binary for the hook script.
			amoraBin, err := os.Executable()
			if err != nil {
				amoraBin = "amora" // Fallback to PATH lookup.
			}

			hookContent := fmt.Sprintf(`#!/bin/bash
# Amora post-receive hook — triggers deploy pipeline
%s hook post-receive --app %s
`, amoraBin, appName)

			if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
				return fmt.Errorf("writing post-receive hook: %w", err)
			}
			LogSuccess("Post-receive hook installed")

			// 3. Create app working directory.
			appDir := filepath.Join(home, "apps", appName)
			if err := os.MkdirAll(appDir, 0755); err != nil {
				return fmt.Errorf("creating app directory: %w", err)
			}
			LogSuccess("App directory created at %s", appDir)

			// 4. Assign a unique port.
			port, err := env.GetOrAssignPort(appName)
			if err != nil {
				return fmt.Errorf("assigning port: %w", err)
			}
			LogSuccess("Assigned PORT=%d", port)

			// Print instructions for the developer.
			fmt.Println()
			LogInfo("App '%s' created successfully!", appName)
			fmt.Println()
			fmt.Println("  Add the git remote on your development machine:")
			fmt.Println()
			fmt.Printf("    git remote add amora amora@<raspberry-pi>:repos/%s.git\n", appName)
			fmt.Println("    git push amora main")
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().StringVar(&appName, "app", "", "Application name (required)")
	_ = cmd.MarkFlagRequired("app")

	return cmd
}

