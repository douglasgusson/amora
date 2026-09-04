package systemd

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestServiceName(t *testing.T) {
	tests := []struct {
		name     string
		app      string
		process  string
		expected string
	}{
		{
			name:     "web process",
			app:      "myapp",
			process:  "web",
			expected: "amora-myapp-web.service",
		},
		{
			name:     "worker process",
			app:      "myapp",
			process:  "worker",
			expected: "amora-myapp-worker.service",
		},
		{
			name:     "hyphenated names",
			app:      "my-cool-app",
			process:  "queue-runner",
			expected: "amora-my-cool-app-queue-runner.service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ServiceName(tt.app, tt.process)
			if got != tt.expected {
				t.Errorf("ServiceName(%q, %q) = %q; want %q", tt.app, tt.process, got, tt.expected)
			}
		})
	}
}

func TestGenerator_GenerateService(t *testing.T) {
	tests := []struct {
		name       string
		cfg        ServiceConfig
		wantSubstr []string
	}{
		{
			name: "standard web service",
			cfg: ServiceConfig{
				App:     "blog",
				Process: "web",
				WorkDir: "/home/amora/apps/blog",
				Command: "bundle exec rails server -p 3000",
				EnvFile: "/home/amora/env/blog.env",
			},
			wantSubstr: []string{
				"Description=Amora: blog (web)",
				"WorkingDirectory=/home/amora/apps/blog",
				"ExecStart=/home/amora/.local/bin/mise exec -- /bin/bash -c 'bundle exec rails server -p 3000'",
				"Restart=on-failure",
				"RestartSec=5",
				"EnvironmentFile=-/home/amora/env/blog.env",
				"WantedBy=default.target",
			},
		},
		{
			name: "worker process without env file",
			cfg: ServiceConfig{
				App:     "workerapp",
				Process: "worker",
				WorkDir: "/var/app",
				Command: "npm run worker",
				EnvFile: "",
			},
			wantSubstr: []string{
				"Description=Amora: workerapp (worker)",
				"WorkingDirectory=/var/app",
				"ExecStart=/home/amora/.local/bin/mise exec -- /bin/bash -c 'npm run worker'",
				"EnvironmentFile=-",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			gen := NewGenerator(tmpDir)

			if err := gen.GenerateService(tt.cfg); err != nil {
				t.Fatalf("GenerateService() unexpected error: %v", err)
			}

			fileName := ServiceName(tt.cfg.App, tt.cfg.Process)
			filePath := filepath.Join(tmpDir, fileName)

			data, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("reading generated service file: %v", err)
			}

			content := string(data)
			for _, substr := range tt.wantSubstr {
				if !strings.Contains(content, substr) {
					t.Errorf("GenerateService() output missing %q;\nContent:\n%s", substr, content)
				}
			}
		})
	}
}

func TestGenerator_Commands(t *testing.T) {
	tests := []struct {
		name        string
		action      func(g *Generator) error
		wantCmd     string
		wantArgs    []string
		returnErr   error
		wantErrText string
	}{
		{
			name: "DaemonReload success",
			action: func(g *Generator) error {
				return g.DaemonReload()
			},
			wantCmd:  "systemctl",
			wantArgs: []string{"--user", "daemon-reload"},
		},
		{
			name: "DaemonReload failure",
			action: func(g *Generator) error {
				return g.DaemonReload()
			},
			wantCmd:     "systemctl",
			wantArgs:    []string{"--user", "daemon-reload"},
			returnErr:   errors.New("daemon reload failed"),
			wantErrText: "daemon reload failed",
		},
		{
			name: "EnableService success",
			action: func(g *Generator) error {
				return g.EnableService("amora-demo-web.service")
			},
			wantCmd:  "systemctl",
			wantArgs: []string{"--user", "enable", "amora-demo-web.service"},
		},
		{
			name: "RestartService success",
			action: func(g *Generator) error {
				return g.RestartService("amora-demo-web.service")
			},
			wantCmd:  "systemctl",
			wantArgs: []string{"--user", "restart", "amora-demo-web.service"},
		},
		{
			name: "StopService success",
			action: func(g *Generator) error {
				return g.StopService("amora-demo-web.service")
			},
			wantCmd:  "systemctl",
			wantArgs: []string{"--user", "stop", "amora-demo-web.service"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var recordedCmd string
			var recordedArgs []string

			gen := &Generator{
				BaseDir: "/tmp",
				RunCmd: func(name string, args ...string) error {
					recordedCmd = name
					recordedArgs = args
					return tt.returnErr
				},
			}

			err := tt.action(gen)
			if tt.wantErrText != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrText) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErrText, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if recordedCmd != tt.wantCmd {
				t.Errorf("command = %q; want %q", recordedCmd, tt.wantCmd)
			}
			if !reflect.DeepEqual(recordedArgs, tt.wantArgs) {
				t.Errorf("args = %v; want %v", recordedArgs, tt.wantArgs)
			}
		})
	}
}

func TestGenerator_RestartAppServices(t *testing.T) {
	t.Run("directory does not exist", func(t *testing.T) {
		gen := &Generator{
			BaseDir: filepath.Join(t.TempDir(), "non-existent"),
			RunCmd: func(name string, args ...string) error {
				t.Fatal("RunCmd should not be called when directory does not exist")
				return nil
			},
		}

		if err := gen.RestartAppServices("myapp"); err != nil {
			t.Fatalf("expected nil error for missing directory, got %v", err)
		}
	})

	t.Run("restarts only matching services", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create files in tmpDir
		files := []string{
			"amora-myapp-web.service",
			"amora-myapp-worker.service",
			"amora-otherapp-web.service",
			"not-a-service.txt",
			"amora-myapp-notes.txt",
		}
		for _, f := range files {
			if err := os.WriteFile(filepath.Join(tmpDir, f), []byte("test"), 0644); err != nil {
				t.Fatalf("failed to create dummy file %s: %v", f, err)
			}
		}

		var restarted []string
		gen := &Generator{
			BaseDir: tmpDir,
			RunCmd: func(name string, args ...string) error {
				if name == "systemctl" && len(args) == 3 && args[0] == "--user" && args[1] == "restart" {
					restarted = append(restarted, args[2])
					return nil
				}
				return errors.New("unexpected command invocation")
			},
		}

		if err := gen.RestartAppServices("myapp"); err != nil {
			t.Fatalf("RestartAppServices() unexpected error: %v", err)
		}

		wantRestarted := []string{"amora-myapp-web.service", "amora-myapp-worker.service"}
		if !reflect.DeepEqual(restarted, wantRestarted) {
			t.Errorf("restarted services = %v; want %v", restarted, wantRestarted)
		}
	})

	t.Run("returns error on service restart failure", func(t *testing.T) {
		tmpDir := t.TempDir()

		svcFile := "amora-myapp-web.service"
		if err := os.WriteFile(filepath.Join(tmpDir, svcFile), []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create dummy file %s: %v", svcFile, err)
		}

		gen := &Generator{
			BaseDir: tmpDir,
			RunCmd: func(name string, args ...string) error {
				return errors.New("systemctl restart failed")
			},
		}

		err := gen.RestartAppServices("myapp")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "restarting amora-myapp-web.service") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}
