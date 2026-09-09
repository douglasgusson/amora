package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// NewRestartCmd creates the `amora restart <app>` command.
func NewRestartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "restart [nome-do-app]",
		Short:   "Reinicia todos os processos de uma aplicação",
		Example: "amora restart meublog",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appName := args[0]

			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("detecting home dir: %w", err)
			}

			// 1. Discover services via globbing
			systemdDir := filepath.Join(home, ".config", "systemd", "user")
			prefix := fmt.Sprintf("amora-%s-", appName)

			entries, err := os.ReadDir(systemdDir)
			if err != nil {
				if os.IsNotExist(err) {
					LogError("App '%s' não encontrado ou não possui processos provisionados.", appName)
					os.Exit(1)
				}
				return fmt.Errorf("reading systemd dir: %w", err)
			}

			// Collect matching service files
			type serviceInfo struct {
				FileName string // e.g. "amora-myapp-web.service"
				Process  string // e.g. "web"
			}
			var services []serviceInfo

			for _, entry := range entries {
				name := entry.Name()
				if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".service") {
					proc := strings.TrimPrefix(name, prefix)
					proc = strings.TrimSuffix(proc, ".service")
					if proc != "" {
						services = append(services, serviceInfo{
							FileName: name,
							Process:  proc,
						})
					}
				}
			}

			// 2. Validate: abort if no services found
			if len(services) == 0 {
				LogError("App '%s' não encontrado ou não possui processos provisionados.", appName)
				os.Exit(1)
			}

			// 3. Iterate and restart each service
			fmt.Printf("🔄 Reiniciando aplicativo '%s'...\n", appName)

			hasError := false
			for _, svc := range services {
				fmt.Printf("   Reiniciando processo '%s'... ", svc.Process)

				if err := exec.Command("systemctl", "--user", "restart", svc.FileName).Run(); err != nil {
					fmt.Println("❌ Falhou")
					LogError("systemctl restart %s: %v", svc.FileName, err)
					hasError = true
				} else {
					fmt.Println("✅ Sucesso")
				}
			}

			if hasError {
				fmt.Println("⚠️  Concluído com erros.")
			} else {
				fmt.Println("✨ Concluído!")
			}

			return nil
		},
	}

	return cmd
}
