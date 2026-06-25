#!/bin/bash
export PATH=$PATH:/usr/local/go/bin:/home/giobon/go/bin
cd /home/giobon/ton618plus

echo "=== Antes ==="
git status --short

# Adiciona tudo explicitamente
git add -u
git add go.mod go.sum Dockerfile .dockerignore docker-compose.yml deploy.sh README.md

echo "=== Após git add ==="
git status --short

# Commit
git commit -m "fix: downgrade go 1.25.0 -> 1.24 no go.mod (Go 1.25 nao existe no Docker Hub); otimiza Dockerfile e docker-compose para RPi"

echo "=== Push ==="
git push origin main
echo "PUSH EXIT: $?"
