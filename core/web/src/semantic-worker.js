/**
 * semantic-worker.js — SOURCE FILE
 *
 * Este é o ARQUIVO FONTE. Edite AQUI, nunca em web/static/semantic-worker.js.
 *
 * BUILD: `npm run build` (executa web/build.js com esbuild)
 *   → web/static/semantic-worker.js     (minificado, ESM para Worker)
 *   → web/static/semantic-worker.js.gz  (gzip)
 *   → web/static/semantic-worker.js.br  (brotli)
 *
 * Web Worker que executa inferência de embeddings com Transformers.js.
 * Roda em thread separada para não bloquear a UI.
 *
 * Modelo: Xenova/paraphrase-multilingual-MiniLM-L12-v2
 * - Suporte nativo a português e múltiplos idiomas
 * - Dimensão de saída: 384
 * - Tamanho quantizado (q8): ~120MB (cacheado pelo browser após 1ª carga)
 *
 * Protocolo de mensagens (postMessage):
 *
 * ── Entrada (main thread → worker) ──
 * @typedef {Object} WorkerInput
 * @property {"embed"|"ping"|"status"} type
 * @property {number} [id]       - ID para correlacionar resposta (embed)
 * @property {string} [text]     - Texto a ser vetorizado (embed)
 *
 * ── Saída (worker → main thread) ──
 * @typedef {Object} WorkerOutput
 * @property {"ready"|"progress"|"embedding"|"embed_error"|"pong"|"status_response"|"error"|"unknown_command"} type
 * @property {number} [id]       - ID da requisição original
 * @property {number[]} [data]   - Vetor float32 com 384 dimensões (embedding)
 * @property {string} [message]  - Mensagem de erro (embed_error, error)
 * @property {string} [status]   - Status do download (progress)
 * @property {string} [file]     - Arquivo sendo baixado (progress)
 * @property {number} [loaded]   - Bytes carregados (progress)
 * @property {number} [total]    - Total de bytes (progress)
 * @property {boolean} [error]   - Se houve erro (pong, status_response)
 */

import { pipeline, env } from "@huggingface/transformers";

// Configuração do Transformers.js
// O modelo está disponível localmente em /static/models/ (servido pelo servidor Go).
// O worker carrega de lá, sem depender de CDN externo. Se o cache local falhar,
// tenta como fallback o HuggingFace CDN + IndexedDB.
env.allowLocalModels = true;
env.localModelPath = "/static/models/";
env.allowRemoteModels = true; // fallback: CDN do HuggingFace se local falhar
// O CacheStorage API (self.caches) só é disponível em contextos seguros (HTTPS ou localhost).
// Em contextos HTTP não seguros (ex: acessando via IP http://192.168.15.6:6180), self.caches é undefined.
env.useBrowserCache = typeof self !== "undefined" && typeof self.caches !== "undefined";
env.backends.onnx.wasm.wasmPaths = "/static/models/ort/";

/** @type {string} Nome do modelo HuggingFace para embeddings multilingues */
const MODEL_NAME = "Xenova/paraphrase-multilingual-MiniLM-L12-v2";

/**
 * Device de execução: lido da query string (?device=wasm ou ?device=auto).
 * Padrão: "wasm" (CPU) — mais compatível e econômico em RAM.
 * "auto" tenta WebGPU (GPU) primeiro, cai para WASM se não disponível.
 * @type {string}
 */
const DEVICE = new URLSearchParams(self.location.search).get("device") || "wasm";

/** @type {Promise<any>|null} Promise da pipeline — lazy init, cacheado após primeira carga */
let pipelinePromise = null;

/** @type {boolean} Se o modelo já foi carregado com sucesso */
let modelLoaded = false;

/** @type {string|null} Mensagem de erro se o carregamento falhou */
let loadError = null;

/** @type {Array<{id: number, text: string}>} Fila de embeddings serializada */
let embedQueue = [];
/** @type {boolean} Se já está processando a fila */
let processingQueue = false;

/**
 * Processa a fila de embeddings sequencialmente — um por vez.
 * ONNX não lida bem com chamadas concorrentes no mesmo pipeline.
 */
async function processEmbedQueue() {
  if (processingQueue) return;
  processingQueue = true;
  while (embedQueue.length > 0) {
    const { id, text } = embedQueue.shift();
    try {
      const embedding = await embed(text);
      self.postMessage({ type: "embedding", id, data: embedding });
    } catch (err) {
      self.postMessage({ type: "embed_error", id, message: err.message });
    }
  }
  processingQueue = false;
}

/**
 * Obtém (ou inicializa) a pipeline de feature-extraction do Transformers.js.
 * Lazy init: só carrega o modelo na primeira chamada.
 * Emite "progress" durante download, "ready" ao concluir, "error" se falhar.
 *
 * @returns {Promise<any>} A pipeline carregada ou null se falhou
 */
async function getModel() {
  if (!pipelinePromise) {
    pipelinePromise = pipeline("feature-extraction", MODEL_NAME, {
      dtype: "q8",
      device: DEVICE,
      progress_callback: (progress) => {
        var p = /** @type {any} */ (progress);
        if (p.status === "downloading" || p.status === "loading") {
          self.postMessage({
            type: "progress",
            status: p.status,
            file: p.file || "",
            loaded: p.loaded || 0,
            total: p.total || 0,
          });
        }
      },
    });

    try {
      await pipelinePromise;
      modelLoaded = true;
      self.postMessage({ type: "ready" });
    } catch (err) {
      loadError = err.message;
      pipelinePromise = null;
      self.postMessage({ type: "error", message: err.message });
    }
  }
  return pipelinePromise;
}

/**
 * Gera embedding para um texto usando o modelo já carregado.
 * Aplica mean pooling e normalização L2.
 *
 * @param {string} text - Texto de entrada (truncado a ~2000 chars pelo caller)
 * @returns {Promise<number[]>} Vetor float32 com 384 dimensões
 * @throws {Error} Se o modelo não estiver disponível
 */
async function embed(text) {
  const pipe = await getModel();
  if (!pipe) throw new Error("modelo não disponível");
  const output = await pipe(text, { pooling: "mean", normalize: true });
  return Array.from(output.data);
}

/**
 * Handler de mensagens recebidas da thread principal.
 *
 * Comandos aceitos:
 * - "embed" {id, text}   → Gera embedding e responde com "embedding" ou "embed_error"
 * - "ping"               → Verifica se modelo está carregado, responde "pong"
 * - "status"             → Retorna estado atual do modelo
 *
 * @param {MessageEvent<WorkerInput>} event
 */
self.onmessage = async (event) => {
  const { type, id, text } = event.data;

  switch (type) {
    case "config":
      // Atualiza configuração em tempo real (ex: device trocado nas settings)
      // A pipeline já criada mantém o device original; o novo valor vale
      // após recriação (recarregar a página ou dispose + nova abertura)
      console.log("[SemanticWorker] Config received:", event.data);
      break;

    case "embed":
      embedQueue.push({ id, text });
      processEmbedQueue();
      break;

    case "ping":
      try {
        await getModel();
        self.postMessage({ type: "pong", loaded: modelLoaded });
      } catch (err) {
        self.postMessage({ type: "pong", loaded: false, error: err.message });
      }
      break;

    case "status":
      self.postMessage({
        type: "status_response",
        loaded: modelLoaded,
        error: loadError,
      });
      break;

    default:
      self.postMessage({ type: "unknown_command", received: type });
  }
};
