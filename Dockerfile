# ─── Estágio 1: Build ───────────────────────────────────
FROM golang:1.24 AS builder

# Instalações mínimas (sem CGO necessário)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    git \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Cache de dependências
COPY go.mod go.sum ./
RUN go mod download && go mod tidy

# Build SEM CGO (driver sqlite puro Go)
ARG TARGETARCH
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -ldflags="-s -w" \
    -o /ton618 ./cmd/server/

# ─── Estágio 2: Runtime ──────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

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
