package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/douglasgusson/amora/internal/monitor"
	"github.com/spf13/cobra"
)

// NewStatusCmd creates the `amora status` command for host-level monitoring.
func NewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Mostra o consumo global do servidor (Zero-Overhead)",
		RunE: func(cmd *cobra.Command, args []string) error {
			metrics, err := monitor.GetHostMetrics()
			if err != nil {
				return fmt.Errorf("erro lendo métricas: %w", err)
			}

			Banner()
			LogInfo("Host Status")
			fmt.Println()

			// Formatação CPU Load
			fmt.Printf("💻 CPU Load (1m, 5m, 15m): %.2f, %.2f, %.2f\n", metrics.Load1, metrics.Load5, metrics.Load15)

			// Formatação RAM
			memPct := 0.0
			if metrics.MemTotal > 0 {
				memPct = float64(metrics.MemUsed) / float64(metrics.MemTotal) * 100
			}
			fmt.Printf("🧠 RAM: %s / %s (%.0f%%)\n", monitor.FormatBytes(metrics.MemUsed), monitor.FormatBytes(metrics.MemTotal), memPct)

			// Formatação Disco
			diskPct := 0.0
			if metrics.DiskTotal > 0 {
				diskPct = float64(metrics.DiskUsed) / float64(metrics.DiskTotal) * 100
			}
			fmt.Printf("💾 Disk (/): %s / %s (%.0f%%)\n", monitor.FormatBytes(metrics.DiskUsed), monitor.FormatBytes(metrics.DiskTotal), diskPct)

			fmt.Println()

			// Contagem de Apps
			apps := countApps()
			appNames := strings.Join(apps, ", ")
			if len(apps) == 0 {
				appNames = "nenhum"
			}
			fmt.Printf("📦 Apps Instalados: %d (%s)\n", len(apps), appNames)
			fmt.Println()

			return nil
		},
	}
}

func countApps() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	appsDir := filepath.Join(home, "apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return nil
	}

	var apps []string
	for _, entry := range entries {
		if entry.IsDir() {
			apps = append(apps, entry.Name())
		}
	}
	return apps
}
