#!/bin/bash
# 🌌 Vortex Engine - Initializer (Final Robust Version)
# TON-618 Knowledge Singularity

# Identificar o diretório raiz do projeto de forma absoluta
BASE_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
DOCS_DIR="$BASE_DIR/docs"
WEB_DIR="$BASE_DIR/web/dist"

# Usar um caminho NEUTRO para o índice (evita problemas com espaços/acentos na Área de Trabalho)
BLEVE_INDEX_DIR="/home/giobon/ton618_data/ton618.bleve"
STATE_DIR="/home/giobon/ton618_data"
PORT="6180"

export DOCS_DIR
export WEB_DIR
export PORT
export BLEVE_INDEX_DIR
export STATE_DIR
export AUTH_USER="admin"
export AUTH_PASS="ton618_secret"

echo "🌌 Iniciando Singularidade TON-618..."
echo "📂 Monitorando documentos em: $DOCS_DIR"
echo "🌐 Servindo Frontend de: $WEB_DIR"
echo "🧠 Banco de Dados Bleve em: $BLEVE_INDEX_DIR"
echo "🔌 Porta: $PORT"

# Limpar processos zumbis que possam estar na porta
fuser -k "${PORT}/tcp" &> /dev/null || true

# Configurar arquivo de Log
LOG_FILE="$STATE_DIR/vortex.log"
touch "$LOG_FILE"

# Entrar na pasta etl, compilar e rodar o binário
cd "$BASE_DIR/etl"
echo "🔨 Compilando binário otimizado..."
go build -o ../vortex_bin main.go

if [ $? -eq 0 ]; then
    echo "🚀 Lançando motor TON-618 (Modo Monitoramento Ativo)..."
    cd ..
    # Rodar em primeiro plano e salvar no log ao mesmo tempo
    ./vortex_bin 2>&1 | tee -a "$LOG_FILE"
else
    echo "❌ Erro na compilação. Abortando."
    exit 1
fi
