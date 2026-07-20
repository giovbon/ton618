# Frontend TON-618

Os arquivos-fonte JS/JSX/CSS ficam em `src/` e são compilados para `static/` via esbuild.

## Build

```bash
npm run build
```

O que o build faz:
1. **Tailwind CSS**: compila `src/input.css` → `static/app.css` (minificado)
2. **esbuild (IIFE)**: `src/editor.js`, `src/spreadsheet.js`, `src/drawing.jsx`, `src/mindmap.js`, `src/map.js`, `src/semantic.js` → `static/*.js` (bundle + minify, IIFE para `<script>`)
3. **esbuild (ESM)**: `src/semantic-worker.js` → `static/semantic-worker.js` (bundle + minify, ESM para Web Worker `type: "module"`)
4. **Compressão**: Gera `.js.gz` (gzip) e `.js.br` (brotli) para cada `.js`/`.css`

## Ambientes

```bash
# Produção (comprime assets)
npm run build

# Desenvolvimento (pula compressão, limpa .gz/.br)
npm run build -- --dev
```

## Regras

- **Edite sempre em `src/`**, nunca em `static/`
- `static/` é diretório de saída — tudo lá é gerado pelo build
- O server Go faz file server de `static/` e serve `.br` com prioridade
- O Web Worker `semantic-worker.js` usa `type: "module"` — por isso o build separado com `format: "esm"`
