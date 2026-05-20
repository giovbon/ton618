# ─── Estágio 1: Build ───────────────────────────────────
FROM golang:1.24 AS builder

# Instalar dependências com suporte multi-arch completo
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    pkg-config \
    sqlite3 \
    libsqlite3-dev \
    ca-certificates \
    git \
    gcc-aarch64-linux-gnu \
    g++-aarch64-linux-gnu \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Cache de dependências
COPY go.mod go.sum ./
RUN go mod download

# Build com arquitetura correta (buildx injeta via ARG)
ARG TARGETARCH
ARG BUILDPLATFORM

# Configurar cross-compiler para ARM64
RUN if [ "$TARGETARCH" = "arm64" ]; then \
      export CC=aarch64-linux-gnu-gcc; \
      export CXX=aarch64-linux-gnu-g++; \
      export CGO_CFLAGS="-I/usr/aarch64-linux-gnu/include"; \
      export CGO_LDFLAGS="-L/usr/aarch64-linux-gnu/lib"; \
    fi

COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=${TARGETARCH} go build \
    -tags sqlite_fts5 \
    -ldflags="-s -w" \
    -o /ton618 ./cmd/server/

# ─── Estágio 2: Runtime ──────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates sqlite-libs tzdata

# Usuário não-root
RUN adduser -D -h /app appuser

WORKDIR /app

# Copia binário
COPY --from=builder /ton618 .

# Diretórios de dados
RUN mkdir -p /app/docs /app/data && chown -R appuser:appuser /app

USER appuser

VOLUME ["/app/docs", "/app/data"]

EXPOSE 6180

ENV DOCS_DIR=/app/docs
ENV DB_PATH=/app/data/ton618.db
ENV STATE_DIR=/app/data
ENV PORT=6180

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:6180/api/health || exit 1

ENTRYPOINT ["./ton618"]
