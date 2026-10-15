/**
 * ort-wasm.js — descobre de onde o ONNX Runtime (ORT) carrega o runtime WebAssembly.
 *
 * O ORT roda dentro do Web Worker de embeddings (`semantic-worker.js`) e precisa de
 * dois arquivos: `ort-wasm-simd-threaded.asyncify.mjs` e o `.wasm` correspondente.
 * Eles são procurados a partir de um caminho base (`env.backends.onnx.wasm.wasmPaths`).
 *
 * Dois caminhos base são possíveis:
 *
 *  1. **LOCAL** (`/static/models/ort/`) — servido pelo próprio core. É o caminho do
 *     desktop/Docker e do APK compilado com `ANDROID_SEMANTIC=1`: funciona offline.
 *  2. **CDN** (padrão do Transformers.js: `cdn.jsdelivr.net/npm/onnxruntime-web@<versão>/dist/`)
 *     — usado quando o servidor não tem os arquivos. É o caso do **APK padrão**, que
 *     omite `web/static/models/ort/` para não somar 22 MB (android/build-core-arm64.sh).
 *     Mesmo espírito do fallback já existente para o modelo de embeddings
 *     (`allowRemoteModels`): o CDN é o plano B quando o local não está disponível.
 *
 * ⚠️ **Por que a sonda é obrigatória (bug real, 15/10/2026):** o ORT não cai para o CDN
 * se o caminho base estiver fixado e não existir — ele aborta a criação do backend com
 * `no available backend found. ERR: [wasm] TypeError: error loading dynamically imported
 * module: .../static/models/ort/ort-wasm-simd-threaded.asyncify.mjs`. No Android (assets
 * sem `models/ort/`) isso deixava a busca semântica 100% indisponível, em silêncio.
 * Por isso o caminho local SÓ é fixado depois de confirmado por um HEAD.
 */

/** Caminho base servido pelo core quando os arquivos do ORT estão presentes. */
export const ORT_LOCAL_PATH = "/static/models/ort/";

/** Arquivo usado como sonda: é exatamente o `.mjs` que o ORT importa dinamicamente. */
export const ORT_LOCAL_PROBE = ORT_LOCAL_PATH + "ort-wasm-simd-threaded.asyncify.mjs";

/**
 * Resolve o `wasmPaths` do ORT.
 *
 * @param {typeof fetch} [fetchImpl] - `fetch` a usar na sonda (injetável para teste).
 * @returns {Promise<string|null>} `ORT_LOCAL_PATH` se os arquivos locais existirem;
 *   `null` para deixar o padrão do Transformers.js (CDN).
 */
export async function resolveOrtWasmPath(fetchImpl) {
  const doFetch = fetchImpl || (typeof fetch === "function" ? fetch : null);
  if (!doFetch) return null;

  try {
    const res = await doFetch(ORT_LOCAL_PROBE, { method: "HEAD" });
    if (res && res.ok) return ORT_LOCAL_PATH;
  } catch (err) {
    // Servidor sem a rota (404), offline ou sonda bloqueada: sem arquivos locais.
    return null;
  }
  return null;
}
