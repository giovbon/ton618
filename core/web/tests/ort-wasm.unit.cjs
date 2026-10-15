/**
 * ort-wasm.unit.cjs — Guarda do fallback do runtime ONNX Runtime (ORT).
 *
 * Uso: node --test tests/ort-wasm.unit.cjs
 *
 * Contexto (bug real, 15/10/2026): o Web Worker de embeddings fixava
 * `env.backends.onnx.wasm.wasmPaths = "/static/models/ort/"`. O APK padrão do
 * Android **não** embarca essa pasta (22 MB omitidos por padrão em
 * android/build-core-arm64.sh) e o ORT, com um caminho base inexistente, **não**
 * cai para o CDN — aborta com:
 *
 *   Erro: no available backend found. ERR: [wasm] TypeError: error loading
 *   dynamically imported module: .../static/models/ort/ort-wasm-simd-threaded.asyncify.mjs
 *
 * Resultado: busca semântica indisponível no celular, sem aviso claro.
 *
 * O contrato agora tem duas pontas que precisam andar juntas:
 *   1. o worker resolve o caminho com uma SONDA (`resolveOrtWasmPath`);
 *   2. a CSP libera `cdn.jsdelivr.net` (em script-src E connect-src) para o plano B.
 *      → a ponta Go é verificada em internal/middleware/security_headers_test.go.
 */
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { pathToFileURL } = require('url');

// ── Config ──
const ROOT = path.resolve(__dirname, '..');
const SRC_DIR = path.join(ROOT, 'src');
const WORKER = path.join(SRC_DIR, 'semantic-worker.js');
const DOWNLOAD_ORT = path.join(ROOT, 'static', 'models', 'download-ort.js');

function ler(p) {
  return fs.readFileSync(p, 'utf8');
}

function carregarModulo() {
  // O fonte é ESM ("type": "module"), este teste é CJS.
  return import(pathToFileURL(path.join(SRC_DIR, 'ort-wasm.js')).href);
}

// ── resolveOrtWasmPath ──

test('usa o caminho local quando os arquivos existem (desktop/Docker)', async () => {
  const { resolveOrtWasmPath, ORT_LOCAL_PATH, ORT_LOCAL_PROBE } = await carregarModulo();

  const chamadas = [];
  const fetchStub = async (url, opts) => {
    chamadas.push({ url, opts });
    return { ok: true, status: 200 };
  };

  const resolved = await resolveOrtWasmPath(fetchStub);

  assert.strictEqual(resolved, ORT_LOCAL_PATH, 'com o arquivo presente o ORT deve carregar do próprio servidor');
  assert.strictEqual(chamadas.length, 1, 'a sonda deve ser uma única requisição');
  assert.strictEqual(chamadas[0].url, ORT_LOCAL_PROBE, 'a sonda deve apontar para o .mjs que o ORT importa');
  assert.strictEqual(chamadas[0].opts.method, 'HEAD', 'a sonda não precisa baixar o arquivo (HEAD)');
});

test('404 (APK sem models/ort) devolve null — o ORT usa o runtime do CDN', async () => {
  const { resolveOrtWasmPath } = await carregarModulo();

  const resolved = await resolveOrtWasmPath(async () => ({ ok: false, status: 404 }));

  assert.strictEqual(resolved, null, 'sem arquivos locais o wasmPaths precisa ficar no padrão (CDN)');
});

test('falha de rede/offline devolve null em vez de derrubar o worker', async () => {
  const { resolveOrtWasmPath } = await carregarModulo();

  const resolved = await resolveOrtWasmPath(async () => {
    throw new Error('Failed to fetch');
  });

  assert.strictEqual(resolved, null);
});

// ── Contrato entre os arquivos ──

test('semantic-worker.js não fixa mais o caminho do ORT às cegas', () => {
  const worker = ler(WORKER);

  assert.doesNotMatch(
    worker,
    /env\.backends\.onnx\.wasm\.wasmPaths\s*=\s*["']\//,
    'fixar um caminho local literal volta a quebrar o Android (assets sem models/ort)'
  );
  assert.match(worker, /configureOrtWasmPaths/, 'o worker precisa resolver o caminho do ORT antes de criar a pipeline');
  assert.match(worker, /from "\.\/ort-wasm\.js"/, 'o worker deve usar o resolvedor compartilhado (testável)');
});

test('a sonda aponta para um arquivo que o projeto realmente copia', async () => {
  const { ORT_LOCAL_PROBE } = await carregarModulo();

  const sonda = path.basename(ORT_LOCAL_PROBE);
  assert.ok(
    ler(DOWNLOAD_ORT).includes(sonda),
    `a sonda "${sonda}" precisa estar na lista ALLOWED de download-ort.js (trocar a variante do ORT exige trocar a sonda)`
  );
});
