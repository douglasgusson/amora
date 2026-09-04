#!/bin/bash
set -e

REPO="douglasgusson/amora"
echo "🍇 Instalando Amora CLI..."

# 1. Detectar Sistema Operacional
OS="$(uname -s)"
case "${OS}" in
    Linux*)     OS_NAME=Linux;;
    Darwin*)    OS_NAME=Darwin;;
    *)          echo "❌ Sistema operacional não suportado: ${OS}"; exit 1;;
esac

# 2. Detectar Arquitetura
ARCH="$(uname -m)"
case "${ARCH}" in
    x86_64)  ARCH_NAME=x86_64;;
    arm64)   ARCH_NAME=arm64;;
    aarch64) ARCH_NAME=arm64;;
    *)       echo "❌ Arquitetura não suportada: ${ARCH}"; exit 1;;
esac

# 3. Buscar a versão mais recente da Release do Github
echo "🔍 Buscando a versão mais recente..."
LATEST_TAG=$(curl -sL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_TAG" ]; then
    echo "❌ Erro ao buscar a versão mais recente. Verifique se existe uma Release publicada no repositório."
    exit 1
fi

echo "📦 Versão encontrada: $LATEST_TAG"

# 4. Baixar e extrair
TAR_FILE="amora_${OS_NAME}_${ARCH_NAME}.tar.gz"
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_TAG/$TAR_FILE"

TMP_DIR=$(mktemp -d)
echo "⬇️ Baixando de $DOWNLOAD_URL..."

if ! curl -sLf "$DOWNLOAD_URL" -o "$TMP_DIR/$TAR_FILE"; then
    echo "❌ Erro ao baixar o arquivo. A URL falhou: $DOWNLOAD_URL"
    exit 1
fi

echo "📂 Extraindo..."
tar -xzf "$TMP_DIR/$TAR_FILE" -C "$TMP_DIR" amora

# 5. Instalar o binário no sistema
INSTALL_DIR="/usr/local/bin"
echo "⚙️ Instalando em $INSTALL_DIR (pode pedir senha de administrador/sudo)..."
sudo mv "$TMP_DIR/amora" "$INSTALL_DIR/amora"
sudo chmod +x "$INSTALL_DIR/amora"

# 6. Limpeza
rm -rf "$TMP_DIR"

echo "✅ Amora instalado com sucesso!"
echo "🚀 Rode 'amora --help' para começar."
