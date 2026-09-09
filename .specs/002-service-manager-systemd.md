# Spec 002: Service Manager (Systemd Generator)

## 1. Objetivo e Fluxo
O Service Manager é o motor responsável por abstrair a execução contínua das aplicações no servidor. Ele traduz as definições efêmeras de um `Procfile` e os segredos do `.env` em serviços estáticos robustos gerenciados pelo Systemd do sistema operacional do host.

### Fluxo de Geração e Ciclo de Vida
1. **Entrada de Dados:** O módulo recebe uma configuração (struct `ServiceConfig`) contendo: nome do app, tipo de processo (ex: `web`, `worker`), pasta de trabalho (workdir), comando shell e o caminho do arquivo `.env`.
2. **Geração do Unit File:** Os dados populam um template textual e um arquivo `.service` é escrito no diretório de serviços do usuário local (por padrão, `~/.config/systemd/user/`). O arquivo gerado segue a nomenclatura `amora-<app>-<process>.service`.
3. **Atualização do Daemon:** O subcomando `systemctl --user daemon-reload` é invocado para notificar o Systemd sobre novos arquivos ou modificações em serviços existentes.
4. **Ativação e Boot:** O serviço é ativado para iniciar junto com o sistema operacional (`systemctl --user enable`) e, logo em seguida, o estado é consolidado efetuando um `restart` que garante a execução da última versão do código (seja num novo deploy ou numa alteração de variáveis).
5. **Limpeza e Teardown:** Em caso de destruição de um ambiente (`amora destroy`), o motor encontra iterativamente todos os serviços com o prefixo do app, os para, desativa e, finalmente, os remove do disco chamando um reload subsequente.

---

## 2. Estrutura de Código Atual

O motor reside inteiramente dentro do pacote `internal/systemd/`.

### Arquivo Core: `service.go`

#### Estruturas Principais
- **`ServiceConfig`:** Define os inputs necessários (App, Process, WorkDir, Command, EnvFile).
- **`Generator`:** A struct que executa o trabalho. Ela contém a dependência `BaseDir` (onde salvar os arquivos) e `RunCmd` (uma função anônima que executa binários, permitindo injeção de dependências em testes para não invocar o `systemctl` real na máquina do desenvolvedor).

#### O Template Go (Constante `ServiceTemplate`)
O arquivo utiliza o pacote `text/template` da biblioteca padrão para criar uma unidade idempotente:
```ini
[Unit]
Description=Amora: {{.App}} ({{.Process}})
After=network.target

[Service]
Type=simple
WorkingDirectory={{.WorkDir}}
ExecStart=/home/amora/.local/bin/mise exec -- /bin/bash -c '{{.Command}}'
Restart=on-failure
RestartSec=5
EnvironmentFile=-{{.EnvFile}}

[Install]
WantedBy=default.target
```

#### Funções de Ciclo de Vida
O gerenciamento dos processos é feito através de abstrações que invocam comandos `systemctl --user`:
- **Criação:** `(g *Generator) GenerateService(cfg ServiceConfig)`
- **Sincronização:** `(g *Generator) DaemonReload()`
- **Operações Básicas:** `EnableService`, `RestartService`, `StopService`, `DisableService`.
- **Orquestração em Massa:**
  - `RestartAppServices(app)`: Lista a pasta buscando o padrão `amora-<app>-*` para dar restart de uma vez (usado quando se altera o `.env`). Também embute uma limpeza legada para antigos serviços `mdns`.
  - `DestroyAppServices(app)`: Realiza Stop -> Disable -> Remove file para cada processo vinculado ao app.

---

## 3. Contratos e Regras de Negócio

1. **Escopo de Usuário (`systemd --user`):**
   - **Contrato:** O Amora roda processos em nível de usuário, não como `root`. A CLI sempre invoca o systemctl com a flag `--user`.
   - **Linger:** Para que os serviços de um usuário continuem rodando após o fechamento da sessão SSH (logout), é pré-requisito (parte do setup inicial) que a flag de *linger* esteja habilitada no sistema operacional (`loginctl enable-linger amora`).

2. **Gerenciamento de Segredos e Variáveis (`EnvironmentFile`):**
   - **Regra:** Os segredos não são exportados via sessão Bash. O Systemd é instruído a carregar as chaves usando a tag `EnvironmentFile=-{{.EnvFile}}`.
   - O sinal de menos (`-`) antes do caminho é intencional: instrui o systemd a não falhar caso o arquivo `.env` não exista (aplicações que não dependem de portas nem de chaves privadas).

3. **Isolamento de Runtime (`mise exec`):**
   - **Regra:** O `ExecStart` não chama o comando de deploy diretamente. Para assegurar que o interpretador da aplicação (Node, Python, Bun, etc.) que o usuário especificou em `.tool-versions` / `mise.toml` seja respeitado, o comando é encapsulado via: `/home/amora/.local/bin/mise exec -- /bin/bash -c '...'`.

4. **Resiliência e Auto-Cura:**
   - Serviços são auto-recuperáveis. A política `Restart=on-failure` somada a `RestartSec=5` garante que se o processo crashear (falha de memória, panic), o daemon do Linux tentará levantá-lo novamente a cada 5 segundos indefinidamente.

---

## 4. Casos de Teste Sugeridos

A arquitetura do `Generator` usando IoC em `RunCmd` facilita muito a criação de testes robustos.

### Testes de Templates e Geração
- **Verificação de Strings (Golden Path):** Criar uma configuração dummy e rodar o `GenerateService`. Ler o arquivo resultante para checar se as chaves GoTemplate (`{{.App}}`, etc) não estão vazadas e se os diretórios bateram perfeitamente na string (testar injetar variáveis corrompidas no path).
- **Tratamento do `.env` nulo:** Certificar-se que a diretiva `-` no `EnvironmentFile` permanece estável nos arquivos gerados, mesmo que a propriedade `EnvFile` passe apenas o nome vazio ou sem path final.

### Testes de Idempotência e Ciclo de Vida
- **Redeploys Massivos:** Escrever o mesmo unit file repetidas vezes usando a API. O sistema deve sobrescrever graciosamente o documento e as invocações de Systemctl subsequentes não devem retornar erros fatais.
- **Limpeza Abrangente (`DestroyAppServices`):**
  1. O teste simula a criação de N serviços (ex: `amora-meuapp-web.service`, `amora-meuapp-worker.service`).
  2. Executa a destruição.
  3. Checa se o `StopService` e `DisableService` foram invocados via mocks *antes* do arquivo ser removido do disco.
  4. Checa no fim a chamada fundamental ao `DaemonReload`.
- **Prevenção de Falsos Positivos:** Passar um `DestroyAppServices("app")` em uma aplicação vizinha tipo `"app-staging"` e garantir que o validador de sufixo `-` previna que `amora-app-staging` seja acidentalmente desligado junto com `amora-app`.
