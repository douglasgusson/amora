package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/douglasgusson/amora/internal/env"
	"github.com/spf13/cobra"
)

// appProcess holds the discovered information for a single process of an app.
type appProcess struct {
	App     string
	Process string
	Port    string
	Status  string
}

// NewAppsCmd creates the `amora apps` command.
func NewAppsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apps",
		Short: "Lista todos os aplicativos provisionados",
		Long: `Exibe uma tabela (Torre de Controle) contendo todos os aplicativos
provisionados, seus respectivos processos, portas alocadas e o status
em tempo real do daemon (via systemd).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("detecting home dir: %w", err)
			}

			// 1. Discover apps from ~/apps/
			appsDir := filepath.Join(home, "apps")
			entries, err := os.ReadDir(appsDir)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Println("Nenhuma aplicação encontrada.")
					return nil
				}
				return fmt.Errorf("reading apps dir: %w", err)
			}

			var apps []string
			for _, entry := range entries {
				if entry.IsDir() {
					apps = append(apps, entry.Name())
				}
			}

			if len(apps) == 0 {
				fmt.Println("Nenhuma aplicação encontrada.")
				return nil
			}

			// 2. Collect process info for each app
			systemdDir := filepath.Join(home, ".config", "systemd", "user")
			envManager := env.NewManager(env.DefaultDir())

			var rows []appProcess

			for _, app := range apps {
				// Discover port from env file
				vars, _ := envManager.Load(app)
				port := vars["PORT"] // empty string if not set

				// Discover processes via systemd service files
				prefix := fmt.Sprintf("amora-%s-", app)
				processes := discoverProcesses(systemdDir, prefix)

				if len(processes) == 0 {
					// App exists but has no services yet
					rows = append(rows, appProcess{
						App:     app,
						Process: "-",
						Port:    portOrDash(port, "-"),
						Status:  "⚪ Sem processos",
					})
					continue
				}

				for _, proc := range processes {
					serviceName := fmt.Sprintf("amora-%s-%s.service", app, proc)
					status := queryServiceStatus(serviceName)

					procPort := "-"
					if proc == "web" && port != "" {
						procPort = port
					}

					rows = append(rows, appProcess{
						App:     app,
						Process: proc,
						Port:    procPort,
						Status:  status,
					})
				}
			}

			// 3. Print table using tabwriter
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "APP\tPROCESSO\tPORTA\tSTATUS")
			fmt.Fprintln(w, "---\t--------\t-----\t------")
			for _, r := range rows {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.App, r.Process, r.Port, r.Status)
			}
			w.Flush()

			return nil
		},
	}

	return cmd
}

// discoverProcesses reads the systemd user directory and returns a list of
// process names for services matching the given prefix (e.g. "amora-myapp-").
func discoverProcesses(systemdDir, prefix string) []string {
	entries, err := os.ReadDir(systemdDir)
	if err != nil {
		return nil
	}

	var processes []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".service") {
			// Extract process name: strip prefix and ".service" suffix
			proc := strings.TrimPrefix(name, prefix)
			proc = strings.TrimSuffix(proc, ".service")
			if proc != "" {
				processes = append(processes, proc)
			}
		}
	}
	return processes
}

// queryServiceStatus runs `systemctl --user is-active <service>` and maps
// the output to a human-readable status string with emoji.
func queryServiceStatus(service string) string {
	out, err := exec.Command("systemctl", "--user", "is-active", service).Output()
	state := strings.TrimSpace(string(out))

	if err != nil && state == "" {
		// Command failed and produced no output — treat as unknown
		return "⚪ Desconhecido"
	}

	switch state {
	case "active":
		return "🟢 Online"
	case "inactive":
		return "🔴 Parado"
	case "failed":
		return "❌ Erro (Crash)"
	case "activating":
		return "🟡 Iniciando"
	default:
		return fmt.Sprintf("⚪ %s", state)
	}
}

// portOrDash returns the port if non-empty, or a fallback dash string.
func portOrDash(port, fallback string) string {
	if port != "" {
		return port
	}
	return fallback
}
