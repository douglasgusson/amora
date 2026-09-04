package deploy

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

const (
	gray  = "\033[90m"
	reset = "\033[0m"
)

// CommandRunner is the interface for executing commands and streaming their output.
type CommandRunner interface {
	Run(dir string, name string, args ...string) error
}

// RealRunner executes commands on the host system, streaming stdout and stderr
// to the terminal in real-time with formatting.
type RealRunner struct{}

var _ CommandRunner = (*RealRunner)(nil)

// Run executes the named command with args, setting its working directory to dir
// (if non-empty), and streams stdout/stderr concurrently.
func (r *RealRunner) Run(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("creating stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting command '%s': %w", name, err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			fmt.Printf("%s       %s%s\n", gray, scanner.Text(), reset)
		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Printf("%s       %s%s\n", gray, scanner.Text(), reset)
		}
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("command '%s' failed: %w", name, err)
	}

	return nil
}

// MockRunner is a CommandRunner implementation for testing.
// It records executed commands and returns a configured error if set.
type MockRunner struct {
	Commands []string
	Err      error
}

var _ CommandRunner = (*MockRunner)(nil)

// Run records the command invocation in Commands formatted as "name arg1 arg2 ..."
// and returns Err (which defaults to nil).
func (m *MockRunner) Run(dir string, name string, args ...string) error {
	entry := name
	if len(args) > 0 {
		entry += " " + strings.Join(args, " ")
	}
	m.Commands = append(m.Commands, entry)
	return m.Err
}

// StreamCommand executes a command and streams its stdout/stderr to the
// terminal in real-time, prefixed with gray coloring.
// Maintained for backward compatibility.
func StreamCommand(name string, args ...string) error {
	runner := &RealRunner{}
	return runner.Run("", name, args...)
}

// StreamShell executes a shell command string via /bin/bash -c and streams
// its output.
// Maintained for backward compatibility.
func StreamShell(command string) error {
	runner := &RealRunner{}
	return runner.Run("", "/bin/bash", "-c", command)
}
