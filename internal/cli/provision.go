package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// Este script roda dentro do Pi e prepara o terreno.
// Usamos 'set -e' para parar no primeiro erro.
const setupScript = `
set -e

echo "📦 Atualizando sistema e instalando dependências base..."
sudo apt-get update
# Note que removemos o nodejs daqui e adicionamos ferramentas de build
sudo apt-get install -y git curl avahi-daemon build-essential libssl-dev

echo "🌐 Configurando Caddy Server..."
sudo apt-get install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor --yes -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list > /dev/null
sudo apt-get update
sudo apt-get install -y caddy

echo "👤 Configurando usuário amora e systemd Linger..."
if ! id -u amora > /dev/null 2>&1; then
    sudo adduser --disabled-password --gecos "" amora
fi
sudo usermod -aG systemd-journal amora
sudo loginctl enable-linger amora

echo "💎 Instalando mise (Gerenciador de Runtimes) para o usuário amora..."
sudo -u amora bash -c 'curl https://mise.run | sh'
# Previne duplicação no .bashrc usando o grep
sudo -u amora bash -c 'grep -q "mise activate" ~/.bashrc || echo "eval \"\$(/home/amora/.local/bin/mise activate bash)\"" >> ~/.bashrc'

echo "📡 Configurando mDNS (Avahi)..."
sudo touch /etc/avahi/hosts
sudo chown root:amora /etc/avahi/hosts
sudo chmod 664 /etc/avahi/hosts

echo "🔑 Configurando chaves SSH..."
sudo mkdir -p /home/amora/.ssh
sudo cp ~/.ssh/authorized_keys /home/amora/.ssh/authorized_keys || true
sudo chown -R amora:amora /home/amora/.ssh
sudo chmod 700 /home/amora/.ssh
sudo chmod 600 /home/amora/.ssh/authorized_keys

echo "✅ Infraestrutura Linux pronta!"
`

// NewProvisionCmd creates the `amora provision` command.
func NewProvisionCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "provision [user@host]",
		Short:   "Prepara um Raspberry Pi do zero para rodar o Amora",
		Example: "amora provision pi@raspberrypi.local",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]

			Banner()
			LogInfo("Iniciando provisionamento em %s...", target)

			// Passo 1: Executar o script de setup via SSH
			LogInfo("Configurando infraestrutura no servidor remoto...")
			if err := runSSH(target, setupScript); err != nil {
				return err
			}

			// Passo 2: Cross-compilar o binário para o Raspberry (ARM64)
			LogInfo("Compilando Amora para Linux ARM64...")
			buildCmd := exec.Command("go", "build", "-o", "amora-linux-arm64", "./cmd/amora")
			buildCmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=arm64")

			// Conecta a saída da compilação para mostrar possíveis erros
			buildCmd.Stdout = os.Stdout
			buildCmd.Stderr = os.Stderr

			if err := buildCmd.Run(); err != nil {
				LogError("Erro na compilação cruzada (ARM64)")
				return fmt.Errorf("compilação: %w", err)
			}
			defer os.Remove("amora-linux-arm64") // Garante que será limpo mesmo em caso de erro no scp

			// Passo 3: Enviar o binário compilado via SCP
			LogInfo("Enviando binário para o servidor...")
			scpCmd := exec.Command("scp", "amora-linux-arm64", fmt.Sprintf("%s:/tmp/amora", target))
			scpCmd.Stdout = os.Stdout
			scpCmd.Stderr = os.Stderr

			if err := scpCmd.Run(); err != nil {
				LogError("Erro no SCP")
				return fmt.Errorf("scp: %w", err)
			}

			// Passo 4: Instalar globalmente no Pi
			LogInfo("Instalando binário...")
			installScript := `sudo mv /tmp/amora /usr/local/bin/amora && sudo chmod +x /usr/local/bin/amora`
			if err := runSSH(target, installScript); err != nil {
				return err
			}

			// A limpeza local já está agendada pelo `defer` acima.

			fmt.Println()
			LogSuccess("Provisionamento concluído com sucesso! 🎉")
			fmt.Println("O Amora está instalado e pronto para receber deploys.")
			return nil
		},
	}
}

// runSSH executa um comando via SSH jogando a saída diretamente no terminal
func runSSH(target, command string) error {
	sshCmd := exec.Command("ssh", "-t", target, command)
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr
	sshCmd.Stdin = os.Stdin

	if err := sshCmd.Run(); err != nil {
		return fmt.Errorf("ssh error: %w", err)
	}
	return nil
}
