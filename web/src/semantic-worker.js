/**
 * semantic-worker.js
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

// Configura para ler os arquivos de modelo localmente do servidor ton618plus
env.allowLocalModels = true;
env.allowRemoteModels = false; // impede qualquer download externo para o Hugging Face
env.localModelPath = "/static/models/";
env.useBrowserCache = true; // continua cacheando no IndexedDB para carregamentos instantâneos seguintes
env.backends.onnx.wasm.wasmPaths = "/static/models/ort/";

/** @type {string} Nome do modelo HuggingFace para embeddings multilingues */
const MODEL_NAME = "Xenova/paraphrase-multilingual-MiniLM-L12-v2";

/** @type {Promise<any>|null} Promise da pipeline — lazy init, cacheado após primeira carga */
let pipelinePromise = null;

/** @type {boolean} Se o modelo já foi carregado com sucesso */
let modelLoaded = false;

/** @type {string|null} Mensagem de erro se o carregamento falhou */
let loadError = null;

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
      device: "auto",
      progress_callback: (progress) => {
        if (progress.status === "downloading" || progress.status === "loading") {
          self.postMessage({
            type: "progress",
            status: progress.status,
            file: progress.file || "",
            loaded: progress.loaded || 0,
            total: progress.total || 0,
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
    case "embed":
      try {
        const embedding = await embed(text);
        self.postMessage({ type: "embedding", id, data: embedding });
      } catch (err) {
        self.postMessage({ type: "embed_error", id, message: err.message });
      }
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
