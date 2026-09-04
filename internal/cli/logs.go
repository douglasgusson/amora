package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// NewLogsCmd creates the `amora logs` command.
func NewLogsCmd() *cobra.Command {
	var followLogs bool
	var linesCount int

	cmd := &cobra.Command{
		Use:   "logs [nome-do-app]",
		Short: "Exibe os logs de uma aplicação",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			appName := args[0]
			serviceName := fmt.Sprintf("amora-%s-web.service", appName)

			LogInfo("Buscando logs de %s...", appName)

			// Monta o comando base do journalctl
			journalArgs := []string{
				"--user",
				"--no-pager",
				"-u", serviceName,
				"-n", fmt.Sprintf("%d", linesCount),
			}

			// Se o usuário passou a flag -f (follow), adicionamos ao comando
			if followLogs {
				journalArgs = append(journalArgs, "-f")
			}

			// Instancia o processo no sistema operacional
			execCmd := exec.Command("journalctl", journalArgs...)

			// O pulo do gato: conectamos as saídas do journalctl direto no terminal atual
			execCmd.Stdout = os.Stdout
			execCmd.Stderr = os.Stderr
			execCmd.Stdin = os.Stdin

			// Roda o comando (se for com -f, ele vai "travar" aqui até o usuário apertar Ctrl+C)
			if err := execCmd.Run(); err != nil {
				// Se o usuário apertar Ctrl+C, o exec.Run retorna um erro genérico que podemos ignorar
				if err.Error() != "signal: interrupt" {
					LogError("Erro ao ler os logs: %v", err)
				}
			}
		},
	}

	// Adicionamos as flags inspiradas no comando clássico do Docker/Heroku
	cmd.Flags().BoolVarP(&followLogs, "follow", "f", false, "Acompanha os logs em tempo real")
	cmd.Flags().IntVarP(&linesCount, "lines", "n", 50, "Número de linhas recentes para exibir")

	return cmd
}
