package systemd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

// ServiceTemplate is the systemd unit file template for Amora application processes.
const ServiceTemplate = `[Unit]
Description=Amora: {{.App}} ({{.Process}})
After=network.target

[Service]
Type=simple
WorkingDirectory={{.WorkDir}}
ExecStart=/home/amora/.local/bin/mise exec -- /bin/bash -c '{{.Command}}'
Restart=on-failure
RestartSec=5
EnvironmentFile=-{{.EnvFile}}

[Install]
WantedBy=default.target
`

// ServiceConfig holds the parameters needed to generate a systemd unit file.
type ServiceConfig struct {
	App     string // Application name
	Process string // Process type from Procfile (e.g. "web", "worker")
	WorkDir string // Working directory (app checkout path)
	Command string // Shell command to execute
	EnvFile string // Path to the .env file (with systemd's `-` prefix for optional)
}

// ServiceName returns the systemd unit name for an app process.
// Format: amora-<app>-<process>.service
func ServiceName(app, process string) string {
	return fmt.Sprintf("amora-%s-%s.service", app, process)
}

// Generator writes systemd unit files to a configurable directory
// and delegates command execution to a CommandRunner.
type Generator struct {
	BaseDir string                                  // Where to write .service files
	RunCmd  func(name string, args ...string) error // Function to run systemctl commands
}

// NewGenerator creates a Generator that writes to baseDir and runs real exec.Command.
func NewGenerator(baseDir string) *Generator {
	return &Generator{
		BaseDir: baseDir,
		RunCmd: func(name string, args ...string) error {
			return exec.Command(name, args...).Run()
		},
	}
}

// GenerateService writes a .service file to BaseDir.
func (g *Generator) GenerateService(cfg ServiceConfig) error {
	if err := os.MkdirAll(g.BaseDir, 0755); err != nil {
		return fmt.Errorf("creating systemd dir: %w", err)
	}

	name := ServiceName(cfg.App, cfg.Process)
	path := filepath.Join(g.BaseDir, name)

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating service file %s: %w", name, err)
	}
	defer f.Close()

	tmpl, err := template.New("service").Parse(ServiceTemplate)
	if err != nil {
		return fmt.Errorf("parsing service template: %w", err)
	}

	if err := tmpl.Execute(f, cfg); err != nil {
		return fmt.Errorf("rendering service template: %w", err)
	}

	return nil
}

// DaemonReload runs systemctl --user daemon-reload.
func (g *Generator) DaemonReload() error {
	return g.RunCmd("systemctl", "--user", "daemon-reload")
}

// EnableService enables a service.
func (g *Generator) EnableService(name string) error {
	return g.RunCmd("systemctl", "--user", "enable", name)
}

// RestartService restarts a service.
func (g *Generator) RestartService(name string) error {
	return g.RunCmd("systemctl", "--user", "restart", name)
}

// StopService stops a service.
func (g *Generator) StopService(name string) error {
	return g.RunCmd("systemctl", "--user", "stop", name)
}

// DisableService disables a service.
func (g *Generator) DisableService(name string) error {
	return g.RunCmd("systemctl", "--user", "disable", name)
}

// DestroyAppServices finds, stops, disables and removes all services for an app.
func (g *Generator) DestroyAppServices(app string) error {
	prefix := fmt.Sprintf("amora-%s-", app)

	entries, err := os.ReadDir(g.BaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading systemd dir: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".service") {
			// Stop and disable via systemctl
			g.StopService(name)
			g.DisableService(name)
			// Remove the unit file
			os.Remove(filepath.Join(g.BaseDir, name))
		}
	}

	return g.DaemonReload()
}

// RestartAppServices finds and restarts all services for an app.
func (g *Generator) RestartAppServices(app string) error {
	prefix := fmt.Sprintf("amora-%s-", app)

	entries, err := os.ReadDir(g.BaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading systemd dir: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".service") {
			// Auto-clean legacy mDNS services since we pivoted to /etc/avahi/hosts
			if strings.HasSuffix(name, "-mdns.service") {
				os.Remove(filepath.Join(g.BaseDir, name))
				continue
			}

			if err := g.RestartService(name); err != nil {
				return fmt.Errorf("restarting %s: %w", name, err)
			}
		}
	}

	return nil
}

// --- Package-level convenience functions for backward compat ---

// DefaultDir returns the default systemd user service directory.
func DefaultDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user")
}

var defaultGenerator = NewGenerator(DefaultDir())

func GenerateService(cfg ServiceConfig) error { return defaultGenerator.GenerateService(cfg) }
func DaemonReload() error                     { return defaultGenerator.DaemonReload() }
func EnableService(name string) error         { return defaultGenerator.EnableService(name) }
func RestartService(name string) error        { return defaultGenerator.RestartService(name) }
func StopService(name string) error           { return defaultGenerator.StopService(name) }
func DisableService(name string) error        { return defaultGenerator.DisableService(name) }
func RestartAppServices(app string) error     { return defaultGenerator.RestartAppServices(app) }
func DestroyAppServices(app string) error     { return defaultGenerator.DestroyAppServices(app) }
