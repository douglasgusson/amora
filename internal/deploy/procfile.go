package deploy

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ProcEntry represents a single process defined in a Procfile.
type ProcEntry struct {
	Name    string // Process type (e.g. "web", "worker")
	Command string // Shell command to execute
}

// ParseProcfile reads a Procfile and returns its entries.
//
// Procfile format (one entry per line):
//
//	web: npm start
//	worker: python worker.py
//
// Empty lines and lines starting with '#' are ignored.
func ParseProcfile(path string) ([]ProcEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening Procfile: %w", err)
	}
	defer f.Close()

	var entries []ProcEntry
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("Procfile line %d: invalid format (expected 'type: command')", lineNum)
		}

		name := strings.TrimSpace(parts[0])
		cmd := strings.TrimSpace(parts[1])

		if name == "" || cmd == "" {
			return nil, fmt.Errorf("Procfile line %d: empty process name or command", lineNum)
		}

		entries = append(entries, ProcEntry{Name: name, Command: cmd})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading Procfile: %w", err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no valid entries found in Procfile")
	}

	return entries, nil
}
