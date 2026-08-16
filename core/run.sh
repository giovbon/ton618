#!/bin/bash
# user: admin | pass: ton618

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

# Resolve o binário do Go em ambientes em que o toolchain fica em local não padrão
if ! command -v go >/dev/null 2>&1; then
    for candidate in \
        /usr/local/go/bin/go \
        /usr/lib/go/bin/go \
        "$HOME/go/bin/go" \
        /snap/go/current/bin/go; do
        if [ -x "$candidate" ]; then
            export PATH="$(dirname "$candidate"):$PATH"
            break
        fi
    done
fi

# Fallback para toolchains baixados pelo Go em $HOME/go/pkg/mod/.../bin/go
if ! command -v go >/dev/null 2>&1; then
    toolchain_go="$(find "$HOME/go/pkg/mod" -path '*/bin/go' 2>/dev/null | head -n 1 || true)"
    if [ -n "$toolchain_go" ] && [ -x "$toolchain_go" ]; then
        export PATH="$(dirname "$toolchain_go"):$PATH"
    fi
fi

# Centraliza caminhos do Go para o sistema e ambiente do usuário
export PATH="$PATH:/usr/local/go/bin:$HOME/go/bin:$HOME/.local/bin"

if ! command -v go >/dev/null 2>&1; then
    echo -e "${RED}❌ Go não foi encontrado no PATH. Instale o Go ou ajuste o caminho do toolchain.${NC}"
    exit 127
fi

# Carregar .env se existir
if [ -f "$BASE_DIR/.env" ]; then
    echo -e "${BLUE}📄 Carregando variáveis de .env${NC}"
    set -a; source "$BASE_DIR/.env"; set +a
fi

# Padrões seguros para variáveis obrigatórias
PORT="${PORT:-6180}"
DATA_DIR="${DATA_DIR:-$BASE_DIR/data}"
DOCS_DIR="${DOCS_DIR:-$BASE_DIR/docs}" # Fallback caso não esteja no .env
DB_PATH="${DB_PATH:-$DATA_DIR/ton618.db}"
EMBEDDING_ALL="${EMBEDDING_ALL:-false}"

export DOCS_DIR DATA_DIR

# 2. Interface e Logs Iniciais
echo -e "${BLUE}🌌 Iniciando TON-618 v2...${NC}"
echo -e "🗄️  Banco SQLite em: ${YELLOW}$DB_PATH${NC}"
echo -e "🔌 Porta:           ${YELLOW}$PORT${NC}"

# Garantir diretórios essenciais
mkdir -p "$DOCS_DIR" "$DATA_DIR"

# Limpar processos zumbis na porta de forma silenciosa
fuser -k "${PORT}/tcp" &> /dev/null || true

LOG_FILE="$DATA_DIR/ton618.log"
touch "$LOG_FILE"

# 3. Gerenciamento do ecossistema Go e Node
cd "$BASE_DIR"

# Otimização do Build Web (TipTap)
if [ -f "$BASE_DIR/web/package.json" ]; then
    cd "$BASE_DIR/web"
    # Só roda npm install se a pasta node_modules não existir (Ganho imenso de velocidade)
    if [ ! -d "node_modules" ]; then
        echo -e "${BLUE}🌐 Instalando dependências do módulo Web...${NC}"
        npm install --silent
    fi
    echo -e "${BLUE}🌐 Compilando bundle web (TipTap)...${NC}"
    node build.js --dev
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

# go mod tidy roda DEPOIS do templ generate: os *_templ.go são gitignored e não
# existem em checkout limpo — se o tidy rodar antes, ele remove a dependência do
# templ (github.com/a-h/templ) do go.mod e o build quebra com "no required module
# provides package".
echo "📦 Verificando dependências do Go..."
go mod tidy

echo -e "${BLUE}🔨 Compilando binário otimizado (SQLite FTS5)...${NC}"
# Adicionado flags para deixar o binário ainda menor e rápido no Go
go build -tags sqlite_fts5 -ldflags="-s -w" -o ton618 ./cmd/server/

# 5. Execução
echo -e "--------------------------------------------"
echo -e "${GREEN}🚀 Servidor rodando em http://localhost:$PORT${NC}"
echo -e "📄 Logs em: $LOG_FILE"
echo -e "--------------------------------------------"

# Atualiza a referência em segundo plano de forma silenciosa se o repomix estiver disponível
if command -v repomix &> /dev/null; then
    repomix --remove-comments --compress --ignore "**/*_test.go,**/*.html,data/**/*,docs/**/*,documents/**/*,node_modules/**/*,web/static/**/*,ton618,server" &> /dev/null &
fi

# Executa e joga pro log mantendo o output limpo
./ton618 2>&1 | tee -a "$LOG_FILE"