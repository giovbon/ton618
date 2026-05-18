#!/bin/bash

# Script para automatizar o build multi-arch e push para o Docker Hub
# Uso: ./deploy.sh [tag] [modo]
# Modos:
#   1 -> Apenas ARM64 (Rápido, ideal para Raspberry Pi / Apple Silicon)
#   2 -> Multi-Arch (AMD64 + ARM64) [PADRÃO]

TAG=${1:-latest}
MODE=${2:-2}
IMAGE_NAME="giovbon/ton618pkm"

if [ "$MODE" == "1" ]; then
    PLATFORMS="linux/arm64"
    MSG="Apenas ARM64"
else
    PLATFORMS="linux/amd64,linux/arm64"
    MSG="Multi-Arch (AMD64 + ARM64)"
fi

echo "🚀 Iniciando deploy ($MSG)..."
echo "📦 Tag alvo: $TAG"

# 1. Garantir que o suporte a multi-arch esteja ativo no Docker
echo "🔧 Verificando suporte multi-arch (qemu/binfmt)..."
docker run --privileged --rm tonistiigi/binfmt --install all > /dev/null 2>&1

# 2. Criar ou usar um builder que suporte multi-plataforma
if ! docker buildx inspect mybuilder > /dev/null 2>&1; then
    echo "🏗️ Criando novo builder 'mybuilder'..."
    docker buildx create --name mybuilder --use
else
    docker buildx use mybuilder
fi

docker buildx inspect --bootstrap

# 3. Executar o build e o push simultâneos
echo "🔨 Compilando para: $PLATFORMS"

# Configura os argumentos de build em um array para maior robustez
BUILD_ARGS=(
  --platform "${PLATFORMS}"
  --build-arg SKIP_TESTS=true
  -t "${IMAGE_NAME}:${TAG}"
  -t "${IMAGE_NAME}:latest"
  --push
  .
)

# O buildx com múltiplas plataformas e --push cria uma imagem manifest list
docker buildx build "${BUILD_ARGS[@]}"

if [ $? -eq 0 ]; then
    echo "✅ Deploy concluído com sucesso!"
    echo "🐳 Imagem: ${IMAGE_NAME}:${TAG}"
    echo "💻 Plataformas: $PLATFORMS"
else
    echo "❌ Erro durante o processo de build/push."
    exit 1
fi

# esse script faz o build multi-arch (amd64 + arm64)