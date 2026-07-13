# ─── Estágio 1: Build do bundle web ────────────────
FROM node:20-alpine AS web-builder

WORKDIR /web

# 1. Instala dependências de forma isolada
COPY web/package.json web/package-lock.json ./
RUN npm install --legacy-peer-deps

# 2. Copia apenas os scripts de download de modelos
COPY web/download_model.js ./
COPY web/static/models/download-ort.js ./static/models/

# 3. Executa o download E a compressão dos modelos de IA e tokenizers
# Como o script nativamente já gera as versões .gz e .br, esta única camada
# resolve o download e a compactação de forma totalmente cacheável pelo GHA.
RUN node download_model.js
RUN node static/models/download-ort.js

# 4. Copia o resto do código fonte do frontend e do backend
COPY web/ .
COPY internal/ ./internal/

# 5. Compila os assets estáticos do seu código (app.css, editor.js, etc.)
# Certifique-se de que o build.js ignore a pasta static/models para não reprocessar.
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

# Copia o bundle compilado (contendo os modelos originais + .gz + .br)
COPY --from=web-builder /web/static ./web/static

# Gerar código do templ antes de compilar
RUN templ generate

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -tags sqlite_fts5 \
    -ldflags="-s -w" \
    -o /ton618 ./cmd/server/

# Compacta o binário com UPX
RUN upx --best --lzma /ton618

# Otimização: Baixa os subsets latinos leves das fontes (~300KB cada) em vez do repositório inteiro
RUN mkdir -p /app/fonts && \
    curl -sSL -o /app/fonts/FiraSans-Regular.ttf "https://fonts.gstatic.com/s/firasans/v17/va9E4kDNxGzdMffhnd631w.ttf" && \
    curl -sSL -o /app/fonts/FiraSans-Bold.ttf "https://fonts.gstatic.com/s/firasans/v17/va9B4kDNxGzdMffhnd631u04wA.ttf"

# ─── Estágio 3: Runtime ──────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata typst

# Copia as fontes baixadas do builder
COPY --from=builder /app/fonts /usr/share/fonts/truetype/fira-sans

RUN adduser -D -h /app appuser

WORKDIR /app

COPY --from=builder /ton618 .
COPY entrypoint.sh .

# Otimização: Copia estritamente a pasta static compilada e os metadados.
# Isso impede que o node_modules de desenvolvimento infle os 271 MB da imagem.
RUN mkdir -p /app/web/static
COPY --from=builder /app/web/static /app/web/static
COPY --from=builder /app/web/package.json /app/web/package.json
COPY --from=builder /app/entrypoint.sh .

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