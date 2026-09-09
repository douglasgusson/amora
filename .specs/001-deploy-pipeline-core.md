# Spec 001: Deploy Pipeline & Git Hook Core

## 1. Objetivo e Fluxo
A pipeline de deploy é a funcionalidade central do Amora, desenhada para fornecer uma experiência de deploy contínuo tipo Heroku em um Raspberry Pi. Todo o processo é disparado via um _git hook_ (`post-receive`) no repositório *bare* da aplicação.

### O Ciclo de Vida do Deploy
1. **Trigger via Git:** Ao fazer um `git push` para o repositório do Amora (ex: `~/repos/<app>.git`), o git invoca o hook `post-receive`.
2. **Leitura do Stdin:** O binário do Amora (rodando via `amora hook post-receive --app <app>`) escuta o `stdin` para capturar a branch, a hash do commit antigo e do commit novo.
3. **Checkout do Código:** Usando `--work-tree` e `--git-dir`, o código é extraído do repositório bare para a pasta de trabalho da aplicação (`~/apps/<app>`).
4. **Provisionamento e Build (Runtime):**
   - Roda `mise install` para garantir as versões dos runtimes (Node, Go, Python, etc.) definidas no projeto.
   - Procura pelo script `amora-build`. Se existir, injeta permissão de execução (`chmod 0755`) e o executa encapsulado no runtime correto via `mise exec -- ./amora-build`.
5. **Parsing do Procfile:** Avalia o arquivo `Procfile` para definir quais processos serão executados (ex: `web`, `worker`).
6. **Alocação de Porta e Variáveis (Zero-Database):** Para processos que requerem rede (ex: `web`), a pipeline checa os arquivos `.env` do ambiente. Se nenhuma `PORT` estiver alocada, ela busca a maior porta usada entre todos os `.env` dos apps, soma 1, salva no `.env` do app atual e a retorna.
7. **Geração de Serviços Systemd:** Converte os processos do Procfile em definições de `systemd user services`, injetando as variáveis do `.env` e montando o comando final encapsulado no `mise exec`.
8. **Configuração de Proxy e mDNS:** Se um processo `web` for identificado, gera a configuração do Caddy (`<app>.caddyfile`) com roteamento reverso para a porta alocada, além de registrar no Avahi (`/etc/avahi/hosts`) o domínio `<app>.local`.
9. **Finalização:** Faz o `daemon-reload`, reinicia os serviços Systemd e informa ao usuário o resultado com links para o acesso HTTP.

---

## 2. Estrutura de Código Atual

### Pacote Core: `internal/cli`
- **Arquivo:** `hook.go`
- **Struct Principal:** `DeployPipeline`
  - Armazena as dependências injetadas (IoC) como: `Runner` (interface para comandos shell), `EnvMgr` (gerenciador de `.env`), `Systemd` (gerador de scripts Systemd), etc.
- **Funções Chave:**
  - `NewHookCmd()`: Retorna o subcomando cobra `post-receive`.
  - `(p *DeployPipeline) Run(app string)`: Método core que orquestra as 8 fases do ciclo de deploy.
  - `parsePushInfo(r io.Reader)`: Lê os argumentos `old-sha new-sha ref` enviados pelo git via `stdin`.

### Pacote de Parsing/Deploy: `internal/deploy`
- **Arquivo:** `procfile.go`
- **Struct:** `ProcEntry` (guarda `Name` e `Command`).
- **Funções Chave:**
  - `ParseProcfile(path string)`: Lê e divide o conteúdo usando `:` como separador.

### Pacote Shell: `internal/deploy`
- **Arquivo:** `runner.go`
- **Struct:** `RealRunner` e `MockRunner`.
- Encarregado de rodar processos no host mapeando adequadamente o `stdout/stderr` para o log colorido (prefixo cinza).

### Pacote de Ambientes: `internal/env`
- **Arquivo:** `env.go`
- **Struct Principal:** `Manager`
- **Funções Chave:**
  - `GetOrAssignPort(app string) (int, error)`: A lógica de alocação de portas baseada unicamente no File System.

---

## 3. Contratos e Regras de Negócio

1. **Procfile:**
   - Obrigatório para definir comandos.
   - Formato: `<tipo>: <comando>`. (Exemplo: `web: npm run start:prod`).
   - Linhas em branco ou iniciadas com `#` são solenemente ignoradas.
   - O tipo de processo dita o roteamento. Se houver o tipo **`web`**, a pipeline habilita mDNS (Avahi) e Caddy (Proxy Reverso). Sem `web`, o deploy é tratado como um *Worker Headless*.

2. **O Script \`amora-build\`:**
   - É o mecanismo oficial de pre-start hook (ex: compilar TypeScript, rodar assets).
   - Se existir na raiz, a pipeline **obriga** sua execução aplicando `chmod 0755` antes de chamar o shell.
   - É encapsulado: `/home/amora/.local/bin/mise exec -- ./amora-build`.
   - Se este passo falhar, o deploy é abortado (não regera systemd ou caddy).

3. **Alocação de Porta (Port Allocation):**
   - Porta Base: `5000`.
   - Busca no diretório `~/.amora/envs/` por todos os `.env` existentes.
   - Varre o arquivo atrás de `PORT=X`.
   - Retorna o maior valor de porta + 1. Se a aplicação já tiver uma `PORT` em seu arquivo `.env`, essa porta é mantida (idempotência).

4. **Isolamento de Processo (Mise):**
   - Todos os comandos contidos no Procfile, e também o comando `amora-build`, nunca rodam crus.
   - Eles são inseridos no unit file do Systemd sob: `/home/amora/.local/bin/mise exec -- /bin/bash -c '<comando>'`. Isso garante isolamento.

---

## 4. Casos de Teste Sugeridos

Ao evoluir esta feature, os seguintes cenários devem ter sua estabilidade garantida (tanto em testes E2E quanto via `MockRunner` e `MockSystemd`):

### Cenários de Sucesso
- **Deploy Básico com Web:** Um app cujo Procfile só contenha `web: node index.js`. Deve ser provida uma `PORT`, `.caddyfile` e injetada no Systemd com `mise exec`.
- **Worker Headless:** App sem `web` (ex: `worker: python cron.py`). A porta não deve ser alocada, o Caddy/Avahi devem ignorar esse app e os serviços criados com sucesso.
- **Transição de Web para Worker:** Um app antes tinha processo `web`, mas num novo deploy o removeu. A pipeline deve remover o `.caddyfile` e limpar a referência do `.local` no AvahiHosts.
- **Idempotência de Porta:** Rodar deploy de um mesmo app dezenas de vezes não deve incrementar a `PORT` se já houver um registro salvo em seu `.env`.

### Cenários de Falha (Circuit Breakers)
- **Erro de Git Push vazio:** Ausência de inputs no `stdin` (quando chamado o post-receive incorretamente) deve abortar graciosamente.
- **Procfile Ausente ou Vazio:** Falha na leitura do arquivo ou arquivo vazio deve invalidar e interromper o deploy imediatamente (pois sem Procfile, não há processo).
- **Formato Inválido no Procfile:** Linhas como `web npm start` (sem dois pontos `:`) devem disparar um alerta e falhar na checagem.
- **Falha no Build:** Se o script `./amora-build` retornar exit code != 0, a esteira aborta para proteger a versão anterior (os units do systemd não devem ser recriados/reiniciados).
