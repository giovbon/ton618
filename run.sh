#!/bin/bash
# user: admin | pass: ton618
# 🌌 TON-618 v2.2 — Runner & Builder
# Motor de Busca Personal Knowledge Management

set -euo pipefail # Interrompe o script se houver erros ou variáveis nulas

# Cores para o terminal
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# 1. Configuração de Diretórios e PATH
BASE_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
DATA_DIR="$BASE_DIR/data"

# Centraliza caminhos do Go para o sistema e ambiente do usuário
export PATH="$PATH:/usr/local/go/bin:$HOME/go/bin"

# Carregar .env se existir
if [ -f "$BASE_DIR/.env" ]; then
    echo -e "${BLUE}📄 Carregando variáveis de .env${NC}"
    set -a; source "$BASE_DIR/.env"; set +a
fi

# Padrões seguros para variáveis obrigatórias
PORT="${PORT:-6180}"
DATA_DIR="${DATA_DIR:-$BASE_DIR/data}"
DOCS_DIR="${DOCS_DIR:-$BASE_DIR/documents}" # Fallback caso não esteja no .env
DB_PATH="${DB_PATH:-$DATA_DIR/ton618.db}"
EMBEDDING_ALL="${EMBEDDING_ALL:-false}"

export DOCS_DIR DATA_DIR

# 2. Interface e Logs Iniciais
echo -e "${BLUE}🌌 Iniciando TON-618 v2...${NC}"
echo -e "🗄️  Banco SQLite em: ${YELLOW}$DB_PATH${NC}"
echo -e "🔌 Porta:           ${YELLOW}$PORT${NC}"

if [ -n "${EMBEDDING_API_KEY:-}" ]; then
    echo -e "🧠 Modo Semântico:  ${GREEN}Ativo (API Key configurada)${NC}"
else
    echo -e "🧠 Modo Semântico:  ${YELLOW}Desativado (Apenas busca local/FTS5)${NC}"
fi

# Garantir diretórios essenciais
mkdir -p "$DOCS_DIR" "$DATA_DIR"

# Limpar processos zumbis na porta de forma silenciosa
fuser -k "${PORT}/tcp" &> /dev/null || true

LOG_FILE="$DATA_DIR/ton618.log"
touch "$LOG_FILE"

# 3. Gerenciamento do ecossistema Go e Node
cd "$BASE_DIR"

echo -e "${BLUE}📦 Verificando dependências do Go...${NC}"
go mod tidy

# Otimização do Build Web (TipTap)
if [ -f "$BASE_DIR/web/package.json" ]; then
    cd "$BASE_DIR/web"
    # Só roda npm install se a pasta node_modules não existir (Ganho imenso de velocidade)
    if [ ! -d "node_modules" ]; then
        echo -e "${BLUE}🌐 Instalando dependências do módulo Web...${NC}"
        npm install --silent
    fi
    echo -e "${BLUE}🌐 Compilando bundle web (TipTap)...${NC}"
    node build.js 2>/dev/null
    cd "$BASE_DIR"
fi

# 4. Geração de Templates e Compilação
echo -e "${BLUE}🔨 Gerando componentes Templ...${NC}"
# Se o templ já estiver instalado no PATH, usa local. Se não, usa o go run (mais lento).
if command -v templ &> /dev/null; then
    templ generate
else
    go run github.com/a-h/templ/cmd/templ@latest generate
fi

echo -e "${BLUE}🔨 Compilando binário otimizado (SQLite FTS5)...${NC}"
# Adicionado flags para deixar o binário ainda menor e rápido no Go
go build -tags sqlite_fts5 -ldflags="-s -w" -o ton618 ./cmd/server/

# 5. Execução
echo -e "--------------------------------------------"
echo -e "${GREEN}🚀 Servidor rodando em http://localhost:$PORT${NC}"
echo -e "📄 Logs em: $LOG_FILE"
echo -e "--------------------------------------------"

# Executa e joga pro log mantendo o output limpo
./ton618 2>&1 | tee -a "$LOG_FILE"