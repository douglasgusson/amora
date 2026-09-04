package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/douglasgusson/amora/internal/env"
	"github.com/douglasgusson/amora/internal/mdns"
	"github.com/douglasgusson/amora/internal/proxy"
	"github.com/douglasgusson/amora/internal/systemd"
)

// NewDestroyCmd creates the `amora destroy` command.
func NewDestroyCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "destroy [nome-do-app]",
		Short:   "Remove completamente uma aplicação",
		Example: "amora destroy meublog",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appName := args[0]

			Banner()

			// Pedir confirmação se não passar a flag --force
			if !force {
				fmt.Printf("⚠️  ATENÇÃO: Você está prestes a DESTRUIR completamente o app '%s'.\n", appName)
				fmt.Printf("Isso apagará código fonte, variáveis de ambiente, histórico git e configurações.\n")
				fmt.Print("Tem certeza? [s/N]: ")

				reader := bufio.NewReader(os.Stdin)
				response, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("erro lendo entrada: %w", err)
				}

				response = strings.TrimSpace(strings.ToLower(response))
				if response != "s" && response != "y" && response != "sim" && response != "yes" {
					fmt.Println("❌ Operação cancelada.")
					return nil
				}
			}

			LogInfo("Iniciando a destruição de '%s'...", appName)

			// 1. Parar e remover serviços systemd
			LogInfo("Limpando serviços do systemd...")
			if err := systemd.DestroyAppServices(appName); err != nil {
				LogError("Erro limpando serviços (systemd): %v", err)
			} else {
				LogSuccess("Serviços systemd removidos")
			}

			// 2. Remover configuração do Caddy e recarregar
			LogInfo("Limpando rotas do Caddy...")
			if err := proxy.RemoveCaddyfile(appName); err != nil {
				LogError("Erro limpando proxy (caddy): %v", err)
			} else {
				// Só recarrega se conseguiu remover
				if err := proxy.ReloadCaddy(); err != nil {
					LogError("Erro recarregando caddy: %v", err)
				} else {
					LogSuccess("Rotas removidas e proxy atualizado")
				}
			}

			// 3. Remover entrada mDNS (Avahi)
			LogInfo("Limpando registro mDNS...")
			if err := mdns.RemoveService(appName); err != nil {
				LogError("Erro limpando mDNS: %v", err)
			} else {
				LogSuccess("Registro de rede (mDNS) removido")
			}

			// 4. Remover arquivo de ambiente
			LogInfo("Limpando variáveis de ambiente e liberando porta...")
			if err := env.Delete(appName); err != nil {
				LogError("Erro deletando .env: %v", err)
			} else {
				LogSuccess("Variáveis removidas")
			}

			// 5. Apagar arquivos do disco (checkout e git bare)
			LogInfo("Apagando arquivos do disco...")
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("detecting home dir: %w", err)
			}

			appDir := filepath.Join(home, "apps", appName)
			repoDir := filepath.Join(home, "repos", appName+".git")

			if err := os.RemoveAll(appDir); err != nil {
				LogError("Erro removendo diretório da app (%s): %v", appDir, err)
			}
			if err := os.RemoveAll(repoDir); err != nil {
				LogError("Erro removendo diretório do repo (%s): %v", repoDir, err)
			}
			LogSuccess("Arquivos do disco apagados")

			fmt.Println()
			LogSuccess("Aplicação '%s' destruída com sucesso! 🗑️", appName)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Ignora o pedido de confirmação interativo")

	return cmd
}
