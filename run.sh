#!/bin/bash
# Atalho de conveniência. Uso:
#   ./run.sh                 → sobe o servidor (equivale a ./core/run.sh)
#   ./run.sh stop            → encerra o servidor que estiver na porta
#   ./run.sh status          → mostra se está rodando (e se ficou suspenso)
#   ./run.sh push "mensagem" → git add/commit/push

command1="${1:-}"
command2="${2:-}"

case "$command1" in
  "")
    ./core/run.sh
    ;;
  stop|status)
    # Repassado para o core/run.sh (não compila nada — só mexe na porta).
    ./core/run.sh "$command1"
    ;;
  push)
    git add .
    git commit -m "$command2"
    git push origin main
    ;;
  *)
    echo "Uso: ./run.sh [stop|status|push \"mensagem\"]" >&2
    exit 1
    ;;
esac
