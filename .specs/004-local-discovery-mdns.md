# Spec 004: Local Discovery (Avahi mDNS)

## 1. Objetivo e Fluxo
Para consolidar a experiência de PaaS local sem depender de servidores de DNS (como BIND ou roteadores customizados), o Amora implementa a funcionalidade de Local Discovery usando o protocolo mDNS (Multicast DNS) através do Avahi Daemon.

O objetivo deste módulo é garantir que assim que a aplicação sobe (especificamente processos do tipo `web`), ela se torna imediatamente acessível na rede local pelo endereço amigável `http://<app>.local`, abstraindo completamente o IP do Raspberry Pi.

### Fluxo de Descoberta e Registro
1. **Identificação da Rede:** A pipeline aciona a descoberta programática (`GetLocalIP`), que varre as interfaces de rede do servidor para encontrar o primeiro endereço IPv4 viável e não-loopback.
2. **Formatação do Domínio:** Cria-se o mapeamento textual exato do IP descoberto apontando para `<app>.local`.
3. **Persistência Direta (Idempotente):** O Amora abre o arquivo de configuração estático do Avahi (`/etc/avahi/hosts`), faz o parse linha a linha, limpa eventuais lixos, encontra a linha correspondente (se já existir) e atualiza o IP (caso tenha mudado via DHCP). Se não existir, ele simplesmente anexa.
4. **Remoção Limpa:** Quando o comando `amora destroy` é acionado, ou o usuário remove a flag `web` do seu `Procfile`, o motor de deploy limpa o registro, lendo o mesmo arquivo e deletando estritamente a linha referente ao domínio daquele app.

---

## 2. Estrutura de Código Atual

A implementação reside exclusivamente no pacote `internal/mdns/`.

### Arquivo Core: `avahi.go`

#### Funções de Rede
- `GetLocalIP() (string, error)`:
  - Consome a API da Standard Library `net.InterfaceAddrs()`.
  - Ignora endereços de loopback (`ipnet.IP.IsLoopback()`).
  - Ignora endereços IPv6 (`ipnet.IP.To4() == nil`), retornando o IP padrão primário em que o Avahi estará escutando e fazendo os broadcastings mDNS.

#### Funções Mutadoras (File System)
- `GenerateService(app string) error`:
  - Encontra o IP local.
  - Formata a string `192.168.x.x meuapp.local`.
  - Realiza a leitura e reescrita inteligente do arquivo `/etc/avahi/hosts`, protegendo contra duplicações.
- `RemoveService(app string) error`:
  - Exclui a linha `<ip> <app>.local` preservando o restante do arquivo intacto. Se o arquivo já não possuir a entrada ou não existir, o método é um "no-op" sem retornar erros.

---

## 3. Contratos e Regras de Negócio Cruciais (Pivô Arquitetônico)

**ATENÇÃO AGENTES DE IA E FUTUROS DESENVOLVEDORES:**

A decisão de manipular diretamente o arquivo estático `/etc/avahi/hosts` em oposição a criar serviços executando o comando CLI oficial `avahi-publish` (ou `avahi-publish-service`) foi um **pivô arquitetônico severamente intencional**.

*Por que não usar o CLI oficial no formato de um daemon Systemd (`amora-<app>-mdns.service`)?*
1. **Restrições de D-Bus e Systemd `--user`:** O CLI do `avahi-publish` exige comunicação com o daemon do Avahi, o que roda em um *bus de sistema* privilegiado. Como a restrição arquitetural número 1 do Amora é rodar sob `systemd --user` (sem privilégios de root, totalmente confinado ao usuário), a comunicação entre processos de usuário e serviços de sistema no D-Bus constantemente falhava por problemas de permissão.
2. **Uso Exagerado de Recursos:** Manter um processo shell em background rodando um `avahi-publish` de forma espelhada para cada aplicação consumia memória e IDs de processo valiosos e desnecessários em um Raspberry Pi (Zero Overhead Policy).

**O Contrato Inviolável:**
A resolução deve ser feita **somente por leitura/escrita do arquivo estático**. O Daemon principal do Avahi tem inotify atrelado ao `/etc/avahi/hosts` por design, ou seja, ao alterarmos o arquivo via I/O do Go, o Avahi absorve a alteração nativamente em milisegundos e faz o reload automático sem precisarmos reestartar o daemon principal, cumprindo todas as nossas necessidades de segurança e alocação de recursos.

---

## 4. Casos de Teste Sugeridos

Ao manter ou refatorar o `avahi.go`, o foco deve ser na idempotência da manipulação do arquivo de hosts e na descoberta robusta da placa de rede.

### Testes de Descoberta de IP
- **Fallback e Múltiplas Placas:** Usar interfaces falsas (Mock) para validar que a função extrai estritamente endereços `IPv4` não-loopback. A função não deve quebrar caso a máquina possua sub-redes virtuais (ex: docker0, tun0), capturando a interface de menor métrica primária.

### Testes de Parsing e Idempotência (Mockando `/etc/avahi/hosts`)
- **Adição Inédita:** Criar um arquivo mock temporário vazio, passar a função de `Generate` e checar a estrutura textual exata do sufixo `.local`.
- **Prevenção de Duplicação e Update de IP (DHCP):** 
  - Pré-popular o arquivo mock com `192.168.0.10 app.local`.
  - Executar a função com a placa de rede reportando um IP novo `192.168.0.25`.
  - Assertividade: O arquivo deve conter exatamente uma linha de `app.local`, substituída com o novo IP `192.168.0.25`. A função não deve anexar uma segunda entrada no final do arquivo.
- **Múltiplos Domínios:** O arquivo possui `app1.local`, `app2.local` e comentários `# header`. O `RemoveService("app1")` não deve destruir `app2.local` nem bagunçar a formatação pré-existente (trimming excessivo de quebras de linha).
