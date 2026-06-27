# ─── Estágio 1: Build do bundle web ────────────────
FROM node:20-alpine AS web-builder

WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm install --legacy-peer-deps
COPY web/ .
RUN node build.js

# Remove arquivos não-comprimidos — o servidor sempre serve o .gz,
# os originais nunca são usados no runtime (economiza ~9 MB na imagem final)
RUN find static -maxdepth 1 -name "*.js"  ! -name "*.gz" -delete && \
    find static -maxdepth 1 -name "*.css" ! -name "*.gz" -delete

# ─── Estágio 2: Build Go ────────────────────────────
FROM golang:1.25-alpine AS builder

# modernc.org/sqlite é pure Go → CGO_ENABLED=0, não precisa de gcc nem git
RUN apk add --no-cache ca-certificates upx curl

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

# Instalar templ
RUN go install github.com/a-h/templ/cmd/templ@latest

ARG TARGETARCH
COPY . .

# Copia bundle web (apenas .gz) do estágio anterior
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

# --chown direto elimina a camada de chown -R separada
COPY --from=builder --chown=appuser:appuser /ton618 .
COPY --from=builder --chown=appuser:appuser /app/web ./web

RUN mkdir -p /app/docs /app/data && chown appuser:appuser /app/docs /app/data

USER appuser

VOLUME ["/app/docs", "/app/data"]

EXPOSE 6180

ENV DOCS_DIR=/app/docs \
    DB_PATH=/app/data/ton618.db \
    STATE_DIR=/app/data \
    WEB_DIR=/app/web \
    PORT=6180

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:6180/api/health || exit 1

ENTRYPOINT ["./ton618"]
