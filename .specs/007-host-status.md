# Spec 007: Host Status Monitor (Zero-Overhead)

## 1. Objetivo e Contexto da Nova Feature

A premissa primária do Amora é manter o ecossistema leve e altamente performático, evitando qualquer sobrecarga desnecessária no Raspberry Pi (como a exigência de flags customizadas de Kernel para accounting de cgroups).

Para conceder visibilidade ao desenvolvedor sem ferir esse princípio, o "Host Status Monitor" tem como objetivo implementar o comando `amora status`. Em vez de rastrear minuciosamente o consumo por aplicativo, ele exibe um sumário vital da saúde da máquina inteira e um agregado dos apps instalados. 

**Exemplo de UX Esperada:**
```text
$ amora status
🍇 Amora - Host Status

💻 CPU Load (1m, 5m, 15m): 0.15, 0.08, 0.04
🧠 RAM: 245 MB / 980 MB (25%)
💾 Disk (/): 4.2 GB / 14 GB (30%)

📦 Apps Instalados: 3 (blog, extrato-bot, rss-bot)
```

---

## 2. Arquitetura Proposta e Estrutura de Arquivos

Para garantir rapidez (sem spawnar sub-processos do shell) e robustez, criaremos um novo sub-módulo interno apenas para lidar com leituras nativas do Linux.

### Módulo Monitor (`internal/monitor/`)
**Novo Arquivo:** `internal/monitor/host.go`
- **Struct:** `HostMetrics{ Load1, Load5, Load15, MemUsed, MemTotal, DiskUsed, DiskTotal }`
- **Funções Responsáveis:**
  - `GetHostMetrics() (HostMetrics, error)`: Abstrai toda a leitura do sistema.
  - Abstrações de leitura simuláveis via interface (para facilitar testes no macOS/Windows onde o `/proc` é ausente ou diferente).

### Módulo CLI (`internal/cli/`)
**Novo Arquivo:** `internal/cli/status.go`
- **Funções Responsáveis:**
  - `NewStatusCmd()`: Comando Cobra sem argumentos.
  - Varre o diretório base de apps (`~/apps/`) via pacote nativo `os` para listar o que está instalado no ecossistema e compõe o dashboard junto às métricas do host.

---

## 3. Contratos e Regras de Negócio Cruciais

1. **Eficiência e Velocidade (Zero Shell Exec):**
   - **RAM e CPU Load:** Não invocaremos os binários `free` ou `uptime` através do shell (`os/exec`). As métricas devem ser extraídas por parsing direto dos arquivos puramente estáticos na memória do Linux: `/proc/meminfo` e `/proc/loadavg`.
   - **Disk Usage:** A métrica do disco não deve depender do comando `df`. Deve invocar a syscall nativa `syscall.Statfs("/")` para extrair os blocos da partição raiz com zero impacto de IO computacional.

2. **Compatibilidade Cruzada e Fallback no SO (Darwin/Windows):**
   - Como o desenvolvimento da CLI acontece localmente (frequentemente em macOS) e o deploy no Linux, o pacote `internal/monitor/` **não deve estourar panics** se o sistema de arquivos `/proc` não existir. 
   - A regra de negócio exige um "Graceful Degradation": caso os arquivos `/proc/meminfo` falhem na leitura (ex: o programador executou `amora status` localmente no Mac), os números podem aparecer como 0 ou N/A na tela sem falhar a compilação ou a execução.

3. **Cálculo Preciso da RAM:**
   - O `/proc/meminfo` no Linux possui diversas chaves (Buffers, Cache, SReclaimable). A conta de uso real de RAM geralmente deve ser baseada em `MemTotal - MemAvailable` (ou uma heurística equivalente) para que o desenvolvedor tenha a percepção real do que pode ser usado, tal qual o `htop` moderno faz.

---

## 4. Casos de Teste Obrigatórios

O teste usará injeção de interface ou sobrescrita do caminho padrão (`/proc/...` para `/tmp/...` nos testes) para validar a confiabilidade do parsing.

### Parsing de Output (Mocking do Linux Kernel File System)
- **Golden Path (RAM):** O arquivo de teste gerará localmente um falso `/proc/meminfo` contendo valores chave, e o teste garantirá que a métrica extraída corresponda à conversão matemática exata (Bytes/Kilobytes para MB/GB).
- **Graceful Downgrade (SO Alheio):** O teste invocará o `GetHostMetrics()` apontando para caminhos quebrados e atestará que os erros são retornados adequadamente, confirmando que a aplicação inteira não crasha devido a um file descriptor ausente.
