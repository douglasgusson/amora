package mdns

import (
	"fmt"
	"net"
	"os"
	"strings"
)

// GetLocalIP discovers the machine's first non-loopback IPv4 address.
// This is the IP that will be advertised via mDNS for <app>.local.
func GetLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", fmt.Errorf("listing network interfaces: %w", err)
	}

	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}

		// Skip loopback and IPv6 addresses.
		if ipnet.IP.IsLoopback() || ipnet.IP.To4() == nil {
			continue
		}

		return ipnet.IP.String(), nil
	}

	return "", fmt.Errorf("no suitable non-loopback IPv4 address found")
}

// GenerateService escreve o IP e o Domínio diretamente no arquivo do Avahi.
// Isso elimina a necessidade de rodar um serviço systemd separado.
// Se o domínio já existir, atualiza o IP (idempotente e resiliente a mudanças de DHCP).
func GenerateService(app string) error {
	ip, err := GetLocalIP()
	if err != nil {
		return fmt.Errorf("detecting local IP: %w", err)
	}

	hostsPath := "/etc/avahi/hosts"
	domain := fmt.Sprintf("%s.local", app)
	entry := fmt.Sprintf("%s %s", ip, domain)

	content, _ := os.ReadFile(hostsPath)
	lines := strings.Split(string(content), "\n")

	var newLines []string
	found := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Verifica se essa linha é do nosso domínio (qualquer IP)
		if trimmed != "" && strings.HasSuffix(trimmed, " "+domain) {
			// Substitui com o IP atual
			newLines = append(newLines, entry)
			found = true
		} else {
			newLines = append(newLines, line)
		}
	}

	if !found {
		// Remove trailing empty line antes de adicionar, para evitar linhas em branco extras
		for len(newLines) > 0 && strings.TrimSpace(newLines[len(newLines)-1]) == "" {
			newLines = newLines[:len(newLines)-1]
		}
		newLines = append(newLines, entry)
	}

	result := strings.Join(newLines, "\n")
	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	if err := os.WriteFile(hostsPath, []byte(result), 0664); err != nil {
		return fmt.Errorf("writing avahi hosts: %w", err)
	}

	return nil
}

// RemoveService deleta o domínio do app do arquivo do Avahi.
func RemoveService(app string) error {
	hostsPath := "/etc/avahi/hosts"
	domain := fmt.Sprintf("%s.local", app)

	content, err := os.ReadFile(hostsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Se o arquivo não existe, não há nada para remover
		}
		return fmt.Errorf("reading avahi hosts: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	changed := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && strings.HasSuffix(trimmed, " "+domain) {
			changed = true
			continue // Pula essa linha para removê-la
		}
		newLines = append(newLines, line)
	}

	if !changed {
		return nil
	}

	result := strings.Join(newLines, "\n")
	if !strings.HasSuffix(result, "\n") && len(newLines) > 0 {
		result += "\n"
	}

	if err := os.WriteFile(hostsPath, []byte(result), 0664); err != nil {
		return fmt.Errorf("writing avahi hosts: %w", err)
	}

	return nil
}
