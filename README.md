<div align="center">
  <h1>🍇 Amora</h1>
  <p><strong>O Micro-PaaS definitivo para Raspberry Pi focado em Developer Experience.</strong></p>

  <p>
    <a href="https://go.dev/"><img src="https://img.shields.io/badge/Made%20with-Go%201.22+-00ADD8?style=flat-square&logo=go" alt="Go Version"></a>
    <a href="https://github.com/douglasgusson/amora/actions"><img src="https://img.shields.io/badge/Build-Passing-brightgreen?style=flat-square" alt="Build Status"></a>
    <a href="https://github.com/douglasgusson/amora/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square" alt="License"></a>
  </p>
</div>

---

## 🚀 1. Introdução

O **Amora** é um Micro-Platform as a Service (PaaS) open-source, ultraleve e feito em Go. Inspirado no workflow ágil do Heroku, ele foi projetado desde o primeiro dia para transformar dispositivos ARM64 — como o Raspberry Pi — no seu servidor de laboratório pessoal ou para redes locais.

**A Regra de Ouro: Zero Docker.**  
Em vez de engolir a memória do seu dispositivo com containers pesados e camadas virtuais, o Amora roda tudo de forma nativa. Ele orquestra seus processos diretamente via `systemd` (em modo _user_), resolve o isolamento de runtimes (Node, Python, Go, Ruby, etc) utilizando o utilitário `mise`, e provê roteamento dinâmico e resolução DNS local sem *overhead*.

Tudo o que você precisa fazer para colocar um app no ar é: `git push amora main`.

---

## ⚙️ 2. Como Funciona (A Engrenagem)

O Amora baseia-se em uma arquitetura de orquestração síncrona e minimalista, onde um simples "push" aciona uma cadeia de eventos elegante e resiliente:

1. **Git Hook (post-receive):** Ao receber o seu código via SSH num repositório *bare*, o hook intercepta o fluxo e aciona o motor do Amora. O código é extraído para uso e o build é isolado dentro do runtime específico do seu projeto usando o `mise`.
2. **systemd --user:** O Amora lê o contrato do seu arquivo `Procfile` (ex: `web: node server.js`) e converte os seus processos efêmeros em serviços reais do Linux. Sem exigir permissões root, os processos são mantidos vivos de forma robusta e segura no escopo do usuário.
3. **Caddy Admin API:** Para processos marcados como `web`, uma porta dinâmica é alocada internamente. A CLI então atualiza a tabela de roteamento no Caddy via uma requisição atômica à sua Admin API local (`:2019/load`), garantindo um _zero-downtime proxy reload_.
4. **Avahi mDNS:** Através de edições diretas e idempotentes no arquivo `/etc/avahi/hosts`, o daemon do Avahi faz o broadcast do domínio na rede local instantaneamente. O aplicativo passa a responder em `http://seu-app.local`.

---

## ✨ 3. Recursos Principais (Features)

A plataforma entrega uma experiência completa *"out of the box"* focada puramente na produtividade do desenvolvedor:

* **🛠️ Pipeline de Deploy Inteligente:** Disparos automáticos via *Git Push*, interpretação sem estado de `Procfile` e suporte a hooks customizados de execução pré-start (como scripts `amora-build`).
* **🔄 Gerenciamento de Serviços Nativos:** Abstração completa do SO, gerando _units_ integradas ao `systemd --user`. Garante restarts imediatos contra falhas isoladas de processo (`Restart=on-failure`) de forma nativa.
* **🌐 Roteamento Dinâmico Avançado:** Proxy Reverso automático amparado na solidez do Caddy. Cada novo serviço _web_ lançado ganha uma porta e um domínio sem que o desenvolvedor tenha que intervir manualmente na infraestrutura.
* **📡 Descoberta Local Automatizada:** Integração com mDNS (Avahi). Descubra os IPs das suas aplicações magicamente na sua rede LAN, acessando os serviços pelo sufixo `.local`.
* **🔐 Gerenciamento de Variáveis e Segredos:** Gestão ágil de chaves/valores via CLI, em um formato isolado e *zero-database*. Protege credenciais injetando as variáveis no momento da inicialização pelo systemd. Mutar uma variável reinicia sua aplicação imediatamente de maneira segura.
* **💻 CLI Administrativa Completa:** Ferramental de terminal super completo. Provisione novos servidores remotamente do seu computador, crie apps com um comando e monitore logs agrupados em tempo real através do utilitário `journalctl`.
* **📊 Monitoramento de Recursos em Tempo Real:** Dashboard simplificado acessível via `amora status`. Visualiza o consumo de CPU, uso de RAM, disco e serviços monitorados cirurgicamente via arquivos do Kernel Linux (`/proc`), agregando estatísticas essenciais sem consumir *overhead* da placa.

---

## ⚡ 4. Guia Rápido de Uso

O fluxo do Amora foi desenhado para não ter fricções. A partir do seu terminal local de desenvolvimento (Mac/Linux), siga o roteiro:

### Passo 1: Provisionar o Servidor
Com a CLI instalada no seu computador, prepare o Raspberry Pi (ou servidor remoto) usando apenas um comando mágico:
```bash
# Executado na sua máquina local
amora provision pi@raspberrypi.local
```

### Passo 2: Criar a Aplicação
Acesse o Pi com a conta de isolamento recém-criada (`amora`), inicialize o sistema base e provisione seu novo app:
```bash
# Acesse o servidor provisionado
ssh amora@raspberrypi.local

# Inicialize as pastas de estrutura base (rode apenas a primeira vez)
amora setup

# Instancie os repositórios da sua aplicação
amora create --app meu-app
```

### Passo 3: Deploy (Takeoff)
De volta ao seu computador local, abra a pasta do código-fonte do projeto, aponte para o servidor amora e mande para a nuvem local:
```bash
# Na raiz do seu projeto (ex: Node, Go, Python, com um Procfile existente)
git remote add amora amora@raspberrypi.local:repos/meu-app.git
git push amora main
```
Observe pelo terminal os logs estilizados da pipeline orquestrando o build e, em segundos, acesse no navegador: `http://meu-app.local`!
