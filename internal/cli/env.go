package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/douglasgusson/amora/internal/env"
	"github.com/douglasgusson/amora/internal/systemd"
	"github.com/spf13/cobra"
)

// NewEnvCmd creates the `amora env` command group.
func NewEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Gerencia variáveis de ambiente dos apps",
	}

	cmd.AddCommand(
		newEnvSetCmd(),
		newEnvLsCmd(),
		newEnvRmCmd(),
	)

	return cmd
}

func newEnvSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set [app] [KEY=VALUE...]",
		Short: "Define uma ou mais variáveis de ambiente",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := args[0]
			vars, err := env.Load(app)
			if err != nil {
				return fmt.Errorf("erro ao carregar envs: %w", err)
			}

			for _, arg := range args[1:] {
				parts := strings.SplitN(arg, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("formato inválido: %s. Use KEY=VALUE", arg)
				}
				vars[parts[0]] = parts[1]
			}

			if err := env.Save(app, vars); err != nil {
				return fmt.Errorf("erro ao salvar envs: %w", err)
			}
			LogSuccess("Variáveis salvas para %s!", app)

			// Reinicia os serviços para aplicar as variáveis
			if err := systemd.DaemonReload(); err != nil {
				LogError("Falha no daemon-reload: %v", err)
			}
			if err := systemd.RestartAppServices(app); err != nil {
				LogError("Falha ao reiniciar serviços: %v", err)
			} else {
				LogSuccess("Serviços reiniciados.")
			}

			return nil
		},
	}
}

func newEnvLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls [app]",
		Short: "Lista as variáveis de ambiente do app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := args[0]
			vars, err := env.Load(app)
			if err != nil {
				return fmt.Errorf("erro ao carregar envs: %w", err)
			}

			if len(vars) == 0 {
				fmt.Println("Nenhuma variável configurada.")
				return nil
			}

			// Sort keys for deterministic output
			keys := make([]string, 0, len(vars))
			for k := range vars {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, k := range keys {
				fmt.Printf("%s=%s\n", k, vars[k])
			}
			return nil
		},
	}
}

func newEnvRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm [app] [KEY]",
		Short: "Remove uma variável de ambiente",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := args[0]
			key := args[1]

			if err := env.Remove(app, key); err != nil {
				return fmt.Errorf("erro ao remover env: %w", err)
			}
			LogSuccess("Variável %s removida de %s!", key, app)

			// Reinicia os serviços para aplicar as mudanças
			if err := systemd.DaemonReload(); err != nil {
				LogError("Falha no daemon-reload: %v", err)
			}
			if err := systemd.RestartAppServices(app); err != nil {
				LogError("Falha ao reiniciar serviços: %v", err)
			} else {
				LogSuccess("Serviços reiniciados.")
			}

			return nil
		},
	}
}
