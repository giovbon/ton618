# ─── Estágio 1: Build do bundle web ────────────────
FROM --platform=$BUILDPLATFORM node:24-alpine AS web-builder

WORKDIR /web

# 1. Instala dependências de forma isolada (npm ci: instalação determinística a partir do lockfile)
COPY web/package.json web/package-lock.json ./
RUN npm ci --legacy-peer-deps

# 2. Copia todo o fonte (web + backend .templ para o Tailwind escanear)
COPY web/ .
COPY internal/ ./internal/

# 3. Compila os assets estáticos (app.css, editor.js, etc.)
RUN node build.js

# ─── Estágio 2: Build Go ────────────────────────────
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

RUN apk add --no-cache ca-certificates upx curl

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

# Pina o CLI do templ à MESMA versão declarada no go.mod (mesma lógica do
# run.sh e dos workflows). ⚠️ `@latest` quebra o build assim que sai uma versão
# nova: o código gerado por um templ mais novo usa APIs de runtime que ainda não
# existem na versão de github.com/a-h/templ fixada no go.mod (ex.: v0.3.1100
# gera templruntime.NewAttributeExpression, ausente na v0.3.1070) — e o
# `go mod tidy` abaixo NÃO atualiza essa dependência, então o `go build` falha.
RUN TEMPL_REQ="$(grep -E 'github.com/a-h/templ[[:space:]]' go.mod | awk '{print $2}' | head -n1 || true)" && \
    if [ -n "$TEMPL_REQ" ]; then \
      go install "github.com/a-h/templ/cmd/templ@${TEMPL_REQ}"; \
    else \
      go install github.com/a-h/templ/cmd/templ@latest; \
    fi

ARG TARGETARCH
ARG TARGETVARIANT
ARG BUILD_TAGS="sqlite_fts5"
COPY . .

# Copia o bundle compilado (contendo os modelos originais + .gz + .br)
COPY --from=web-builder /web/static ./web/static

# Gerar código do templ antes de compilar
RUN templ generate

# go mod tidy DEPOIS do templ generate: os *_templ.go são gitignored e não
# existem no COPY acima, então o go.mod commitado (que o Dependabot "tidi" sem os
# _templ.go) não exige github.com/a-h/templ. Tidy após o generate restaura a
# dependência (mesma lógica do run.sh e do pr_checks.yml), senão o go build
# quebra com "no required module provides package github.com/a-h/templ".
RUN go mod tidy

RUN GOARM=$(echo "${TARGETVARIANT}" | sed 's/v//') && \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} GOARM=${GOARM} go build \
    -tags "${BUILD_TAGS}" \
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

RUN apk add --no-cache ca-certificates tzdata

# Copia as fontes baixadas do builder
COPY --from=builder /app/fonts /usr/share/fonts/truetype/fira-sans

RUN adduser -D -h /app appuser

WORKDIR /app

COPY --from=builder /ton618 .
# Otimização: Copia estritamente a pasta static compilada e os metadados.
# Isso impede que o node_modules de desenvolvimento infle os 271 MB da imagem.
RUN mkdir -p /app/web/static
COPY --from=builder /app/web/static /app/web/static
COPY --from=builder /app/web/package.json /app/web/package.json

# Documentação de Ajuda: o help (/help) lê o HELP.md. O contexto do build é
# ./core, então ele vive em core/HELP.md (é o mesmo conteúdo do HELP.md na raiz
# do repo). Copia para /app/HELP.md e aponta HELP_DOC para ele.
COPY --from=builder /app/HELP.md ./HELP.md

RUN mkdir -p /app/docs /app/data && chmod 777 /app/docs /app/data

VOLUME ["/app/docs", "/app/data"]

EXPOSE 6180

ENV DOCS_DIR=/app/docs \
    DB_PATH=/app/data/ton618.db \
    STATE_DIR=/app/data \
    WEB_DIR=/app/web \
    HELP_DOC=/app/HELP.md \
    PORT=6180

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:6180/api/health || exit 1

CMD chmod -R 777 /app/data /app/docs 2>/dev/null || true && exec ./ton618