#!/bin/bash
# user: admin | pass: ton618
# 🌌 TON-618 v2 — Runner
# Motor de Busca Personal Knowledge Management

# Identificar o diretório raiz do projeto de forma absoluta
BASE_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
DATA_DIR="$BASE_DIR/data"

export DOCS_DIR
export DATA_DIR

# Carregar .env se existir
if [ -f "$BASE_DIR/.env" ]; then
    echo "📄 Carregando variáveis de .env"
    set -a
    source "$BASE_DIR/.env"
    set +a
fi

# Usar valores do .env ou padrões
PORT="${PORT:-6180}"
DB_PATH="${DB_PATH:-$DATA_DIR/ton618.db}"
STATE_DIR="${STATE_DIR:-$DATA_DIR}"

EMBEDDING_ALL="${EMBEDDING_ALL:-false}"

if [ -n "$EMBEDDING_API_KEY" ]; then
    echo "🌌 Iniciando TON-618 v2..."
    echo "🗄️  Banco SQLite em: $DB_PATH"
else
    echo "🌌 Iniciando TON-618 v2..."
    echo "🗄️  Banco SQLite em: $DB_PATH"
fi
echo "🔌 Porta: $PORT"

# Garantir diretórios
mkdir -p "$DOCS_DIR" "$DATA_DIR"

# Limpar processos zumbis na porta
fuser -k "${PORT}/tcp" &> /dev/null || true

# Log
LOG_FILE="$DATA_DIR/ton618.log"
touch "$LOG_FILE"

echo "📦 Baixando dependências..."
cd "$BASE_DIR"
export PATH=$PATH:/usr/local/go/bin:/home/giobon/go/bin
go mod tidy

# Build do bundle web (TipTap) se o diretório web existir
if [ -f "$BASE_DIR/web/package.json" ]; then
    echo "🌐 Compilando bundle web (TipTap)..."
    cd "$BASE_DIR/web"
    npm install --silent 2>/dev/null && node build.js 2>/dev/null
    cd "$BASE_DIR"
fi

echo "🔨 Compilando binário otimizado..."
export PATH=$PATH:/usr/local/go/bin:/home/giobon/go/bin
go run github.com/a-h/templ/cmd/templ@latest generate
go build -tags sqlite_fts5 -ldflags="-s -w" -o ton618 ./cmd/server/

if [ $? -eq 0 ]; then
    echo "🚀 Servidor rodando em http://localhost:$PORT"
    echo "📄 Logs em: $LOG_FILE"
    ./ton618 2>&1 | tee -a "$LOG_FILE"
else
    echo "❌ Erro na compilação. Abortando."
    exit 1
fi
