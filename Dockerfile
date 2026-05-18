# ─── Estágio 1: Build ───────────────────────────────────
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# Cache de dependências
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -tags sqlite_fts5 -ldflags="-s -w" -o /ton618 ./cmd/server/

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
