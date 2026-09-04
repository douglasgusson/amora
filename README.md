# 🍇 Amora — Micro-PaaS para Raspberry Pi

O **Amora** é um Micro-Platform as a Service (PaaS) leve e open-source, focado em Developer Experience (DX) e inspirado no Heroku, projetado para rodar em dispositivos ARM64 como o Raspberry Pi.

Deploy automatizado via `git push`, múltiplas linguagens (via `mise`), variáveis de ambiente dinâmicas, logs em tempo real, proxy reverso automatizado e descoberta na rede local (mDNS) — tudo sem Docker.

---

## 🚀 Quickstart — Do zero ao deploy em 5 minutos

### Pré-requisitos

- Um **Raspberry Pi** com Raspberry Pi OS (64-bit / ARM64) acessível via SSH
- Um **Mac/Linux** de desenvolvimento com Go 1.22+ instalado

### 1. Provisionar o Raspberry Pi

No seu Mac ou Linux, baixe e instale a CLI do Amora com um único comando:

```bash
curl -sL https://raw.githubusercontent.com/douglasgusson/amora/main/install.sh | bash
```

Em seguida, provisione o Pi (instala tudo automaticamente):

```bash
amora provision pi@raspberrypi.local
```

Esse comando conecta via SSH, instala as dependências base (`git`, `curl`, `caddy`, `avahi-daemon`, `build-essential`), cria o usuário `amora`, configura o `mise`, habilita o `systemd linger` e envia o binário compilado para o Pi.

### 2. Criar uma aplicação no Pi

Conecte no Pi como o usuário `amora` e crie o app:

```bash
ssh amora@raspberrypi.local

# Inicializar o ambiente (roda apenas uma vez)
amora setup

# Criar a aplicação
amora create --app meu-app
```

O comando `create` inicializa um repositório Git bare, instala o hook de deploy e atribui uma porta única (a partir de 5000).

### 3. Preparar o seu projeto

Na raiz do seu projeto, crie dois arquivos:

**`Procfile`** — define os processos:
```
web: node server.js
```

**`amora-build`** *(opcional)* — script de build:
```bash
#!/bin/bash
npm install
```

Se você usa Node.js, crie também um **`.node-version`**:
```
20.11.0
```

### 4. Fazer o deploy

De volta ao seu Mac, adicione o remote e faça push:

```bash
git remote add amora amora@raspberrypi.local:repos/meu-app.git
git push amora main
```

O terminal vai exibir todo o pipeline em tempo real:

```
🍇 Amora

-----> Deploying 'meu-app'...
-----> Checking out code...
       ✓ Code checked out to /home/amora/apps/meu-app
-----> Provisionando runtimes (mise)...
-----> Executando amora-build...
       ✓ Build concluído
-----> Reading Procfile...
       ✓ Process: web → node server.js
-----> Generating systemd services...
       ✓ Generated amora-meu-app-web.service
-----> Configuring reverse proxy...
       ✓ Caddyfile: meu-app.local → localhost:5000
-----> Deploy complete! 🎉

  🌐 http://meu-app.local
  📡 http://192.168.1.120:5000
  🔌 http://localhost:5000
```

### 5. Acessar a aplicação

Abra o navegador em qualquer dispositivo da rede local:

```
http://meu-app.local
```

---

## 💻 CLI — Referência de Comandos

### `amora provision [user@host]`

Prepara um Raspberry Pi do zero. Roda no **Mac/Linux** do desenvolvedor.

- Instala dependências via `apt-get` (apenas ferramentas vitais — sem Node/Python)
- Cria o usuário `amora` com SSH configurado
- Instala o `mise` isolado em `/home/amora/.local/bin/mise`
- Habilita `loginctl enable-linger amora`
- Cross-compila (`GOOS=linux GOARCH=arm64`) e envia o binário via SCP
- **Idempotente**: pode rodar múltiplas vezes sem quebrar

```bash
amora provision pi@raspberrypi.local
```

### `amora setup`

Inicializa a estrutura de diretórios no Pi. Roda **dentro do Pi** como usuário `amora`.

```bash
amora setup
```

### `amora create --app <nome>`

Cria uma nova aplicação no Pi.

- Inicializa repositório Git bare em `~/repos/<nome>.git`
- Instala o hook `post-receive` que aciona o pipeline de deploy
- Atribui uma porta única sequencial (5000, 5001, ...)
- Cria o diretório de trabalho em `~/apps/<nome>`

```bash
amora create --app blog
```

### `amora env`

Gerenciamento de variáveis de ambiente (12-Factor App).

```bash
# Definir variáveis (reinicia o serviço automaticamente)
amora env set blog PORT=5000 NODE_ENV=production DATABASE_URL=postgres://...

# Listar variáveis
amora env ls blog

# Remover uma variável (reinicia o serviço automaticamente)
amora env rm blog DATABASE_URL
```

As variáveis são armazenadas em `~/.amora/envs/<app>.env` e injetadas no processo via diretiva `EnvironmentFile` do systemd — sem dependência de pacotes como `dotenv`.

### `amora logs <app> [-f] [-n <linhas>]`

Visualização de logs em tempo real.

```bash
# Últimas 50 linhas (padrão)
amora logs blog

# Acompanhar em tempo real (Ctrl+C para sair)
amora logs blog -f

# Últimas 200 linhas
amora logs blog -n 200
```

Wrapper do `journalctl --user-unit=amora-<app>-web.service` com stdout/stderr conectados diretamente ao terminal.

---

## 🏗️ Arquitetura e Tech Stack

```
┌─────────────────────────────────────────────────────────────┐
│  Mac/PC do Desenvolvedor                                    │
│                                                             │
│  git push amora main ──────────────────────┐                │
│  amora provision pi@...                    │                │
│  amora env set app KEY=VALUE               │                │
│  amora logs app -f                         │                │
└────────────────────────────────────────────┼────────────────┘
                                             │ SSH / Git
┌────────────────────────────────────────────▼────────────────┐
│  Raspberry Pi (ARM64)                                       │
│                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │  Git (bare)  │→ │ Amora (Go)   │→ │ systemd --user   │  │
│  │  post-receive│  │ Orquestrador │  │ amora-app-web    │  │
│  └──────────────┘  └──────┬───────┘  └────────┬─────────┘  │
│                           │                    │            │
│  ┌──────────────┐  ┌──────▼───────┐  ┌────────▼─────────┐  │
│  │  mise        │  │ Caddy Server │  │ Avahi (mDNS)     │  │
│  │  Runtimes    │  │ :80 → :5000  │  │ app.local → IP   │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

| Componente | Tecnologia | Função |
|------------|-----------|--------|
| **Orquestrador** | Go (Golang) | CLI + pipeline de deploy |
| **Processos** | `systemd --user` | Init system, restarts, journald |
| **Proxy Reverso** | Caddy Server | Roteia `:80` → porta do app via API |
| **mDNS** | Avahi | Broadcast de `<app>.local` na rede |
| **Runtimes** | mise | Node.js, Python, Ruby, etc. isolados |

---

## 🔄 Pipeline de Deploy (detalhado)

Ao executar `git push amora main`, o seguinte pipeline é executado no Pi:

1. **Git Post-Receive Hook** — aciona `amora hook post-receive --app <app>`
2. **Checkout** — extrai o código para `/home/amora/apps/<app>`
3. **Runtime Provisioning** — executa `mise install` (respeita `.node-version`, `.python-version`, `.mise.toml`)
4. **Build Phase** — se existir `amora-build` na raiz, recebe `chmod 0755` e é executado via `mise exec -- ./amora-build`
5. **Procfile Parsing** — lê processos definidos (ex: `web: node server.js`)
6. **Systemd Generation** — cria units em `~/.config/systemd/user/amora-<app>-<processo>.service`
7. **Proxy Routing** — gera Caddyfile e recarrega via API (`POST localhost:2019/load`)
8. **mDNS Setup** — registra `<app>.local` no `/etc/avahi/hosts`
9. **Restart** — `daemon-reload` + `enable` + `restart` dos serviços

### Contratos do Repositório

| Arquivo | Obrigatório | Descrição |
|---------|:-----------:|-----------|
| `Procfile` | ✅ | Define processos. Formato: `tipo: comando` |
| `amora-build` | ❌ | Script de build (qualquer linguagem via shebang) |
| `.node-version` | ❌ | Versão do Node.js para o mise provisionar |
| `.python-version` | ❌ | Versão do Python para o mise provisionar |
| `.mise.toml` | ❌ | Configuração avançada do mise (múltiplos runtimes) |

---

## 🏛️ Decisões Arquitetônicas (ADRs)

### 1. Systemd em vez de Docker

O Docker consome muitos recursos em dispositivos limitados. O `systemd --user` oferece overhead zero — processos rodam nativamente sob o usuário `amora`. O *Linger* (`loginctl enable-linger amora`) garante que os serviços sobrevivam ao logout da sessão SSH.

### 2. Pivô do mDNS — De `avahi-publish` para `/etc/avahi/hosts`

Inicialmente, o Amora publicava domínios `.local` via serviço systemd executando `avahi-publish`. O Linux barrava a execução (Exit Status 5) pois processos `systemd --user` não têm permissão para se comunicar com o System D-Bus (sandboxing Polkit).

**Solução**: O Go descobre o IP local e escreve diretamente no `/etc/avahi/hosts`. O daemon do Avahi detecta a mudança via *inotify* e faz o broadcast automaticamente. Se o IP mudar (DHCP), o Amora atualiza a entrada existente em vez de duplicá-la.

### 3. Pivô de Runtimes — De `apt-get` global para `mise` isolado

Instalar runtimes via `apt-get install nodejs` gera conflitos quando apps precisam de versões diferentes. O `mise` é instalado exclusivamente na pasta do usuário `amora` (`/home/amora/.local/bin/mise`).

Cada serviço systemd envelopa o processo com mise:
```ini
ExecStart=/home/amora/.local/bin/mise exec -- /bin/bash -c 'node server.js'
```

Isso garante que o processo herda o runtime correto sem poluir o sistema.

### 4. Build delegado ao repositório (`amora-build`)

O PaaS não precisa conhecer os comandos de build de cada linguagem. O contrato é simples: se existir um arquivo `amora-build` na raiz do repositório, ele será executado. O script pode ter qualquer shebang (`#!/bin/bash`, `#!/usr/bin/env python3`, etc.).

---

## 📂 Estrutura de Diretórios no Pi

```text
/home/amora/
├── .amora/
│   └── envs/                    # Variáveis de ambiente (<app>.env)
├── apps/
│   └── meu-app/                 # Checkout do código fonte
├── repos/
│   └── meu-app.git/             # Repositório bare (recebe git push)
│       └── hooks/post-receive   # Hook que aciona o pipeline
├── caddy/
│   └── meu-app.caddyfile        # Snippet de roteamento por app
├── .config/systemd/user/
│   └── amora-meu-app-web.service  # Unit gerada dinamicamente
└── .local/bin/
    └── mise                     # Runtime manager isolado
```

---

## 🛠️ Desenvolvimento

### Requisitos

- Go 1.22+
- Raspberry Pi com SSH acessível (para testes reais)

### Build local

```bash
go build -o amora ./cmd/amora
```

### Testes

```bash
go test ./...
```

### Deploy do binário para o Pi (dev)

```bash
./deploy-dev.sh
```

---

## 📄 Licença

Este projeto é open-source. Contribuições são bem-vindas!
