#!/bin/bash
# 🐳 TON-618 v2 — Deploy Multi-Arch para Docker Hub
# Uso: ./deploy.sh [tag] [modo]
# Modos:
#   1 -> Apenas ARM64 (Rápido, ideal para Raspberry Pi / Apple Silicon)
#   2 -> Multi-Arch (AMD64 + ARM64) [PADRÃO]

set -e

TAG=${1:-latest}
MODE=${2:-2}
IMAGE_NAME="${IMAGE_NAME:-giovbon/ton618plus}"

if [ "$MODE" == "1" ]; then
    PLATFORMS="linux/arm64"
    MSG="Apenas ARM64"
else
    PLATFORMS="linux/amd64,linux/arm64"
    MSG="Multi-Arch (AMD64 + ARM64)"
fi

echo "🚀 Iniciando deploy ($MSG)..."
echo "📦 Tag alvo: $TAG"
echo "🐳 Imagem: $IMAGE_NAME"

# 1. Garantir suporte multi-arch
echo "🔧 Verificando suporte multi-arch (qemu/binfmt)..."
docker run --privileged --rm tonistiigi/binfmt --install all > /dev/null 2>&1

# 2. Criar/usar builder multi-plataforma
if ! docker buildx inspect mybuilder > /dev/null 2>&1; then
    echo "🏗️  Criando novo builder 'mybuilder'..."
    docker buildx create --name mybuilder --use
else
    docker buildx use mybuilder
fi

docker buildx inspect --bootstrap

# 3. Build + Push
echo "🔨 Compilando para: $PLATFORMS"

docker buildx build \
  --platform "${PLATFORMS}" \
  -t "${IMAGE_NAME}:${TAG}" \
  -t "${IMAGE_NAME}:latest" \
  --push \
  .

echo "✅ Deploy concluído com sucesso!"
echo "🐳 Imagem: ${IMAGE_NAME}:${TAG}"
echo "💻 Plataformas: $PLATFORMS"
