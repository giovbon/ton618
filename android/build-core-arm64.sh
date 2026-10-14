#!/bin/bash
# ==============================================================================
# Script de Compilação do TON-618 Core para Android (ARM64)
# ==============================================================================
set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
CORE_DIR="$ROOT_DIR/core"
ASSETS_DIR="$SCRIPT_DIR/app/src/main/assets"

mkdir -p "$ASSETS_DIR"

echo "🌌 Compilando TON-618 Core para Android (ARM64)..."

# 1. Garante que os templates Templ estão gerados
echo "🔨 Gerando templates Templ..."
cd "$CORE_DIR"
go run github.com/a-h/templ/cmd/templ@latest generate

# 2. Localiza o compilador C do Android NDK se configurado
NDK_CC=""
if [ -n "${ANDROID_NDK_HOME:-}" ] || [ -n "${ANDROID_NDK_ROOT:-}" ]; then
    NDK_PATH="${ANDROID_NDK_HOME:-$ANDROID_NDK_ROOT}"
    TOOLCHAIN="$NDK_PATH/toolchains/llvm/prebuilt/linux-x86_64/bin"
    if [ -x "$TOOLCHAIN/aarch64-linux-android26-clang" ]; then
        NDK_CC="$TOOLCHAIN/aarch64-linux-android26-clang"
    elif [ -x "$TOOLCHAIN/aarch64-linux-android28-clang" ]; then
        NDK_CC="$TOOLCHAIN/aarch64-linux-android28-clang"
    fi
fi

TARGET_OUT="$ASSETS_DIR/ton618-arm64"

if [ -n "$NDK_CC" ]; then
    echo "🤖 Usando Android NDK Clang: $NDK_CC"
    CGO_ENABLED=1 \
    GOOS=android \
    GOARCH=arm64 \
    CC="$NDK_CC" \
    go build -tags sqlite_fts5 -ldflags="-s -w" -o "$TARGET_OUT" ./cmd/server
else
    # Fallback caso NDK não esteja no path direto: tenta aarch64-linux-gnu-gcc
    if command -v aarch64-linux-gnu-gcc >/dev/null 2>&1; then
        echo "🐧 Usando cross-compiler GNU: aarch64-linux-gnu-gcc"
        CGO_ENABLED=1 \
        GOOS=linux \
        GOARCH=arm64 \
        CC=aarch64-linux-gnu-gcc \
        go build -tags sqlite_fts5 -ldflags="-s -w" -o "$TARGET_OUT" ./cmd/server
    else
        echo "⚠️  Android NDK não detectado (variável ANDROID_NDK_HOME vazia)."
        echo "   Para compilar com SQLite FTS5 nativo, instale o Android NDK ou o pacote gcc-aarch64-linux-gnu."
        echo "   Tentando build puro sem CGO como fallback de validação..."
        CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o "$TARGET_OUT" ./cmd/server
    fi
fi

if [ -f "$TARGET_OUT" ]; then
    chmod +x "$TARGET_OUT"
    echo "✅ Binário gerado com sucesso em: $TARGET_OUT ($(du -h "$TARGET_OUT" | cut -f1))"
else
    echo "❌ Erro ao gerar o binário."
    exit 1
fi
