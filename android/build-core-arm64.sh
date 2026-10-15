#!/bin/bash
# ==============================================================================
# Compilação do TON-618 Core para Android + sincronização dos assets web
# ==============================================================================
# Gera:
#   app/src/main/jniLibs/<abi>/libton618.so   → binário Go (executado pelo app)
#   app/src/main/assets/web/static/**         → bundle web (CSS/JS/fontes)
#
# ⚠️ Os assets precisam ser copiados RECURSIVAMENTE (js/, js/vendor/, fonts/).
# Sem js/vendor/htmx.min.js, js/vendor/alpine.min.js e js/app.js a página abre
# mas nenhum recurso funciona (404 em /static/js/*).
#
# Uso:
#   ./build-core-arm64.sh                       # arm64-v8a (celular real)
#   ANDROID_ABI=x86_64 ./build-core-arm64.sh     # emulador
#   ANDROID_SEMANTIC=1 ./build-core-arm64.sh     # inclui onnxruntime (+22 MB)
# ==============================================================================
set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
CORE_DIR="$ROOT_DIR/core"
ANDROID_ABI="${ANDROID_ABI:-arm64-v8a}"
JNILIBS_DIR="$SCRIPT_DIR/app/src/main/jniLibs/$ANDROID_ABI"
ASSETS_DIR="$SCRIPT_DIR/app/src/main/assets"
STATIC_ASSETS="$ASSETS_DIR/web/static"

# Mesma versão fixada em core/go.mod — evita divergência silenciosa de templates.
TEMPL_VERSION="v0.3.1020"

case "$ANDROID_ABI" in
    arm64-v8a)   GOARCH="arm64" ;;
    x86_64)      GOARCH="amd64" ;;
    armeabi-v7a) GOARCH="arm"   ;;
    *) echo "❌ ANDROID_ABI inválido: $ANDROID_ABI (use arm64-v8a, x86_64 ou armeabi-v7a)"; exit 1 ;;
esac

mkdir -p "$ASSETS_DIR" "$JNILIBS_DIR"

echo "🌌 Compilando TON-618 Core para Android ($ANDROID_ABI → GOARCH=$GOARCH)..."

if ! command -v go >/dev/null 2>&1; then
    echo "❌ Go não encontrado no PATH."
    exit 127
fi

# 1. Garante que os templates Templ estão gerados
#    ($TEMPL_VERSION espelha o core/go.mod; '*_templ.go' é gitignored)
echo "🔨 Gerando templates templ ($TEMPL_VERSION)..."
cd "$CORE_DIR"
go run "github.com/a-h/templ/cmd/templ@$TEMPL_VERSION" generate

# 2. Binário Go
#    O SQLite é o modernc.org/sqlite (Go puro, ver DECISIONS.md §1) → NÃO
#    precisa de CGO nem do Android NDK. -buildmode=pie é exigido pelo Android.
TARGET_SO="$JNILIBS_DIR/libton618.so"

echo "🤖 Compilando binário estático (CGO_ENABLED=0, buildmode=pie)..."
CGO_ENABLED=0 \
GOOS=android \
GOARCH="$GOARCH" \
go build \
    -trimpath \
    -buildmode=pie \
    -tags sqlite_fts5 \
    -ldflags="-s -w" \
    -o "$TARGET_SO" \
    ./cmd/server

if [ ! -f "$TARGET_SO" ]; then
    echo "❌ Erro ao gerar o binário."
    exit 1
fi

# 3. Assets web (recursivo, sem duplicatas que só inflam o APK)
#    - *.gz/*.br: o core já comprime em runtime (chi Compress) e o APK comprime
#      os assets de qualquer forma — manter os dois é desperdício puro.
#    - models/ort: 22 MB do onnxruntime-web, usado só pela busca semântica
#      (que exige baixar ~120 MB de modelo). Ver ANDROID_SEMANTIC.
echo "🌐 Sincronizando assets web (web/static → assets/web/static)..."
rm -rf "$STATIC_ASSETS"
mkdir -p "$STATIC_ASSETS"

cd "$CORE_DIR/web/static"
find . -type f \
    ! -name '*.gz' \
    ! -name '*.br' \
    ! -name '*.map' \
    ! -name '*.ts' \
    ! -name '*.test.cjs' \
    ! -path './models/ort/*' \
    -print0 | while IFS= read -r -d '' file; do
        mkdir -p "$STATIC_ASSETS/$(dirname "$file")"
        cp "$file" "$STATIC_ASSETS/$file"
    done

if [ "${ANDROID_SEMANTIC:-0}" = "1" ]; then
    if [ -d "$CORE_DIR/web/static/models/ort" ]; then
        echo "🧠 Incluindo onnxruntime-web (ANDROID_SEMANTIC=1)..."
        mkdir -p "$STATIC_ASSETS/models/ort"
        cp "$CORE_DIR/web/static/models/ort/"* "$STATIC_ASSETS/models/ort/"
    else
        echo "⚠️  ANDROID_SEMANTIC=1, mas web/static/models/ort não existe."
        echo "   Rode: cd core/web && node static/models/download-ort.js"
    fi
else
    echo "🧠 onnxruntime-web omitido (busca semântica indisponível no app)."
fi

# 4. Verificação: sem estes arquivos a UI abre mas não funciona
echo "🔎 Verificando assets essenciais..."
MISSING=0
for required in app.css js/app.js js/vendor/htmx.min.js js/vendor/alpine.min.js fonts/fonts.css; do
    if [ ! -f "$STATIC_ASSETS/$required" ]; then
        echo "   ❌ faltando: $required"
        MISSING=1
    fi
done
if [ "$MISSING" = "1" ]; then
    echo "❌ Assets incompletos — rode o build do frontend (cd core/web && node build.js) antes."
    exit 1
fi

echo ""
echo "✅ Build concluído"
echo "   binário : $TARGET_SO ($(du -h "$TARGET_SO" | cut -f1))"
echo "   assets  : $STATIC_ASSETS ($(du -sh "$STATIC_ASSETS" | cut -f1))"
echo ""
echo "👉 Agora gere o APK: ./gradlew assembleDebug (ou Android Studio)"
