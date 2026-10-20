#!/bin/bash
# Valida gofmt do jeito que o CI faz (checkout LF), mas a partir do worktree Windows (CRLF).
set -u
docker run --rm -v /mnt/c/Users/Giovani/Desktop/Code/ton618/core:/app -w /app golang:1.26-alpine sh -c '
set -e
cp -r /app /tmp/w
cd /tmp/w
# normaliza CRLF -> LF em todos os .go (o CI faz checkout LF via .gitattributes)
find . -name "*.go" -type f -exec sh -c "for f; do tr -d \"\\r\" < \"\$f\" > \"\$f.tmp\" && mv \"\$f.tmp\" \"\$f\"; done" sh {} +
echo "=== gofmt -l . ==="
gofmt -l .
echo "=== fim (vazio acima = OK) ==="
'
