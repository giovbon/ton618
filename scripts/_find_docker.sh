#!/usr/bin/env bash
echo "=== containers ==="
docker ps -a --format '{{.ID}}  {{.Image}}  {{.Status}}  {{.Ports}}  {{.Names}}' 2>&1 | head -20
echo "=== volumes ==="
docker volume ls 2>&1 | head -20
echo "=== compose files no repo ==="
ls -la /mnt/c/Users/Giovani/Desktop/Code/ton618/core/docker-compose.yml 2>&1
