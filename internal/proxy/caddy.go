package proxy

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	// CaddyDir is where per-app Caddyfile snippets are stored.
	CaddyDir = "/home/amora/caddy"

	// CaddyAPI is the Caddy admin API endpoint for loading configuration.
	CaddyAPI = "http://localhost:2019/load"
)

// CaddyfilePath returns the path to an app's Caddyfile snippet.
func CaddyfilePath(app string) string {
	return filepath.Join(CaddyDir, app+".caddyfile")
}

// GenerateCaddyfile creates a Caddyfile snippet that reverse-proxies
// <app>.local to the app's port on localhost.
func GenerateCaddyfile(app string, port int) error {
	return GenerateCaddyfileAt(CaddyDir, app, port)
}

// ReloadCaddy combines all per-app Caddyfile snippets and POSTs the result
// to Caddy's admin API at /load with Content-Type: text/caddyfile.
//
// This approach lets each app own its own routing snippet while Caddy
// receives a single unified configuration on each deploy.
func ReloadCaddy() error {
	return ReloadCaddyFrom(CaddyDir)
}

// RemoveCaddyfile deletes an app's Caddyfile snippet.
func RemoveCaddyfile(app string) error {
	path := CaddyfilePath(app)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing Caddyfile for %s: %w", app, err)
	}
	return nil
}

// GenerateCaddyfileAt generates a Caddyfile snippet in the given directory.
// This is the testable variant — GenerateCaddyfile delegates to it using CaddyDir.
func GenerateCaddyfileAt(dir, app string, port int) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating caddy dir: %w", err)
	}

	content := fmt.Sprintf("http://%s.local {\n\treverse_proxy 127.0.0.1:%d\n}\n", app, port)
	path := filepath.Join(dir, app+".caddyfile")

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing Caddyfile for %s: %w", app, err)
	}

	return nil
}

// ReloadCaddyFrom combines all Caddyfile snippets from the given directory
// and POSTs them to the Caddy admin API.
func ReloadCaddyFrom(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading caddy dir: %w", err)
	}

	var combined strings.Builder

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".caddyfile") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return fmt.Errorf("reading %s: %w", entry.Name(), err)
		}

		combined.Write(data)
		combined.WriteString("\n")
	}

	if combined.Len() == 0 {
		return nil
	}

	req, err := http.NewRequest("POST", CaddyAPI, strings.NewReader(combined.String()))
	if err != nil {
		return fmt.Errorf("creating Caddy API request: %w", err)
	}
	req.Header.Set("Content-Type", "text/caddyfile")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("posting to Caddy API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Caddy API returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
