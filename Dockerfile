# ─── Estágio 1: Build do bundle web ────────────────
FROM node:20-alpine AS web-builder

# Instala ferramentas de compressão nativas do Linux no Alpine (muito mais rápidas que JS)
RUN apk add --no-cache gzip brotli

WORKDIR /web

# 1. Instala dependências de forma isolada
COPY web/package.json web/package-lock.json ./
RUN npm install --legacy-peer-deps

# 2. Copia apenas os scripts de download de modelos
COPY web/download_model.js ./
COPY web/static/models/download-ort.js ./static/models/

# 3. Executa o download dos modelos de IA
RUN node download_model.js
RUN node static/models/download-ort.js

# 4. COMPRIME OS MODELOS AQUI (Fica salvo no cache do Docker/GitHub Actions)
# Isso gera os arquivos .gz e .br nativamente e os deixa salvos na pasta.
# O find busca todos os arquivos .onnx e .bin dentro de static/models/ e os comprime.
RUN find static/models/ -type f \( -name "*.onnx" -o -name "*.bin" \) | while read file; do \
      echo "Pré-comprimindo modelo cacheado: $file"; \
      gzip -9 -c "$file" > "$file.gz" && \
      brotli -q 11 -c "$file" > "$file.br"; \
    done

# 5. Copia o resto do código fonte do frontend e do backend
# Como os .gz e .br já existem na pasta, o COPY não vai sobrescrevê-los se o .dockerignore estiver correto.
COPY web/ .
COPY internal/ ./internal/

# 6. Compila os assets estáticos do seu código (app.css, editor.js, etc.)
# IMPORTANTE: Garanta que o seu web/build.js apenas ignore arquivos .onnx/.bin se eles já possuírem
# os equivalentes .gz/.br na pasta, ou configure-o para pular a pasta static/models/
RUN node build.js

# ─── Estágio 2: Build Go ────────────────────────────
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache ca-certificates upx curl

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

RUN go install github.com/a-h/templ/cmd/templ@latest

ARG TARGETARCH
COPY . .

# Copia o bundle compilado (contendo os modelos .onnx originais + .gz + .br)
COPY --from=web-builder /web/static ./web/static

# Gerar código do templ antes de compilar
RUN templ generate

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -tags sqlite_fts5 \
    -ldflags="-s -w" \
    -o /ton618 ./cmd/server/

# Compacta o binário com UPX
RUN upx --best --lzma /ton618

# Baixa as fontes Fira Sans Regular e Bold diretamente do repositório Google Fonts
RUN mkdir -p /app/fonts && \
    curl -sSL -o /app/fonts/FiraSans-Regular.ttf https://github.com/google/fonts/raw/main/ofl/firasans/FiraSans-Regular.ttf && \
    curl -sSL -o /app/fonts/FiraSans-Bold.ttf https://github.com/google/fonts/raw/main/ofl/firasans/FiraSans-Bold.ttf

# ─── Estágio 3: Runtime ──────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata typst

# Copia as fontes baixadas do builder
COPY --from=builder /app/fonts /usr/share/fonts/truetype/fira-sans

RUN adduser -D -h /app appuser

WORKDIR /app

COPY --from=builder /ton618 .
COPY --from=builder /app/web ./web
COPY entrypoint.sh .

RUN mkdir -p /app/docs /app/data && chmod 777 /app/docs /app/data && chmod +x /app/entrypoint.sh

VOLUME ["/app/docs", "/app/data"]

EXPOSE 6180

ENV DOCS_DIR=/app/docs \
    DB_PATH=/app/data/ton618.db \
    STATE_DIR=/app/data \
    WEB_DIR=/app/web \
    PORT=6180

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:6180/api/health || exit 1

ENTRYPOINT ["/app/entrypoint.sh"]