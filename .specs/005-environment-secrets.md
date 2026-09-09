# Spec 005: Environment & Secrets Management

## 1. Objetivo e Fluxo
O gerenciamento de variáveis de ambiente no Amora centraliza a injeção de configurações e credenciais (segredos) em tempo de execução sem demandar a criação de bancos de dados adicionais (Zero-Database policy). Ele garante que variáveis definidas pelo desenvolvedor sejam armazenadas de forma segura e refletidas de maneira síncrona nos processos daemonizados da aplicação alvo.

### O Ciclo de Vida das Variáveis
1. **Interface de Interação (CLI):** O desenvolvedor executa comandos de mutação (ex: `amora env set app-name DATABASE_URL=postgres://...`) ou leitura (`amora env ls app-name`).
2. **Parsing do File System:** O módulo localiza o arquivo persistente do app em `~/.amora/envs/<app>.env`. Se o arquivo não existir (primeira variável setada para o app), ele é criado na hora.
3. **Mapeamento KV em Memória:** O conteúdo do arquivo é lido linha a linha. Espaços vazios e comentários (`#`) são ignorados. As chaves válidas sofrem bypass para um `map[string]string`. 
4. **Mutação e Ordem Determinística:** A chave desejada é adicionada, alterada ou removida no mapa. Em seguida, as chaves do mapa são ordenadas alfabeticamente e reescritas no disco. Essa ordenação é fundamental para evitar ruídos desnecessários em diffs de backup e manter consistência na ordem injetada no systemd.
5. **Aplicação do Estado (Restart):** Após qualquer modificação bem-sucedida, o CLI orquestra automaticamente as chamadas necessárias ao motor do Systemd (`daemon-reload` seguido de `RestartAppServices`), forçando a aplicação a droppar processos antigos e ler o `.env` renovado instantaneamente.

---

## 2. Estrutura de Código Atual

O motor lógico reside em `internal/env/` enquanto a abstração do usuário vive em `internal/cli/env.go`.

### Lógica de Negócio: `env.go`
- **Struct Principal:** `Manager`
  - Mantém o encapsulamento através da propriedade `BaseDir` (por padrão, extrai o home: `/home/amora/.amora/envs/`).
- **Funções Chave de I/O:**
  - `Load(app string) (map[string]string, error)`: Lê o arquivo físico (isolado do systemd). Faz bypass com o `bufio.Scanner`, ignorando strings vazias. Se o arquivo não existe, retorna o mapa vazio de forma limpa, não um `error`.
  - `Save(app string, vars map[string]string) error`: Grava de volta. Ele puxa as chaves do mapa para um slice auxiliar, utiliza `sort.Strings(keys)` e invoca iterativamente a escrita de `KEY=VALUE`.
- **Helpers de Mutação:** `Set`, `Remove` e `Delete`. Todos abrem o arquivo chamando `Load`, modificam o estado em memória, e consolidam usando o `Save`.

### Integração (Orquestração de Comandos): `cli/env.go`
- Módulo gerado em cima do ecossistema Cobra (`cobra.Command`).
- Possui os construtores:
  - `newEnvSetCmd()`: Capaz de extrair múltiplos argumentos chave=valor `KEY=VALUE...` de um mesmo input e iterar o mapa chamando as funções do `env.go`.
  - `newEnvLsCmd()`: Faz o pull e também re-ordena (`sort`) visualmente o retorno para não poluir o terminal.
  - `newEnvRmCmd()`: Encontra uma chave e a descarta.
- **Acoplamento Vital:** Tanto no comando `set` quanto no `rm`, há chamadas diretas atrelando o módulo do Systemd. O código executa `systemd.DaemonReload()` e depois `systemd.RestartAppServices(app)`. Sem este passo, as mudanças seriam escritas, mas o runtime da aplicação ignoraria os novos segredos até o próximo deploy manual.

---

## 3. Contratos e Regras de Negócio

1. **A Fonte da Verdade em Disco (`~/.amora/envs/`):**
   - **Regra:** O Amora optou por persistir `.env` fora do escopo do repositório da aplicação (o `work-tree`). Isso garante que variáveis nunca serão substituídas ou sobrepostas acidentalmente por um comando de Git Checkout durante um deploy e respeita perfeitamente as diretrizes da *Twelve-Factor App*.

2. **Isolamento de Segurança (Systemd `EnvironmentFile`):**
   - Os valores carregados não poluem as exportações do bash root ou bash do usuário. As variáveis só existem sob a égide do Systemd, carregadas pontualmente por via do unit file da aplicação (na flag `EnvironmentFile=-/home/amora/.amora/envs/meuapp.env`). Isso reduz drasticamente a superfície de ataque caso um script malicioso tente invocar `printenv` num processo concorrente do bash principal.

3. **Automação de Ciclo de Vida (Restarts):**
   - **Regra:** Mutar variáveis (via Set ou Rm) obrigatoriamente ativa o comportamento de *graceful restart* na plataforma, disparando o restart de todos os processos Systemd vinculados unicamente àquele app. O desenvolvedor não deve em momento algum ser instruído a dar reload manual nos containers ou units de sistema após definir uma chave.

4. **Port Allocation Binding:**
   - Este módulo de variáveis armazena também (e protege) a chave intrínseca do framework: `PORT`. Embora a `PORT` surja na etapa de build/deploy, ela reside dentro da estrutura gerenciada pelos módulos descritos aqui. 

---

## 4. Casos de Teste Sugeridos

A arquitetura orientada a manipuladores I/O isolados do pacote Systemd torna esse sub-sistema bem fácil de testar via mock do diretório base.

### Testes do Parser (`Manager.Load` e `Save`)
- **Caracteres Especiais e Quotes:** Injetar `.env` mocks com chaves problemáticas: `DATABASE_URL="postgres://user:p@ss=word@host/db"`. A extração do `strings.SplitN(line, "=", 2)` foi feita pra aguentar até 2 campos, preservando quaisquer `=` subsequentes no valor. Esse caso base é obrigatório ser validado em testes unitários.
- **Tratamento de Comentários e Espaçamento:** Simular arquivos com `# comentário solto`, espaços excessivos antes das chaves e testar se a leitura ignora perfeitamente as strings sem sujar o HashMap Go.
- **Garantia de Idempotência e Order:** Salvar uma sequência desordenada `Z=1`, `A=2`, ler novamente o arquivo com Go primitivo e asserir se a primeira linha escrita é estritamente `A=2` independente de concorrência.

### Testes de Integração e Side Effects (CLI)
- Invocação mockada dos hooks de CLI (`set` e `rm`) validando via Injeção de Dependência se os pacotes simuladores do Systemd (`MockSystemd`) registraram as duas chamadas primordiais do workflow na ordem exata: `DaemonReload` primeiro e `RestartAppServices` por último.
