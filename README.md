# 🍇 Amora - Micro-PaaS para Raspberry Pi

O **Amora** é um Micro-Platform as a Service (PaaS) leve, focado em Developer Experience (DX) e inspirado no Heroku e Vercel, projetado para rodar perfeitamente em dispositivos limitados como o Raspberry Pi. 

Ele permite o deploy automatizado de aplicações usando um simples `git push`, suportando múltiplas linguagens (via `mise`), configuração de variáveis de ambiente dinâmicas, logs em tempo real, proxy reverso automatizado e descoberta de rede local (mDNS) sem necessidade de dependências pesadas como o Docker.

---

## 🏗️ Arquitetura e Tech Stack

O Amora baseia-se fortemente em recursos nativos do ecossistema Linux moderno, coordenados por um orquestrador escrito em **Go (Golang)**.

*   **Orquestrador (CLI/Hook):** Escrito em Go, atua como o cérebro da plataforma. Ele recebe o git push, compila, lê configurações e gerencia os serviços do sistema.
*   **Processos (Init System):** `systemd --user`. Roda os aplicativos de forma resiliente, lidando com restarts e logs (`journald`), sem necessidade de permissões `root` ou de containers pesados.
*   **Proxy Reverso:** `Caddy Server`. Roteia o tráfego da porta 80 para a porta alocada para cada aplicação de forma dinâmica via API.
*   **mDNS (Rede Local):** `Avahi`. Responsável por mapear e fazer o broadcast dos domínios customizados (ex: `http://app-teste.local`) na rede Wi-Fi local.
*   **Gerenciador de Runtimes:** `mise`. Instala e isola dinamicamente versões específicas de linguagens (Node.js, Python, etc.) solicitadas por cada aplicação (ex: `.node-version`), sem sujar o sistema operacional global.

---

## 🚀 O Ciclo de Vida do Deploy (Pipeline)

Quando um desenvolvedor executa `git push amora main` no seu Mac/PC, a seguinte cadeia de eventos ocorre em milissegundos no Raspberry Pi:

1.  **Git Post-Receive Hook:** O Git recebe o código e engatilha o binário do Amora.
2.  **Checkout:** O código-fonte é extraído para `/home/amora/apps/<app>`.
3.  **Runtime Provisioning (`mise`):** O Amora detecta arquivos como `.node-version` ou `.mise.toml`, baixa e instala o runtime da linguagem estritamente na pasta do usuário `amora`.
4.  **Build Phase (`amora-build`):** Se houver um script `amora-build` na raiz (ex: `npm install`), ele é executado usando o ambiente provisionado.
5.  **Procfile Parsing:** O arquivo `Procfile` é lido para mapear os processos necessários (ex: `web: node server.js`).
6.  **Systemd Generation:** Arquivos de serviço (`amora-<app>-web.service`) são criados dinamicamente em `~/.config/systemd/user/`.
7.  **Proxy Routing:** O Caddy é reconfigurado para rotear o tráfego do domínio local (ex: `<app>.local`) para a porta da aplicação.
8.  **mDNS Setup:** O domínio `<app>.local` é atrelado ao IP do Raspberry Pi no serviço Avahi.
9.  **Restart:** O serviço systemd reinicia a aplicação sob a nova versão do código com zero-downtime percebido.

---

## 💻 Comandos e CLI

O Amora possui uma CLI poderosa cross-platform (roda no Mac para administrar remotamente, ou dentro do Pi).

### `amora provision [user@host]`
Prepara um Raspberry Pi "cru" de forma automatizada (Zero Setup).
*   **O que faz:** Conecta via SSH, atualiza dependências do `apt-get`, instala o Caddy, Avahi, configura chaves SSH, cria o usuário restrito `amora`, habilita o *Linger* do systemd, instala o `mise`, compila cruzado o próprio Amora para `ARM64` no host e envia o binário pronto para uso.

### `amora env`
Gerenciamento de variáveis de ambiente estilo 12-Factor App.
*   `amora env set <app> KEY=VALUE`: Define a variável, salva em um arquivo blindado e **reinicia** automaticamente o serviço no Pi.
*   `amora env ls <app>`: Lista todas as variáveis ativas do app.
*   *Implementação Técnica:* O Amora gera serviços systemd com a diretiva `EnvironmentFile=-/home/amora/.amora/envs/<app>.env`, fazendo com que o próprio Kernel Linux injete as variáveis nativamente no processo (eliminando pacotes como `dotenv`).

### `amora logs [app] [-f]`
Visualização unificada de logs.
*   **O que faz:** Conecta as saídas padrão e de erro do processo remoto diretamente no terminal do desenvolvedor.
*   *Implementação Técnica:* É um wrapper do comando `journalctl --user -u amora-<app>-web.service`. A flag `-f` suporta *tailing* (acompanhamento em tempo real).

---

## 🏛️ Decisões Arquitetônicas e Evolução (ADRs)

Ao longo do desenvolvimento do Amora, decisões cruciais foram tomadas para balancear segurança, performance (CPU/RAM limitados no Pi) e Developer Experience.

### 1. Systemd vs Docker
*   **Problema:** O Docker consome muitos recursos (RAM e CPU) em dispositivos limitados como o Raspberry Pi.
*   **Decisão:** Utilizar o sistema de init nativo do Linux (`systemd --user`).
*   **Benefício:** Overhead zero. Processos rodam de forma nativa e isolada sob o usuário `amora`. Habilitamos o *Linger* (`loginctl enable-linger amora`) para que os aplicativos continuem rodando após o logout da sessão SSH.

### 2. O Pivô do mDNS (Avahi)
*   **Problema:** Inicialmente, o Amora tentou publicar os domínios `.local` criando um serviço systemd que rodava o comando `avahi-publish`. O Linux barrou a execução (Exit Status 5) pois processos em background do `systemd --user` não possuem permissão para se comunicar com o *System D-Bus* por razões estritas de segurança do Debian/Polkit.
*   **Decisão (Pivô):** Substituir a chamada ativa de D-Bus por edição de arquivo estático. O orquestrador em Go agora descobre o IP da máquina e anexa uma entrada diretamente no arquivo `/etc/avahi/hosts`.
*   **Benefício:** O daemon central do Avahi monitora o arquivo via *inotify* e faz o broadcast instantâneo na rede local, eliminando dezenas de instâncias rodando em background e mitigando os bloqueios de segurança do sistema operacional.

### 3. O Pivô de Runtimes (Global vs Mise)
*   **Problema:** Instalar dependências globais via `apt-get` (ex: `sudo apt install nodejs`) gerava o "risco do canivete suíço". Aplicações diferentes precisariam de versões diferentes do Node.js ou Python, gerando conflitos.
*   **Decisão (Pivô):** Implementar o `mise` como gerenciador dinâmico de linguagens, instalado exclusivamente na pasta do usuário `amora`.
*   **Benefício:** Cada repositório dita suas dependências no arquivo de versão (ex: `.node-version`). O `systemd` então é configurado com uma chamada envelopada no executável, garantindo isolamento total (ex: `ExecStart=/home/amora/.local/bin/mise exec -- node server.js`).

### 4. A Abstração de Build (`amora-build`)
*   **Problema:** O PaaS (Go) não deveria precisar conhecer os comandos específicos de build de todas as linguagens (Go, Rust, Node, Python).
*   **Decisão:** Delegação de responsabilidade. O projeto estabeleceu o contrato do arquivo executável `amora-build` na raiz do repositório.
*   **Benefício:** Se o arquivo existir, o Amora o executa com saída em tempo real no terminal do usuário (`npm install`, `pip install`, `go build`). O Amora se mantém agnóstico; o repositório é dono do seu próprio pipeline de build.

---

## 📂 Estrutura de Diretórios no Raspberry Pi

A infraestrutura mantida pelo Amora sob o usuário `/home/amora/` segue o seguinte padrão:

```text
~/.amora/
  ├── envs/               # Arquivos .env segregados por aplicação (<app>.env)
~/apps/                   # Diretório de checkout das aplicações via Git
  ├── app-teste/          # Aplicação rodando
~/repos/                  # Repositórios Git (bare) que recebem o `git push`
  ├── app-teste.git/      # Repositório remoto onde o hook do Amora reside
~/.config/systemd/user/   # Diretório nativo de units do systemd
  ├── amora-app-teste-web.service # Unit gerada dinamicamente
~/.local/bin/             # Ferramentas isoladas do usuário amora (mise)
