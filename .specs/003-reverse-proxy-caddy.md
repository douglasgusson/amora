# Spec 003: Reverse Proxy & Routing (Caddy Integration)

## 1. Objetivo e Fluxo
A abstração do Caddy Server dentro do Amora serve para fornecer o roteamento dinâmico sem que o usuário precise gerenciar portas abertas. O módulo transforma os processos designados como `web` (que possuem alocação de `PORT`) em serviços transparentemente roteados via um nome de domínio (`<app>.local`). 

Para evitar reiniciar o serviço do proxy abruptamente em cada deploy (o que derrubaria conexões de outras aplicações vizinhas), o Amora utiliza a **Admin API do Caddy** para carregamento de estado *Zero-Downtime*.

### O Ciclo de Vida do Roteamento
1. **Geração do Snippet:** Uma vez que a porta do processo `web` é alocada, a função geradora escreve um pequeno trecho de configuração (um arquivo `.caddyfile`) exclusivamente para o app.
2. **Agregação (Concatenação):** O Amora não envia a configuração individual, mas sim orquestra a leitura de **todos** os arquivos de snippet contidos no diretório de Caddy do usuário (`/home/amora/caddy/*.caddyfile`), concatenando-os em um único payload monolítico em memória.
3. **Commit de Estado (API Load):** O payload é injetado via HTTP `POST` para `http://localhost:2019/load` (a API Admin do Caddy) com o header `Content-Type: text/caddyfile`.
4. **Aplicação Atômica:** O Caddy valida sintaticamente o payload em memória e, se estiver correto, assume imediatamente a nova tabela de roteamento sem derrubar conexões ativas. Se o Caddy encontrar erro (ex: sintaxe inválida), ele rejeita o payload HTTP (retornando erro 4xx/5xx) e o Amora propaga essa falha sem comprometer o estado anterior do proxy.
5. **Remoção e Limpeza:** Na destruição de um app (`amora destroy`) ou na mudança de um app de `web` para worker puro, o arquivo `.caddyfile` específico é apagado e o processo de agregação e envio de estado é reexecutado para refletir o proxy enxuto.

---

## 2. Estrutura de Código Atual

Toda a lógica está isolada no pacote `internal/proxy/`.

### Arquivo Core: `caddy.go`

#### Constantes Chave
- `CaddyDir = "/home/amora/caddy"`: O diretório-fonte da verdade, onde os snippets individuais de cada app repousam.
- `CaddyAPI = "http://localhost:2019/load"`: O endpoint local da API oficial do Caddy.

#### Funções Geradoras
- `GenerateCaddyfileAt(dir, app string, port int)`:
  - Formata e escreve o snippet em disco usando uma string injetável muito simples:
    ```caddyfile
    http://%s.local {
        reverse_proxy 127.0.0.1:%d
    }
    ```
- `GenerateCaddyfile` e `CaddyfilePath`: Wrappers que invocam as primitivas passando a constante do diretório oficial de produção.
- `RemoveCaddyfile(app string)`: Responsável pela faxina do domínio, removendo o arquivo `.caddyfile`.

#### Funções de Transporte (API)
- `ReloadCaddyFrom(dir string)`: 
  - Realiza um `os.ReadDir(dir)`.
  - Ignora pastas ou arquivos que não terminem com o sufixo `.caddyfile`.
  - Acumula o conteúdo de todos eles usando um `strings.Builder`.
  - Executa uma requisição `http.NewRequest` e valida o `resp.StatusCode == http.StatusOK`.

---

## 3. Contratos e Regras de Negócio

1. **Desacoplamento e Granularidade (Snippets):**
   - **Regra:** O Caddy não mantém um `Caddyfile` global persistido que o Amora precise ficar abrindo e fazendo parser para deletar ou adicionar linhas. A persistência é gerenciada através de arquivos por aplicação (`app1.caddyfile`, `app2.caddyfile`). Isso elimina o risco de corrupção textual e conflito de concorrência.

2. **Carga Atômica e Rollback Implícito:**
   - **Regra:** O Amora não interage com binários CLI (`caddy reload`). O fluxo é gerido integralmente por requisições HTTP para a Admin API. O Caddy Server possui validação transacional: se os snippets combinados estiverem malformados, a requisição HTTP do Amora falhará (`StatusCode != 200`), os logs de erro da pipeline mostrarão a rejeição do Caddy, e a configuração rodando anteriormente no proxy não sofrerá absolutamente nada (nenhum "app down" acidental).

3. **Restrição de Conexão Local:**
   - A requisição `POST` vai direcionada para `127.0.0.1:2019` obrigatoriamente. O Caddy deve estar rodando no host e não expor essa porta à rede externa por razões óbvias de segurança de configuração.

---

## 4. Casos de Teste Sugeridos

A isolação do tráfego (`ReloadCaddyFrom`) em contraste aos wrappers de produção permite criar suítes robustas de teste injetando diretórios temporários (`t.TempDir()`) e servidores mock (`httptest.NewServer`).

### Testes de Formatação e Geração (Snippets)
- **Criação Sintática:** Gerar o arquivo num diretório mock e usar regex para afirmar se o template final bate precisamente com `http://nome-do-app.local {\n\treverse_proxy 127.0.0.1:8080\n}\n`. Espaços importam no ecossistema Caddy.
- **Isolamento de Extensão:** Injetar arquivos lixo (`app1.txt`, `app2.md`, `.caddyfile_backup`) no diretório de testes e validar se o parser do `ReloadCaddyFrom` é restrito o suficiente para compilar no builder exclusivamente os `.caddyfile`.

### Testes de Comportamento Transacional HTTP
- **Golden Path:** Subir um `httptest.NewServer` que retorne `200 OK` na rota `/load` e validar se o Amora não propaga erros e processou tudo adequadamente.
- **Falhas Críticas de API:** 
  1. O servidor mock retorna `400 Bad Request` ou `502 Bad Gateway` (simulando sintaxe inválida). A função deve retornar o erro populado com o body de resposta para troubleshooting imediato via logs do cli.
  2. O servidor proxy falha por Timeout (Caddy inativo). A função deve abortar e propagar o erro de rede, ao invés de pânicos silenciados, indicando na tela que o dev precisa rodar (ou ativar) o processo root do proxy no Raspberry Pi.
