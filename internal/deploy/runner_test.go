package deploy

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMockRunner_Interface(t *testing.T) {
	var runner CommandRunner = &MockRunner{}
	if runner == nil {
		t.Fatal("expected runner to not be nil")
	}
}

func TestMockRunner_Run(t *testing.T) {
	tests := []struct {
		name         string
		calls        []struct {
			dir  string
			name string
			args []string
		}
		mockErr      error
		wantCommands []string
		wantErr      bool
	}{
		{
			name: "single command without args",
			calls: []struct {
				dir  string
				name string
				args []string
			}{
				{dir: "", name: "ls", args: nil},
			},
			mockErr:      nil,
			wantCommands: []string{"ls"},
			wantErr:      false,
		},
		{
			name: "command with multiple args",
			calls: []struct {
				dir  string
				name string
				args []string
			}{
				{dir: "/app", name: "git", args: []string{"checkout", "-b", "main"}},
			},
			mockErr:      nil,
			wantCommands: []string{"git checkout -b main"},
			wantErr:      false,
		},
		{
			name: "multiple sequential calls",
			calls: []struct {
				dir  string
				name string
				args []string
			}{
				{dir: "", name: "git", args: []string{"init"}},
				{dir: "", name: "git", args: []string{"add", "."}},
				{dir: "", name: "git", args: []string{"commit", "-m", "initial"}},
			},
			mockErr:      nil,
			wantCommands: []string{"git init", "git add .", "git commit -m initial"},
			wantErr:      false,
		},
		{
			name: "returns configured error",
			calls: []struct {
				dir  string
				name string
				args []string
			}{
				{dir: "", name: "fail-cmd", args: []string{"arg"}},
			},
			mockErr:      errors.New("execution failed"),
			wantCommands: []string{"fail-cmd arg"},
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &MockRunner{Err: tt.mockErr}

			for _, call := range tt.calls {
				err := runner.Run(call.dir, call.name, call.args...)
				if (err != nil) != tt.wantErr {
					t.Fatalf("Run() error = %v, wantErr %v", err, tt.wantErr)
				}
				if tt.wantErr && !errors.Is(err, tt.mockErr) {
					t.Errorf("Run() error = %v, want %v", err, tt.mockErr)
				}
			}

			if len(runner.Commands) != len(tt.wantCommands) {
				t.Fatalf("Commands length = %d, want %d", len(runner.Commands), len(tt.wantCommands))
			}

			for i := range runner.Commands {
				if runner.Commands[i] != tt.wantCommands[i] {
					t.Errorf("Commands[%d] = %q, want %q", i, runner.Commands[i], tt.wantCommands[i])
				}
			}
		})
	}
}

func TestRealRunner_Interface(t *testing.T) {
	var runner CommandRunner = &RealRunner{}
	if runner == nil {
		t.Fatal("expected runner to not be nil")
	}
}

func TestRealRunner_Run(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		dir       string
		cmd       string
		args      []string
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "successful command",
			dir:     "",
			cmd:     "echo",
			args:    []string{"hello"},
			wantErr: false,
		},
		{
			name:    "successful command with working directory",
			dir:     tmpDir,
			cmd:     "pwd",
			args:    nil,
			wantErr: false,
		},
		{
			name:      "nonexistent command",
			dir:       "",
			cmd:       "nonexistent_command_12345",
			args:      nil,
			wantErr:   true,
			errSubstr: "starting command",
		},
		{
			name:      "command that exits with error",
			dir:       "",
			cmd:       "/bin/bash",
			args:      []string{"-c", "exit 1"},
			wantErr:   true,
			errSubstr: "failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &RealRunner{}
			err := runner.Run(tt.dir, tt.cmd, tt.args...)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("Run() error = %q, want substring %q", err.Error(), tt.errSubstr)
			}
		})
	}
}

func TestRealRunner_WorkingDir(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	runner := &RealRunner{}
	// Running "ls test.txt" inside tmpDir will succeed only if working directory is tmpDir
	err := runner.Run(tmpDir, "ls", "test.txt")
	if err != nil {
		t.Fatalf("expected command to succeed in working directory: %v", err)
	}
}

func TestStreamCommand(t *testing.T) {
	err := StreamCommand("echo", "backward", "compatibility")
	if err != nil {
		t.Fatalf("StreamCommand() error = %v", err)
	}

	err = StreamCommand("nonexistent_command_12345")
	if err == nil {
		t.Fatal("StreamCommand() expected error for nonexistent command, got nil")
	}
}

func TestStreamShell(t *testing.T) {
	err := StreamShell("echo 'shell backward compatibility'")
	if err != nil {
		t.Fatalf("StreamShell() error = %v", err)
	}

	err = StreamShell("exit 2")
	if err == nil {
		t.Fatal("StreamShell() expected error for failing shell command, got nil")
	}
}
