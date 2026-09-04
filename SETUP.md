# 🍓 Setup Inicial: Preparando o Raspberry Pi do Zero

Este guia é para você que acabou de tirar um Raspberry Pi da caixa (ou formatou o cartão SD) e quer transformá-lo no seu próprio servidor Micro-PaaS com o **Amora**.

O Amora foi construído com a filosofia **"Zero Setup"**: você faz quase tudo a partir do seu computador pessoal (Mac/Linux), e o Amora se encarrega de instalar e configurar tudo lá no Pi de forma automatizada.

---

## 🛠️ Passo 1: Preparando o Cartão SD

Se o seu Raspberry Pi já está na rede e acessível via SSH, pode pular para o Passo 2. Caso contrário, siga estes passos para instalar o sistema operacional:

1. Baixe o **[Raspberry Pi Imager](https://www.raspberrypi.com/software/)** no seu Mac/PC.
2. Em **Operating System**, escolha **Raspberry Pi OS (64-bit)**. O Amora é otimizado para a arquitetura ARM64.
3. Clique no ícone de **Engrenagem (Configurações Avançadas)** antes de gravar:
   - **Hostname**: Defina como `raspberrypi` (ou outro de sua preferência).
   - **Enable SSH**: Marque esta opção e escolha "Use password authentication".
   - **Set username and password**: Defina um usuário temporário de admin (ex: usuário `pi`, senha `raspberry`).
   - **Configure wireless LAN**: Insira o nome (SSID) e a senha do seu Wi-Fi.
4. Grave a imagem no cartão SD, insira no Raspberry Pi e ligue-o na tomada.
5. Aguarde uns 2 a 3 minutos para ele dar boot e conectar no seu Wi-Fi.

---

## 💻 Passo 2: Instalando o Amora (Seu Computador)

Agora, volte para o seu computador (Mac/Linux). Graças ao nosso sistema de distribuição, você não precisa ter o Go instalado para compilar nada.

Abra o seu terminal e instale a CLI do Amora rodando o comando mágico:

```bash
curl -sL https://raw.githubusercontent.com/douglasgusson/amora/main/install.sh | bash
```

*(O script vai detectar se você usa Mac ou Linux e baixar o executável pronto diretamente do Github).*

Em seguida, execute o comando de provisionamento apontando para o usuário e IP/hostname do seu Pi recém-instalado:
```bash
amora provision pi@raspberrypi.local
```
*(Ele vai te pedir a senha do usuário `pi` algumas vezes)*

### O que o provisionamento está fazendo?
Relaxe e assista. Em cerca de 3 a 5 minutos, ele vai:
- Atualizar a lista de pacotes (`apt-get update`).
- Instalar dependências vitais (Git, Caddy Server, Avahi mDNS).
- Criar um **usuário restrito e seguro** chamado `amora`.
- Configurar as suas chaves SSH do Mac direto no usuário `amora` (adeus senhas!).
- Habilitar o `systemd linger` (para as aplicações continuarem rodando após você fechar o SSH).
- Baixar o `mise` (o gerenciador de linguagens como Node/Python) apenas para o usuário `amora`.
- **Cross-compilar** o binário do Amora (para arquitetura Linux ARM64) no seu Mac e enviar via rede para o Pi.

---

## 🏗️ Passo 3: Inicializando a Estrutura (Setup)

Agora que o servidor está com todas as ferramentas de sistema instaladas, vamos preparar a estrutura de pastas do PaaS. 

1. Conecte-se ao Pi usando o novo usuário do Amora (a senha não será mais pedida, pois a chave SSH foi configurada no passo anterior):
   ```bash
   ssh amora@raspberrypi.local
   ```

2. Dentro do Pi, rode o comando de setup do Amora:
   ```bash
   amora setup
   ```
   *(Este comando roda instantaneamente. Ele cria os diretórios onde seus aplicativos e repositórios vão morar, como `~/apps`, `~/repos`, `~/caddy` e `~/.amora/envs`).*

---

## 🎉 Pronto! 

Seu Raspberry Pi foi transformado em um Micro-PaaS caseiro superpoderoso. 

Ele está aguardando você fazer o deploy do primeiro aplicativo via `git push`. Para isso, basta seguir o **[TUTORIAL.md](./TUTORIAL.md)**.

