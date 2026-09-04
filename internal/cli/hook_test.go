package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/douglasgusson/amora/internal/deploy"
	"github.com/douglasgusson/amora/internal/env"
	"github.com/douglasgusson/amora/internal/systemd"
)

// TestParsePushInfo verifies that the git ref stdin parser extracts the
// correct new SHA and branch name from different ref formats.
func TestParsePushInfo(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantRef    string
		wantBranch string
		wantErr    bool
	}{
		{
			name:       "standard main branch push",
			input:      "0000000000000000 a1b2c3d4e5f67890 refs/heads/main\n",
			wantRef:    "a1b2c3d4e5f67890",
			wantBranch: "main",
		},
		{
			name:       "feature branch push",
			input:      "abc123 def456 refs/heads/feature/login\n",
			wantRef:    "def456",
			wantBranch: "login",
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "malformed line",
			input:   "not-enough-fields\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			ref, branch, err := parsePushInfo(r)

			if (err != nil) != tt.wantErr {
				t.Fatalf("parsePushInfo() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if ref != tt.wantRef {
				t.Errorf("newRef = %q, want %q", ref, tt.wantRef)
			}
			if branch != tt.wantBranch {
				t.Errorf("branch = %q, want %q", branch, tt.wantBranch)
			}
		})
	}
}

// TestDeployPipeline_FullFlow simulates a full deploy and verifies that the
// MockRunner recorded the expected sequence of commands (git checkout,
// systemctl daemon-reload, systemctl enable, systemctl restart).
func TestDeployPipeline_FullFlow(t *testing.T) {
	// Set up a temporary directory structure mimicking the Amora home.
	homeDir := t.TempDir()

	app := "myblog"
	appDir := filepath.Join(homeDir, "apps", app)
	repoDir := filepath.Join(homeDir, "repos", app+".git")
	envDir := filepath.Join(homeDir, "env")
	systemdDir := filepath.Join(homeDir, "systemd")
	caddyDir := filepath.Join(homeDir, "caddy")

	// Create all required directories.
	for _, d := range []string{appDir, repoDir, envDir, systemdDir, caddyDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("creating dir %s: %v", d, err)
		}
	}

	// Create a Procfile in the app directory (simulating post-checkout state).
	procfile := "web: node server.js\nworker: python worker.py\n"
	if err := os.WriteFile(filepath.Join(appDir, "Procfile"), []byte(procfile), 0644); err != nil {
		t.Fatalf("writing Procfile: %v", err)
	}

	// Create a package.json (optional — proves checkout worked).
	if err := os.WriteFile(filepath.Join(appDir, "package.json"), []byte(`{"name":"myblog"}`), 0644); err != nil {
		t.Fatalf("writing package.json: %v", err)
	}

	// Seed the env file with a PORT.
	mgr := env.NewManager(envDir)
	if err := mgr.Save(app, map[string]string{"PORT": "5000"}); err != nil {
		t.Fatalf("saving env: %v", err)
	}

	// Create the mock runner and a noop-command systemd generator.
	runner := &deploy.MockRunner{}
	gen := &systemd.Generator{
		BaseDir: systemdDir,
		RunCmd: func(name string, args ...string) error {
			// Record in the same mock runner for unified assertion.
			parts := append([]string{name}, args...)
			runner.Commands = append(runner.Commands, strings.Join(parts, " "))
			return nil
		},
	}

	// Simulate stdin from git push (old-sha new-sha refs/heads/main).
	stdinContent := "0000000 a1b2c3d4e5f6 refs/heads/main\n"

	pipeline := &DeployPipeline{
		Runner:   runner,
		EnvMgr:   mgr,
		Systemd:  gen,
		CaddyDir: caddyDir,
		HomeDir:  homeDir,
		Stdin:    strings.NewReader(stdinContent),
	}

	if err := pipeline.Run(app); err != nil {
		t.Fatalf("pipeline.Run() error = %v", err)
	}

	// ── Assertions ─────────────────────────────────────────────────────

	// 1. Verify git checkout was called.
	assertCommandContains(t, runner.Commands, "git --work-tree=", "git checkout")

	// 2. Verify systemd services were generated (files should exist).
	webService := filepath.Join(systemdDir, "amora-myblog-web.service")
	workerService := filepath.Join(systemdDir, "amora-myblog-worker.service")

	if _, err := os.Stat(webService); os.IsNotExist(err) {
		t.Errorf("expected web service file to exist at %s", webService)
	}
	if _, err := os.Stat(workerService); os.IsNotExist(err) {
		t.Errorf("expected worker service file to exist at %s", workerService)
	}

	// 3. Verify the generated service file content.
	webContent, err := os.ReadFile(webService)
	if err != nil {
		t.Fatalf("reading web service: %v", err)
	}
	wantSubstrs := []string{
		"Description=Amora: myblog (web)",
		"WorkingDirectory=" + appDir,
		"EnvironmentFile=-" + mgr.FilePath(app),
	}
	for _, s := range wantSubstrs {
		if !strings.Contains(string(webContent), s) {
			t.Errorf("web service missing %q in:\n%s", s, string(webContent))
		}
	}
	if !strings.Contains(string(webContent), "ExecStart=/home/amora/.local/bin/mise exec -- /bin/bash -c 'node server.js'") {
		t.Errorf("web service missing ExecStart with mise in:\n%s", string(webContent))
	}

	// 4. Verify systemctl commands were called (daemon-reload, enable, restart).
	assertCommandContains(t, runner.Commands, "systemctl --user daemon-reload", "daemon-reload")
	assertCommandContains(t, runner.Commands, "systemctl --user enable", "enable service")
	assertCommandContains(t, runner.Commands, "systemctl --user restart", "restart service")

	// 5. Verify Caddyfile was generated.
	caddyFile := filepath.Join(caddyDir, "myblog.caddyfile")
	caddyContent, err := os.ReadFile(caddyFile)
	if err != nil {
		t.Fatalf("reading Caddyfile: %v", err)
	}
	if !strings.Contains(string(caddyContent), "http://myblog.local {") {
		t.Errorf("Caddyfile missing 'http://myblog.local {': %s", string(caddyContent))
	}
	if !strings.Contains(string(caddyContent), "reverse_proxy 127.0.0.1:5000") {
		t.Errorf("Caddyfile missing reverse_proxy 127.0.0.1:5000: %s", string(caddyContent))
	}
}

// TestDeployPipeline_MissingProcfile verifies the pipeline fails gracefully
// when the Procfile is missing from the checked-out app.
func TestDeployPipeline_MissingProcfile(t *testing.T) {
	homeDir := t.TempDir()
	app := "noapp"

	// Create directories but NO Procfile.
	appDir := filepath.Join(homeDir, "apps", app)
	os.MkdirAll(appDir, 0755)
	os.MkdirAll(filepath.Join(homeDir, "repos", app+".git"), 0755)

	runner := &deploy.MockRunner{}
	mgr := env.NewManager(filepath.Join(homeDir, "env"))
	gen := systemd.NewGenerator(filepath.Join(homeDir, "systemd"))
	gen.RunCmd = func(name string, args ...string) error { return nil }

	pipeline := &DeployPipeline{
		Runner:   runner,
		EnvMgr:   mgr,
		Systemd:  gen,
		CaddyDir: filepath.Join(homeDir, "caddy"),
		HomeDir:  homeDir,
		Stdin:    strings.NewReader("000 abc123 refs/heads/main\n"),
	}

	err := pipeline.Run(app)
	if err == nil {
		t.Fatal("expected error for missing Procfile, got nil")
	}
	if !strings.Contains(err.Error(), "Procfile") {
		t.Errorf("error = %q, want substring 'Procfile'", err.Error())
	}
}

// TestDeployPipeline_NoPort verifies the pipeline skips Caddy configuration
// when no PORT is set in the app's environment.
func TestDeployPipeline_NoPort(t *testing.T) {
	homeDir := t.TempDir()
	app := "noportapp"

	appDir := filepath.Join(homeDir, "apps", app)
	os.MkdirAll(appDir, 0755)
	os.MkdirAll(filepath.Join(homeDir, "repos", app+".git"), 0755)

	// Create Procfile but no env file (so no PORT).
	os.WriteFile(filepath.Join(appDir, "Procfile"), []byte("web: ./serve\n"), 0644)

	runner := &deploy.MockRunner{}
	mgr := env.NewManager(filepath.Join(homeDir, "env"))
	systemdDir := filepath.Join(homeDir, "systemd")
	gen := &systemd.Generator{
		BaseDir: systemdDir,
		RunCmd:  func(name string, args ...string) error { return nil },
	}
	caddyDir := filepath.Join(homeDir, "caddy")

	pipeline := &DeployPipeline{
		Runner:   runner,
		EnvMgr:   mgr,
		Systemd:  gen,
		CaddyDir: caddyDir,
		HomeDir:  homeDir,
		Stdin:    strings.NewReader("000 abc123 refs/heads/main\n"),
	}

	if err := pipeline.Run(app); err != nil {
		t.Fatalf("pipeline.Run() error = %v", err)
	}

	// Caddy dir should NOT have any file since PORT was not set.
	entries, _ := os.ReadDir(caddyDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".caddyfile") {
			t.Errorf("unexpected Caddyfile generated without PORT: %s", e.Name())
		}
	}
}

// assertCommandContains checks that at least one recorded command contains
// the given substring.
func assertCommandContains(t *testing.T, commands []string, substr, label string) {
	t.Helper()
	for _, cmd := range commands {
		if strings.Contains(cmd, substr) {
			return
		}
	}
	t.Errorf("no command found containing %q (%s); recorded commands:\n  %s",
		substr, label, strings.Join(commands, "\n  "))
}
