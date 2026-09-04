package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/douglasgusson/amora/internal/deploy"
	"github.com/douglasgusson/amora/internal/env"
	"github.com/douglasgusson/amora/internal/mdns"
	"github.com/douglasgusson/amora/internal/proxy"
	"github.com/douglasgusson/amora/internal/systemd"
	"github.com/spf13/cobra"
)

// NewHookCmd creates the `amora hook` command group.
func NewHookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Git hook handlers (used internally)",
	}

	var appName string
	postReceive := &cobra.Command{
		Use:   "post-receive",
		Short: "Handle the git post-receive hook and trigger a deploy",
		Long: `This command is called automatically by git after a push to
the bare repository. It orchestrates the full deploy pipeline:
checkout, Procfile parsing, systemd service generation, Caddy
configuration, and mDNS advertisement.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Production: use real implementations.
			pipeline := &DeployPipeline{
				Runner:   &deploy.RealRunner{},
				EnvMgr:   env.NewManager(env.DefaultDir()),
				Systemd:  systemd.NewGenerator(systemd.DefaultDir()),
				CaddyDir: proxy.CaddyDir,
				HomeDir:  homeDir(),
				Stdin:    os.Stdin,
			}
			return pipeline.Run(appName)
		},
	}

	postReceive.Flags().StringVar(&appName, "app", "", "Application name (required)")
	_ = postReceive.MarkFlagRequired("app")

	cmd.AddCommand(postReceive)

	return cmd
}

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

// DeployPipeline holds all dependencies for the deploy hook,
// making the entire pipeline testable via dependency injection.
type DeployPipeline struct {
	Runner   deploy.CommandRunner // Executes shell commands (git, systemctl, etc.)
	EnvMgr   *env.Manager         // Reads/writes .env files
	Systemd  *systemd.Generator   // Generates and manages systemd services
	CaddyDir string               // Directory for Caddyfile snippets
	HomeDir  string               // Base home directory (e.g. /home/amora)
	Stdin    io.Reader            // Source for git push ref info (os.Stdin in prod)
}

// Run executes the full deploy pipeline for the given app.
//
// Pipeline steps:
//  1. Parse ref from stdin (git sends oldsha newsha refname)
//  2. Checkout code from bare repo to ~/apps/<app>
//  3. Parse Procfile for process definitions
//  4. Generate systemd user services for each process
//  5. Generate Caddyfile and reload Caddy reverse proxy
//  6. Generate mDNS sidecar service for .local discovery
//  7. Reload systemd daemon and (re)start all services
func (p *DeployPipeline) Run(app string) error {
	Banner()
	LogInfo("Deploying '%s'...", app)

	repoPath := filepath.Join(p.HomeDir, "repos", app+".git")
	appDir := filepath.Join(p.HomeDir, "apps", app)

	// ── Step 1: Read push info from stdin ──────────────────────────────

	newRef, branch, err := parsePushInfo(p.Stdin)
	if err != nil {
		return err
	}

	shortSHA := newRef
	if len(shortSHA) > 8 {
		shortSHA = shortSHA[:8]
	}
	LogInfo("Received push on branch '%s' (%s)", branch, shortSHA)

	// ── Step 2: Checkout code ──────────────────────────────────────────

	LogInfo("Checking out code...")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return fmt.Errorf("creating app directory: %w", err)
	}

	err = p.Runner.Run("", "git",
		"--work-tree="+appDir,
		"--git-dir="+repoPath,
		"checkout", "-f", branch,
	)
	if err != nil {
		LogError("Checkout failed")
		return fmt.Errorf("git checkout: %w", err)
	}
	LogSuccess("Code checked out to %s", appDir)

	// ── Step 2.5: Runtime Provisioning (mise) & Build Phase ────────────

	LogInfo("Provisionando runtimes (mise)...")
	err = p.Runner.Run(appDir, "/home/amora/.local/bin/mise", "install")
	if err != nil {
		LogError("Falha ao instalar runtimes (mise install): %v", err)
		// Non-fatal: user might not be using mise runtimes
	}

	buildScript := filepath.Join(appDir, "amora-build")
	if stat, err := os.Stat(buildScript); err == nil && !stat.IsDir() {
		LogInfo("Executando amora-build...")
		if err := os.Chmod(buildScript, 0755); err != nil {
			LogError("Falha ao definir permissão em amora-build: %v", err)
			return fmt.Errorf("chmod amora-build: %w", err)
		}
		err = p.Runner.Run(appDir, "/home/amora/.local/bin/mise", "exec", "--", "./amora-build")
		if err != nil {
			LogError("Falha no build")
			return fmt.Errorf("build phase: %w", err)
		}
		LogSuccess("Build concluído")
	}

	// ── Step 3: Parse Procfile ─────────────────────────────────────────

	LogInfo("Reading Procfile...")
	procfilePath := filepath.Join(appDir, "Procfile")
	entries, err := deploy.ParseProcfile(procfilePath)
	if err != nil {
		LogError("Failed to parse Procfile: %v", err)
		return err
	}

	for _, e := range entries {
		LogSuccess("Process: %s → %s", e.Name, e.Command)
	}

	// ── Step 4: Allocate port (dynamic) ───────────────────────────────

	LogInfo("Resolving port allocation...")
	port, err := p.EnvMgr.GetOrAssignPort(app)
	if err != nil {
		LogError("Failed to allocate port: %v", err)
		return fmt.Errorf("port allocation: %w", err)
	}
	LogSuccess("PORT=%d", port)

	// ── Step 5: Generate systemd services ──────────────────────────────

	LogInfo("Generating systemd services...")
	envFile := p.EnvMgr.FilePath(app)

	var serviceNames []string
	for _, e := range entries {
		cfg := systemd.ServiceConfig{
			App:     app,
			Process: e.Name,
			WorkDir: appDir,
			Command: e.Command,
			EnvFile: envFile,
		}

		if err := p.Systemd.GenerateService(cfg); err != nil {
			LogError("Failed to generate service for '%s': %v", e.Name, err)
			return err
		}

		svcName := systemd.ServiceName(app, e.Name)
		serviceNames = append(serviceNames, svcName)
		LogSuccess("Generated %s", svcName)
	}

	// ── Step 6: Configure reverse proxy (Caddy) ────────────────────────

	if port > 0 {
		LogInfo("Configuring reverse proxy...")

		if err := proxy.GenerateCaddyfileAt(p.CaddyDir, app, port); err != nil {
			LogError("Failed to generate Caddyfile: %v", err)
			return err
		}
		LogSuccess("Caddyfile: %s.local → localhost:%d", app, port)

		if err := proxy.ReloadCaddyFrom(p.CaddyDir); err != nil {
			LogError("Failed to reload Caddy (is it running?): %v", err)
			// Non-fatal: the app may still work on its port directly.
		} else {
			LogSuccess("Caddy configuration reloaded")
		}
	}

	// ── Step 7: Configure mDNS (Avahi) ─────────────────────────────────

	LogInfo("Configuring mDNS (Avahi)...")
	if err := mdns.GenerateService(app); err != nil {
		LogError("mDNS setup failed: %v", err)
		// Non-fatal: the app is still accessible by IP.
	} else {
		LogSuccess("Generated mDNS entry in Avahi hosts")
	}

	// ── Step 8: Reload systemd and (re)start services ──────────────────

	LogInfo("Reloading systemd daemon...")
	if err := p.Systemd.DaemonReload(); err != nil {
		LogError("Failed to reload daemon: %v", err)
		return err
	}
	LogSuccess("Daemon reloaded")

	LogInfo("Starting services...")
	for _, svc := range serviceNames {
		if err := p.Systemd.EnableService(svc); err != nil {
			LogError("Failed to enable %s: %v", svc, err)
		}

		if err := p.Systemd.RestartService(svc); err != nil {
			LogError("Failed to restart %s: %v", svc, err)
		} else {
			LogSuccess("Started %s", svc)
		}
	}

	// ── Done ───────────────────────────────────────────────────────────

	fmt.Println()
	LogInfo("Deploy complete! 🎉")
	fmt.Println()

	if port > 0 {
		fmt.Printf("  🌐 http://%s.local\n", app)
		fmt.Printf("  🔌 http://localhost:%d\n", port)
	}
	fmt.Println()

	return nil
}

// parsePushInfo reads git push ref info from the given reader.
// Git sends lines formatted as: <old-sha> <new-sha> <ref>
func parsePushInfo(r io.Reader) (newRef, branch string, err error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) >= 3 {
			newRef = parts[1]
			// Extract branch name from refs/heads/<branch>
			refParts := strings.Split(parts[2], "/")
			branch = refParts[len(refParts)-1]
		}
	}

	if newRef == "" {
		return "", "", fmt.Errorf("no ref information received from git")
	}

	return newRef, branch, nil
}
