# ─── Estágio 1: Build do bundle web ────────────────
FROM node:20 AS web-builder

WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm install
COPY web/ .
RUN node build.js

# ─── Estágio 2: Build Go ────────────────────────────
FROM golang:1.24 AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    git \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && go mod tidy

# Instalar templ e dependências
RUN go install github.com/a-h/templ/cmd/templ@latest

ARG TARGETARCH
COPY . .

# Copia bundle web do estágio anterior
COPY --from=web-builder /web/static/editor.js web/static/editor.js
COPY --from=web-builder /web/static/editor.js.gz web/static/editor.js.gz

# Gerar código do templ antes de compilar
RUN templ generate

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -tags sqlite_fts5 \
    -ldflags="-s -w" \
    -o /ton618 ./cmd/server/

# ─── Estágio 3: Runtime ──────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

RUN adduser -D -h /app appuser

WORKDIR /app

COPY --from=builder /ton618 .
COPY --from=builder /app/web /app/web

# Garante permissão de escrita nos volumes
RUN mkdir -p /app/docs /app/data && chown -R appuser:appuser /app

USER appuser

VOLUME ["/app/docs", "/app/data"]

EXPOSE 6180

ENV DOCS_DIR=/app/docs
ENV DB_PATH=/app/data/ton618.db
ENV STATE_DIR=/app/data
ENV WEB_DIR=/app/web
ENV PORT=6180

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:6180/api/health || exit 1

ENTRYPOINT ["./ton618"]
