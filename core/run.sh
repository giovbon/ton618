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
PID_FILE="$DATA_DIR/ton618.pid"

export DOCS_DIR DATA_DIR

# ── Porta: quem está escutando (PID) ─────────────────────────────────────────
# Motivo: instâncias antigas (ou SUSPENSAS — stat "T", típico de Ctrl+Z) seguram
# a porta; sem isso o servidor novo morre com "address already in use" e a porta
# fica "presa" sem explicação. `port_pids` usa o dono do socket via ss.
# Obs.: os "|| true" existem porque o script roda com `set -e`/`pipefail` e um
# grep sem match (porta livre) derrubaria o script.
port_pids() {
    ss -ltnpH "sport = :$PORT" 2>/dev/null | grep -oE 'pid=[0-9]+' | cut -d= -f2 | sort -u || true
}

wait_port_free() {
    local i
    for i in $(seq 1 25); do
        if [ -z "$(port_pids)" ]; then
            return 0
        fi
        sleep 0.2
    done
    return 1
}

free_port() {
    local pids
    pids="$(port_pids)"
    if [ -z "$pids" ]; then
        return 0
    fi
    echo -e "${YELLOW}🧹 Encerrando instância antiga na porta $PORT (pid: $(echo $pids | tr '\n' ' '))${NC}"
    kill -TERM $pids 2>/dev/null || true
    if wait_port_free; then return 0; fi
    kill -KILL $pids 2>/dev/null || true
    if wait_port_free; then return 0; fi
    echo -e "${RED}❌ A porta $PORT continua ocupada (pid: $(port_pids | tr '\n' ' ')).${NC}"
    echo -e "${RED}   Rode './run.sh stop' ou libere a porta manualmente.${NC}"
    return 1
}

cmd_stop() {
    local pids
    pids="$(port_pids)"
    if [ -z "$pids" ] && [ -f "$PID_FILE" ]; then
        local pid
        pid="$(cat "$PID_FILE" 2>/dev/null || true)"
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            pids="$pid"
        fi
    fi
    if [ -z "$pids" ]; then
        echo -e "${YELLOW}⏸️  Nada rodando na porta $PORT.${NC}"
        rm -f "$PID_FILE"
        return 0
    fi
    echo -e "${BLUE}🛑 Parando TON-618 (pid: $(echo $pids | tr '\n' ' '))...${NC}"
    kill -TERM $pids 2>/dev/null || true
    if ! wait_port_free; then
        kill -KILL $pids 2>/dev/null || true
        wait_port_free || true
    fi
    rm -f "$PID_FILE"
    echo -e "${GREEN}✅ Porta $PORT livre.${NC}"
}

cmd_status() {
    local pids
    pids="$(port_pids)"
    if [ -z "$pids" ]; then
        echo -e "${YELLOW}⏸️  TON-618 não está rodando na porta $PORT.${NC}"
        return 0
    fi
    echo -e "${GREEN}✅ TON-618 rodando na porta $PORT${NC}"
    ss -ltnpH "sport = :$PORT" 2>/dev/null || true
    local p st
    for p in $pids; do
        st="$(ps -o stat= -p "$p" 2>/dev/null | tr -d ' ' || true)"
        case "$st" in
            T*)
                echo -e "${YELLOW}⚠️  pid $p está SUSPENSO (stat '$st'): a porta fica presa, sem responder.${NC}"
                echo -e "${YELLOW}   Rode './run.sh stop' para encerrar e depois suba de novo.${NC}"
                ;;
        esac
    done
}

# Subcomandos que NÃO compilam nada (não exigem Go nem Node instalados).
if [ "${1:-}" = "stop" ] || [ "${1:-}" = "status" ]; then
    cmd_"${1}"
    exit $?
fi

if ! command -v go >/dev/null 2>&1; then
    echo -e "${RED}❌ Go não foi encontrado no PATH. Instale o Go ou ajuste o caminho do toolchain.${NC}"
    exit 127
fi

# 2. Interface e Logs Iniciais
echo -e "${BLUE}🌌 Iniciando TON-618 v2...${NC}"
echo -e "🗄️  Banco SQLite em: ${YELLOW}$DB_PATH${NC}"
echo -e "🔌 Porta:           ${YELLOW}$PORT${NC}"

# Garantir diretórios essenciais
mkdir -p "$DOCS_DIR" "$DATA_DIR"

# Libera a porta ANTES de compilar (o build pode demorar; melhor já deixar limpo).
free_port || exit 1

LOG_FILE="$DATA_DIR/ton618.log"
touch "$LOG_FILE"

# 3. Gerenciamento do ecossistema Go e Node
cd "$BASE_DIR"

# Otimização do Build Web (TipTap)
if [ -f "$BASE_DIR/web/package.json" ]; then
    cd "$BASE_DIR/web"
    # Só roda npm ci se a pasta node_modules não existir (ganho imenso de
    # velocidade). ⚠️ A pasta pode existir INCOMPLETA (install interrompido ou
    # ferramenta que não instalou tudo) — nesse caso o build do esbuild quebra
    # com "Could not resolve ...". Por isso validamos alguns pacotes que o
    # bundle usa de verdade antes de pular a instalação.
    web_deps_ok=1
    if [ ! -d "node_modules" ]; then
        web_deps_ok=0
    else
        for dep in esbuild markmap-common markmap-lib markmap-view; do
            if [ ! -d "node_modules/$dep" ]; then
                web_deps_ok=0
                break
            fi
        done
    fi
    if [ "$web_deps_ok" -eq 0 ]; then
        echo -e "${GREEN}🌐 Instalando dependências do módulo Web (node_modules ausente/incompleto)...${NC}"
        npm ci --legacy-peer-deps --silent
    fi
    echo -e "${BLUE}🌐 Compilando bundle web (TipTap)...${NC}"
    node build.js --dev
    cd "$BASE_DIR"
fi

# 4. Geração de Templates e Compilação
echo -e "${BLUE}🔨 Gerando componentes Templ...${NC}"
# Usa o CLI templ local APENAS se a versão for >= à exigida no go.mod (rápido).
# Se estiver ausente ou desatualizado, usa `go run` com a versão pinada (determinístico).
# Motivo: um templ antigo no PATH (ex: v0.2.x) não parseia os .templ atuais e
# quebra o build com "if/for/case: expected nodes, but none were found".
TEMPL_REQ="$(grep -E 'github.com/a-h/templ[[:space:]]' go.mod | awk '{print $2}' | head -n1 || true)"
TEMPL_USE_LOCAL=0
if [ -n "$TEMPL_REQ" ] && command -v templ &> /dev/null; then
    TEMPL_LOCAL="$(templ --version 2>/dev/null | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | head -n1 || true)"
    # local >= requerida ⇒ usa o binário local
    if [ -n "$TEMPL_LOCAL" ] && [ "$(printf '%s\n%s\n' "$TEMPL_REQ" "$TEMPL_LOCAL" | sort -V | tail -n1)" = "$TEMPL_LOCAL" ]; then
        TEMPL_USE_LOCAL=1
    fi
fi

if [ "$TEMPL_USE_LOCAL" -eq 1 ]; then
    templ generate
else
    if [ -n "$TEMPL_REQ" ]; then
        echo -e "${YELLOW}⚠️  templ local ausente/desatualizado — usando go run templ@${TEMPL_REQ}${NC}"
        go run github.com/a-h/templ/cmd/templ@"$TEMPL_REQ" generate
    else
        echo -e "${YELLOW}⚠️  templ não encontrado no go.mod — usando go run templ@latest${NC}"
        go run github.com/a-h/templ/cmd/templ@latest generate
    fi
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

# Última checagem da porta (o build pode ter levado minutos) e PID para o `stop`.
free_port || exit 1
echo $$ > "$PID_FILE"

# Ctrl+Z passa a ser IGNORADO (disposição herdada pelo exec): suspender o
# servidor deixaria a porta presa, aceitando conexões e respondendo nada — foi
# a origem dos "processos fantasmas" (stat T) que apareciam depois de rodar.
trap '' TSTP
echo -e "${YELLOW}💡 Ctrl+C encerra o servidor. Se algo sobrar: ./run.sh stop | ./run.sh status${NC}"

# `exec` substitui ESTE shell pelo servidor: não sobra processo-pai órfão e o
# Ctrl+C vai direto para ele. O `tee` mantém o log em arquivo e a saída na tela.
exec ./ton618 > >(tee -a "$LOG_FILE") 2>&1