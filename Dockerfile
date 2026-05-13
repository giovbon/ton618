# ======== STAGE 1: FRONTEND (Vite + Preact) ========
FROM node:20-alpine AS builder-web
WORKDIR /app/web

# Cache otimizado: Copia apenas os arquivos de dependência primeiro
COPY web/package*.json ./
# npm ci é mais rápido e seguro para builds do que npm install
RUN npm ci

COPY web/ .

# Permite pular testes com --build-arg SKIP_TESTS=true
ARG SKIP_TESTS=false
RUN if [ "$SKIP_TESTS" = "false" ]; then npm run test; else echo "⏩ Pulando testes do Frontend..."; fi
RUN npm run build

# ======== STAGE 2: BACKEND (Golang) ========
FROM golang:1.25-alpine AS builder-go
WORKDIR /app/etl

# Cache otimizado: Copia mod/sum antes de baixar
COPY etl/go.mod etl/go.sum* ./
RUN go mod download

# Copia o código fonte (go mod tidy removido para não quebrar o cache)
COPY etl/ .

# Permite pular testes com --build-arg SKIP_TESTS=true
ARG SKIP_TESTS=false
RUN if [ "$SKIP_TESTS" = "false" ]; then go test -v ./...; else echo "⏩ Pulando testes do Backend..."; fi

# Compilação limpa
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o ton-server main.go

# ======== STAGE 3: RUNNER (Alpine Minimalista) ========
FROM alpine:3.19
WORKDIR /app

# Agrupa instalação de dependências e limpa o cache da apk numa camada só
RUN apk --no-cache add ca-certificates tzdata poppler-utils

COPY --from=builder-web /app/web/dist ./web/dist
COPY --from=builder-go /app/etl/ton-server .

# Agrupar ENV vars reduz o número de camadas no Docker
ENV PORT=6180 \
    WEB_DIR="./web/dist" \
    DOCS_DIR="/docs" \
    BLEVE_INDEX_DIR="/app/data/ton618.bleve" \
    POLL_INTERVAL_SEC=30

# Garantir que os diretórios existam
RUN mkdir -p /app/data /app/state /docs

EXPOSE 6180

CMD ["./ton-server"]
