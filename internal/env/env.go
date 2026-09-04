package env

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultDir returns the default directory for per-app .env files.
func DefaultDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".amora", "envs")
}

// Manager handles reading and writing per-app .env files.
type Manager struct {
	BaseDir string
}

// NewManager creates a Manager with the given base directory.
func NewManager(baseDir string) *Manager {
	return &Manager{BaseDir: baseDir}
}

// FilePath returns the absolute path to an app's .env file.
func (m *Manager) FilePath(app string) string {
	return filepath.Join(m.BaseDir, app+".env")
}

// Load reads an app's .env file and returns a map of key=value pairs.
// Returns an empty map (not an error) if the file does not exist yet.
func (m *Manager) Load(app string) (map[string]string, error) {
	path := m.FilePath(app)
	vars := make(map[string]string)

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return vars, nil
		}
		return nil, fmt.Errorf("opening env file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			vars[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading env file: %w", err)
	}

	return vars, nil
}

// Save writes a map of key=value pairs to an app's .env file.
// Keys are sorted alphabetically for deterministic output.
func (m *Manager) Save(app string, vars map[string]string) error {
	path := m.FilePath(app)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating env directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating env file: %w", err)
	}
	defer f.Close()

	// Sort keys for deterministic output.
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if _, err := fmt.Fprintf(f, "%s=%s\n", k, vars[k]); err != nil {
			return fmt.Errorf("writing env var %s: %w", k, err)
		}
	}

	return nil
}

// Set loads, sets one key, and saves.
func (m *Manager) Set(app, key, value string) error {
	vars, err := m.Load(app)
	if err != nil {
		return err
	}
	vars[key] = value
	return m.Save(app, vars)
}

// Remove loads, deletes one key if it exists, and saves.
func (m *Manager) Remove(app, key string) error {
	vars, err := m.Load(app)
	if err != nil {
		return err
	}
	if _, exists := vars[key]; exists {
		delete(vars, key)
		return m.Save(app, vars)
	}
	return nil
}

// --- Package-level convenience functions using DefaultManager ---

var defaultManager = NewManager(DefaultDir())

// FilePath returns the absolute path to an app's .env file.
func FilePath(app string) string { return defaultManager.FilePath(app) }

// Load reads an app's .env file and returns a map of key=value pairs.
func Load(app string) (map[string]string, error) { return defaultManager.Load(app) }

// Save writes a map of key=value pairs to an app's .env file.
func Save(app string, vars map[string]string) error { return defaultManager.Save(app, vars) }

// Set loads, sets one key, and saves.
func Set(app, key, value string) error { return defaultManager.Set(app, key, value) }

// Remove loads, deletes one key if it exists, and saves.
func Remove(app, key string) error { return defaultManager.Remove(app, key) }
