#!/bin/bash
set -e
export PATH=$PATH:/usr/local/go/bin:/home/giobon/go/bin
cd /home/giobon/ton618plus

echo "=== Antes ==="
head -4 go.mod | cat -A

# Cria novo go.mod com versão corrigida
python3 - <<'PYEOF'
with open('go.mod', 'r') as f:
    content = f.read()
content = content.replace('go 1.25.0', 'go 1.24', 1)
with open('go.mod', 'w') as f:
    f.write(content)
print("go.mod fixed")
PYEOF

echo "=== Depois ==="
head -4 go.mod | cat -A
xxd go.mod | head -3

echo "=== git diff ==="
git diff go.mod

go mod tidy
echo "TIDY: $?"

git add go.mod go.sum
git status --short

git commit -m "fix: go.mod go 1.25.0 -> 1.24 para compatibilidade com Docker Hub"
git push origin main
echo "DONE"
