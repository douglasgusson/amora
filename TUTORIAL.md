# 🍇 Tutorial: Meu Primeiro App no Amora

Este guia prático mostrará como criar e publicar (deploy) uma aplicação Node.js do zero usando o **Amora**, do seu computador até a nuvem (seu Raspberry Pi).

---

## 🛠️ Pré-requisitos
Assumimos que o seu Raspberry Pi já foi provisionado com sucesso e está na mesma rede Wi-Fi que você.

- O usuário `amora` existe no seu Raspberry Pi.
- Você tem o IP do Raspberry Pi (ex: `192.168.1.120`) ou o hostname (ex: `raspberrypi.local`).

---

## Passo 1: Criar o App no Servidor (Raspberry Pi)

Primeiro, precisamos dizer ao Amora para preparar o terreno para receber o código.

1. Acesse o seu Raspberry Pi via SSH:
   ```bash
   ssh amora@raspberrypi.local
   ```
2. Crie a aplicação usando a CLI do Amora. Vamos chamar o app de **`meublog`**:
   ```bash
   amora create --app meublog
   ```
   *(O Amora vai inicializar um repositório Git bare, preparar o hook de deploy e pré-alocar uma porta).*

3. Pode sair do servidor (`exit`). O resto faremos da sua máquina!

---

## Passo 2: Preparar sua Aplicação Localmente (PC)

Abra o terminal no seu computador de trabalho e crie uma nova pasta para o projeto:

```bash
mkdir meublog
cd meublog
git init
```

### 1. O Código da Aplicação (`server.js`)
Crie um servidor web simples. Note que usamos o fallback `process.env.PORT || 5000` (Padrão 12-Factor):

```javascript
// server.js
const http = require('http');

const PORT = process.env.PORT || 5000;
const NOME = process.env.NOME || 'Visitante';

const server = http.createServer((req, res) => {
  res.statusCode = 200;
  res.setHeader('Content-Type', 'text/plain; charset=utf-8');
  res.end(`Olá, ${NOME}! O deploy no Amora foi um sucesso! 🍇🚀\n`);
});

server.listen(PORT, () => {
  console.log(`Servidor rodando na porta ${PORT}`);
});
```

### 2. O Contrato de Processos (`Procfile`)
Crie um arquivo chamado `Procfile` para dizer ao Amora como iniciar o seu app:
```text
web: node server.js
```

### 3. A Versão do Node (`.node-version`)
Crie um arquivo `.node-version` para dizer ao Amora qual versão exata baixar via `mise`:
```text
20.11.0
```

### 4. (Opcional) Script de Build (`amora-build`)
Se o seu app precisasse de `npm install` ou transpilação, você criaria um script executável chamado `amora-build`:
```bash
#!/bin/bash
# npm install
```

---

## Passo 3: Fazer o Deploy (A Mágica 🪄)

Faça o commit de tudo o que criamos:
```bash
git add .
git commit -m "Primeiro commit da aplicação"
```

Agora, adicione o servidor do Amora como um "remote" do Git. Substitua `raspberrypi.local` pelo IP do seu Pi, se preferir:
```bash
git remote add amora amora@raspberrypi.local:repos/meublog.git
```

Faça o push (o deploy de fato):
```bash
git push amora main
```

Você verá a saída do pipeline em tempo real no seu terminal:
```text
🍇 Amora

-----> Deploying 'meublog'...
-----> Received push on branch 'main'
-----> Checking out code...
       ✓ Code checked out to /home/amora/apps/meublog
-----> Provisionando runtimes (mise)...
-----> Reading Procfile...
       ✓ Process: web → node server.js
-----> Resolving port allocation...
       ✓ PORT=5000
-----> Generating systemd services...
       ✓ Generated amora-meublog-web.service
-----> Configuring reverse proxy...
       ✓ Caddyfile: meublog.local → localhost:5000
-----> Configuring mDNS (Avahi)...
-----> Reloading systemd daemon...
       ✓ Daemon reloaded
-----> Starting services...
       ✓ Started amora-meublog-web.service

-----> Deploy complete! 🎉

  🌐 http://meublog.local
  📡 http://192.168.1.120:5000
  🔌 http://localhost:5000
```

---

## Passo 4: Acessar a Aplicação

Abra o navegador no seu celular ou computador conectado à mesma rede Wi-Fi e digite:

**[http://meublog.local](http://meublog.local)**

Se o cache de DNS da sua máquina estiver demorando para reconhecer o `.local`, você pode usar a rota direta de IP e porta fornecida no final do deploy:

**[http://192.168.1.120:5000](http://192.168.1.120:5000)**

---

## ⚙️ Bônus: Gerenciamento do Dia a Dia

Para administrar a aplicação, conecte-se ao Raspberry Pi (`ssh amora@raspberrypi.local`) e use os comandos do Amora:

### Ver os Logs em Tempo Real
```bash
amora logs meublog -f
```

### Configurar Variáveis de Ambiente
Lembra que o nosso código usa `process.env.NOME`? Vamos mudar o nome de Visitante para o seu nome:
```bash
amora env set meublog NOME="Douglas"
```
*(O Amora vai salvar a variável e reiniciar a aplicação automaticamente em milissegundos sem você perceber o downtime).*

Atualize o navegador e você verá: **Olá, Douglas! O deploy no Amora foi um sucesso!** 🍇🚀

