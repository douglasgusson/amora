# Spec 006: CLI de Administração e Setup

## 1. Objetivo e Fluxo
O módulo da Interface de Linha de Comando (CLI) centraliza não só a usabilidade local do desenvolvedor, mas também o setup inicial da infraestrutura no servidor de destino. Através de comandos explícitos e sem estado (`stateless`), a CLI abstrai configurações complexas de SSH, cross-compilação, systemd e roteamento num modelo amigável de Developer Experience (DX).

### Fluxos Principais
1. **Provisionamento (Remote Setup):** O comando `amora provision pi@...` é disparado na máquina local do desenvolvedor. O Amora injeta um shell script idempotente no Raspberry Pi via SSH instalando Caddy, Avahi, configurando o usuário `amora` (com persistência de processos via *Linger*) e instalando o `mise`. Logo após, o CLI na máquina do dev cruza a compilação do próprio Amora para `GOOS=linux GOARCH=arm64`, envia via `scp` e o instala em `/usr/local/bin/amora`.
2. **Criação e Setup de App:** Uma vez no Pi (via SSH), o usuário invoca `amora create --app <app>`. A CLI cria o ecossistema necessário: repositório Git bare, a pasta de working tree (apps) e, mais criticamente, implanta o script `post-receive` na pasta `.git/hooks` apontando para o próprio binário do Amora.
3. **Observabilidade (Logs):** A visualização de logs é abstraída do Linux, usando `amora logs <app>` que empacota uma requisição para o `journalctl --user-unit`, repassando formatação de tela para facilitar debug.
4. **Descomissionamento (Destroy):** Exclui rigorosamente as pastas do repo, do app, segredos e engatilha o apagão via Systemd Generator para remover serviços e derrubar rotas do proxy/mDNS.

---

## 2. Estrutura de Código Atual

A base da CLI é construída no pacote robusto e extensível `github.com/spf13/cobra`.

### Entrypoint e Framework
- **Arquivo:** `cmd/amora/main.go`
  - Minimalista. Apenas evoca `cli.NewRootCmd().Execute()` e faz os exits de erro fundamentais (0 ou 1).
- **Pacote:** `internal/cli/`
  - Contém o agrupamento e o binding dos comandos. Cada arquivo é um subcomando direto: `create.go`, `provision.go`, `destroy.go`, `logs.go`, `apps.go`.

### Módulo Visual (DX)
- **Arquivo:** `colors.go`
  - Mantém funções padronizadas (`LogInfo`, `LogSuccess`, `LogError`) usando sequências de Escape ANSI (magenta, cyan, red, green) e padronização visual com prefixos lógicos (`----->`, `✓`, `!`).

---

## 3. Contratos e Regras de Negócio Cruciais

1. **Provisionamento Idempotente:**
   - **Regra:** O `amora provision` executa um block shell `setupScript`. Cada passo dentro desse block foi desenhado para não quebrar caso rode 10x seguidas no mesmo host. 
   - A criação do usuário `amora` é envelopada num condicional (`if ! id -u amora...`). 
   - O appending do `mise activate` no `~/.bashrc` é precedido por um `grep -q "mise activate"` prevenindo múltiplas repetições indesejadas no bashrc ao provisionar.

2. **Linger Mode Obrigatório:**
   - **Regra:** No Provision, a execução de `sudo loginctl enable-linger amora` é **inviolável**. Sem essa diretriz do Linux, sempre que o usuário deslogar do SSH, todos os processos de background atrelados à conta do `amora` morrerão (derrubando todos os apps simultaneamente).

3. **Arquitetura Bare Git (O Coração da Criação):**
   - No `amora create`, não há clone. Ele inicia `git init --bare` na pasta `~/repos/<nome>.git`. 
   - Imediatamente, ele cria o arquivo executável (`0755`) `hooks/post-receive`.
   - O conteúdo desse hook usa a função `os.Executable()` para auto-descobrir onde o amora foi instalado na máquina e cria a string blindada que disparará o comando core: `/usr/local/bin/amora hook post-receive --app <app>`.

4. **Transparência de Logs:**
   - **Regra:** O Amora não tenta recriar um coletor de logs nativo. Ele tira proveito total do utilitário `journald` do host.
   - Quando `amora logs [app]` é invocado, ele varre os serviços com o prefixo da aplicação, e executa: `journalctl --user-unit=amora-<app>-<process>.service -f -n 100`. Os descritores de stdout/stderr são atrelados diretamente no terminal do host, sem buffers escondidos.

---

## 4. Casos de Teste Sugeridos

Testar a CLI do Cobra se torna prático pelo mock dos streams de entrada/saída (stdin/stdout) e validação dos inputs antes deles chamarem as mutações em si.

### Parsing de Flags e Validação
- **Obrigatoriedade de Flags:** Chamar `NewCreateCmd()` ou `NewDestroyCmd()` testando a submissão via `cmd.Execute()` passando flags vazias ou sem flags e esperar que as validações originais do cobra ou ifs internos proíbam o avanço (`--app flag is required`).

### Operações em File System (Setup e Mocks)
- Injetar o `os.UserHomeDir()` para um diretório simulado `/tmp/amora_test_home`. Rodar `amora create --app test-app` e validar fisicamente no fim do teste se as pastas `repos/test-app.git`, `apps/test-app` e se o script `/hooks/post-receive` foram geradas e contêm a chave do amora.

### Invocação de Sub-processos (Logs e Runner)
- Usando a interface `MockRunner` ou captando processos instanciados por `exec.Command`, validar se chamar `amora logs appzinho` realmente monta um slice ordenado no terminal exato: `["journalctl", "--user-unit=amora-appzinho...", "-f", "-n", "100"]`. Validando que a abstração está injetando o comando seguro e não repassando injeções shell vindas do input do usuário.
