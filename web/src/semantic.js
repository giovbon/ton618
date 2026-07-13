/**
 * semantic.js — SOURCE FILE
 *
 * Este é o ARQUIVO FONTE. Edite AQUI, nunca em web/static/semantic.js.
 *
 * BUILD: `npm run build` (executa web/build.js com esbuild)
 *   → web/static/semantic.js     (minificado, IIFE para <script>)
 *   → web/static/semantic.js.gz  (gzip, servido preferencialmente)
 *   → web/static/semantic.js.br  (brotli)
 *
 * O server em cmd/server/main.go faz file server do diretório web/static/
 * e serve arquivos .br com Content-Encoding: br quando disponíveis.
 *
 * Módulo UNIFICADO de embeddings semânticos — usado por todas as páginas.
 *
 * Responsabilidades:
 *  1. Gerenciar o ciclo de vida do Web Worker (instância única)
 *  2. Indexação de notas (embed da nota inteira, truncada a 2000 chars)
 *  3. Indexação lazy: processa pendentes ao abrir a busca (via index.templ)
 *  4. Hook global window._semanticIndexNote para editor após save
 *
 * O worker semantic-worker.js é criado UMA única vez e compartilhado
 * entre todos os consumidores, evitando download duplicado do modelo (~120MB).
 *
 * @typedef {Object} ProgressMessage
 * @property {string} status  - "downloading" ou "loading"
 * @property {string} file    - Nome do arquivo sendo baixado
 * @property {number} loaded  - Bytes carregados
 * @property {number} total   - Total de bytes
 *
 * @typedef {Object} EmbeddingStatus
 * @property {number} total_notes   - Total de notas indexáveis
 * @property {number} indexed_notes - Notas já com embedding
 * @property {number} pending_notes - Notas sem embedding
 *
 * @typedef {Object} SearchResultItem
 * @property {string} filename - Caminho da nota (ex: "notes/foo.md")
 * @property {number} distance - Distância cosseno (menor = mais similar)
 */

/** @type {string} URL do Web Worker de embeddings */
var WORKER_URL = "/static/semantic-worker.js";
/** @type {string} Endpoint para salvar embedding */
var SAVE_ENDPOINT = "/api/embeddings/save";
/** @type {string} Endpoint para busca semântica */
var SEARCH_ENDPOINT = "/api/embeddings/search";
/** @type {string} Endpoint para status de indexação */
var STATUS_ENDPOINT = "/api/embeddings/status";
/** @type {string} Endpoint para notas pendentes de indexação */
var PENDING_ENDPOINT = "/api/embeddings/pending";

/**
 * Construtor do gerenciador de embeddings semânticos.
 * Singleton — use a instância global `semanticIndex`.
 *
 * @constructor
 */
function SemanticIndex() {
  /** @private @type {Worker|null} */
  this._worker = null;
  /** @private @type {Map<number, {resolve: Function, reject: Function}>} */
  this._pendingCallbacks = new Map();
  /** @private @type {number} */
  this._nextId = 1;
  /** @private @type {boolean} */
  this._modelReady = false;
  /** @private @type {Function[]} */
  this._onReadyCallbacks = [];
  /** @private @type {Function[]} */
  this._onProgressCallbacks = [];
  /** @private @type {Function[]} */
  this._onErrorCallbacks = [];
  /** @private @type {boolean} */
  this._indexing = false;
}

/** @private Inicializa o Web Worker (lazy — só cria na primeira chamada) */
SemanticIndex.prototype._ensureWorker = function() {
  if (this._worker) return;
  var self = this;
  this._worker = new Worker(WORKER_URL, { type: "module" });

  this._worker.onmessage = function(event) {
    var msg = event.data;
    switch (msg.type) {
      case "ready":
        self._modelReady = true;
        self._onReadyCallbacks.forEach(function(cb) { cb(); });
        self._onReadyCallbacks = [];
        break;
      case "progress":
        self._onProgressCallbacks.forEach(function(cb) { cb(msg); });
        break;
      case "embedding": {
        var cb = self._pendingCallbacks.get(msg.id);
        if (cb) { cb.resolve(msg.data); self._pendingCallbacks.delete(msg.id); }
        break;
      }
      case "embed_error": {
        var cb = self._pendingCallbacks.get(msg.id);
        if (cb) { cb.reject(new Error(msg.message)); self._pendingCallbacks.delete(msg.id); }
        break;
      }
      case "pong":
        if (msg.loaded && !self._modelReady) {
          self._modelReady = true;
          self._onReadyCallbacks.forEach(function(cb) { cb(); });
          self._onReadyCallbacks = [];
        }
        break;
      case "error":
        self._onErrorCallbacks.forEach(function(cb) { cb(msg.message); });
        break;
    }
  };

  this._worker.onerror = function(err) {
    console.error("[SemanticIndex] Worker error:", err);
    self._onErrorCallbacks.forEach(function(cb) { cb(err.message); });
  };
};

/**
 * Indica se o modelo já foi carregado e está pronto para uso.
 * @type {boolean}
 */
Object.defineProperty(SemanticIndex.prototype, "isReady", {
  get: function() { return this._modelReady; }
});

/**
 * Registra callback chamado quando o modelo estiver pronto (ou imediatamente se já estiver).
 * @param {Function} callback
 * @returns {Object}
 */
SemanticIndex.prototype.onReady = function(callback) {
  if (this._modelReady) { callback(); }
  else { this._onReadyCallbacks.push(callback); }
  return this;
};

/**
 * Registra callback para progresso de download do modelo.
 * @param {Function} callback
 * @returns {Object}
 */
SemanticIndex.prototype.onProgress = function(callback) {
  this._onProgressCallbacks.push(callback);
  return this;
};

/**
 * Registra callback para erros do worker/modelo.
 * @param {Function} callback
 * @returns {Object}
 */
SemanticIndex.prototype.onError = function(callback) {
  this._onErrorCallbacks.push(callback);
  return this;
};

/**
 * Pré-carrega o modelo (warm-up) sem gerar embedding.
 */
SemanticIndex.prototype.warmup = function() {
  this._ensureWorker();
  this._worker.postMessage({ type: "ping" });
};

/**
 * Gera embedding para um texto via Web Worker.
 * @param {string} text - Texto a ser vetorizado
 * @param {number} [timeoutMs=30000] - Timeout em ms por chunk
 * @returns {Promise<number[]>} Vetor float32 com 384 dimensões
 */
SemanticIndex.prototype.embed = function(text, timeoutMs) {
  timeoutMs = timeoutMs || 30000;
  var self = this;
  return new Promise(function(resolve, reject) {
    self._ensureWorker();
    var id = self._nextId++;
    self._pendingCallbacks.set(id, { resolve: resolve, reject: reject });

    var timer = setTimeout(function() {
      self._pendingCallbacks.delete(id);
      reject(new Error("Timeout ao gerar embedding (30s)"));
    }, timeoutMs);

    // Wrap resolve/reject to clear the timeout
    var originalResolve = resolve;
    var originalReject = reject;
    self._pendingCallbacks.set(id, {
      resolve: function(val) {
        clearTimeout(timer);
        originalResolve(val);
      },
      reject: function(err) {
        clearTimeout(timer);
        originalReject(err);
      }
    });

    self._worker.postMessage({ type: "embed", id: id, text: text });
  });
};

/**
 * Divide um texto em múltiplos chunks baseando-se em limites de parágrafo ou espaço.
 * @param {string} text - O texto a ser dividido
 * @param {number} maxChars - Máximo de caracteres por chunk
 * @param {number} overlapChars - Caracteres de sobreposição entre chunks
 * @returns {string[]} Array de chunks
 */
function chunkText(text, maxChars, overlapChars) {
  var chunks = [];
  var start = 0;
  while (start < text.length) {
    var end = start + maxChars;
    if (end < text.length) {
      // Tenta quebrar no limite de um parágrafo ou frase
      var lastNewline = text.lastIndexOf("\n", end);
      if (lastNewline > start + maxChars * 0.6) {
        end = lastNewline;
      } else {
        var lastSpace = text.lastIndexOf(" ", end);
        if (lastSpace > start + maxChars * 0.6) {
          end = lastSpace;
        }
      }
    }
    var chunk = text.slice(start, end).trim();
    if (chunk) chunks.push(chunk);

    start = end - overlapChars;
    if (start >= text.length - overlapChars) break;
  }
  return chunks;
}

/**
 * Gera embedding de uma nota com Semantic Chunking.
 * Limpa o markdown, divide o conteúdo em pedaços e gera vetores iterativamente.
 * O título é extraído e fixado no início de cada pedaço para manter o contexto.
 *
 * @param {string} filename - Caminho da nota (ex: "notes/foo.md")
 * @param {string} content  - Conteúdo textual da nota
 * @returns {Promise<void>}
 */
SemanticIndex.prototype.indexNote = function(filename, content) {
  if (!filename || !content) return Promise.resolve();

  // 1. Extrai o título: primeiro heading # do conteúdo, ou nome do arquivo
  var title = "";
  var headingMatch = content.match(/^#\s+(.+)$/m);
  if (headingMatch) {
    title = headingMatch[1].trim();
  } else {
    title = filename.replace(/^notes\//, "").replace(/\.md$/, "").replace(/[_\-]/g, " ");
  }

  // 2. Limpeza básica de Markdown para economizar tokens
  var cleanContent = content
    .replace(/```[\s\S]*?```/g, "") // remove blocos de código
    .replace(/!\[([^\]]*)\]\([^)]+\)/g, "") // remove imagens
    .replace(/\[([^\]]+)\]\([^)]+\)/g, "$1") // mantém só texto de links
    .replace(/\s+/g, " ") // colapsa múltiplos espaços
    .trim();

  // 3. Divide em chunks de ~1500 caracteres
  var rawChunks = chunkText(cleanContent, 1500, 200);
  if (rawChunks.length === 0) rawChunks = [""];

  // Concatena o título em cada parte para manter a relevância global da nota
  var chunksText = rawChunks.map(function(c) {
    return title + "\n\n" + c;
  });

  var self = this;

  // 4. Gera embeddings SEQUENCIALMENTE (evita travar o Worker com chamadas concorrentes)
  function embedNext(idx) {
    if (idx >= chunksText.length) {
      return Promise.resolve([]);
    }
    return self.embed(chunksText[idx]).then(function(embedding) {
      // Valida NaN/Inf antes de incluir no resultado
      for (var ei = 0; ei < embedding.length; ei++) {
        if (!isFinite(embedding[ei]) || isNaN(embedding[ei])) {
          console.warn("[SemanticIndex] NaN/Inf no chunk", idx, "- pulando");
          return null;
        }
      }
      return {
        chunk_id: filename + "#" + idx,
        filename: filename,
        index: idx,
        content: chunksText[idx],
        embedding: embedding
      };
    }).then(function(chunk) {
      // Processa próximo chunk e acumula resultados
      return embedNext(idx + 1).then(function(rest) {
        var result = chunk ? [chunk] : [];
        return result.concat(rest);
      });
    });
  }

  return embedNext(0).then(function(chunks) {
    if (chunks.length === 0) return Promise.resolve();

    var payload = {
      filename: filename,
      chunks: chunks
    };
    return fetch(SAVE_ENDPOINT, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    }).then(function(r) {
      if (!r.ok) console.debug("[SemanticIndex] indexNote HTTP error:", r.status);
    });
  }).catch(function(err) {
    console.debug("[SemanticIndex] indexNote error:", err);
    // NÃO rejeita — sempre resolve para o caller continuar o progresso
  });
};

/**
 * Busca semântica: gera embedding da query e consulta o backend.
 *
 * @param {string} query - Texto da busca
 * @param {number} [limit=10] - Máximo de resultados
 * @returns {Promise<SearchResultItem[]>}
 */
SemanticIndex.prototype.search = function(query, limit) {
  limit = limit || 10;
  return this.embed(query).then(function(embedding) {
    return fetch(SEARCH_ENDPOINT, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ embedding: embedding, limit: limit }),
    });
  }).then(function(response) {
    if (!response.ok) throw new Error("Erro na busca semântica: " + response.statusText);
    return response.json();
  }).then(function(data) {
    return data.results || [];
  });
};

/**
 * Busca o status de indexação semântica no backend.
 * @returns {Promise<EmbeddingStatus>}
 */
SemanticIndex.prototype.getStatus = function() {
  return fetch(STATUS_ENDPOINT).then(function(response) {
    if (!response.ok) throw new Error("Erro ao buscar status");
    return response.json();
  });
};

/**
 * Indexa múltiplas notas em lote (com chunking).
 * @param {Array<{filename: string, content: string}>} notes - Lista de notas
 * @param {Function} [onProgress] - Callback(indexed, total)
 * @returns {Promise<void>}
 */
SemanticIndex.prototype.batchIndex = function(notes, onProgress) {
  var indexed = 0;
  var self = this;
  function next() {
    if (indexed >= notes.length) return Promise.resolve();
    var note = notes[indexed];
    return self.indexNote(note.filename, note.content).catch(function() {}).then(function() {
      indexed++;
      if (onProgress) onProgress(indexed, notes.length);
      return new Promise(function(r) { setTimeout(r, 50); }).then(next);
    });
  }
  return next();
};

/**
 * Indexação lazy: processa notas pendentes imediatamente (chamado ao abrir busca semântica).
 * Aguarda o modelo carregar antes de começar.
 * Retorna Promise que resolve quando todas as pendentes forem indexadas.
 *
 * @param {Function} [onProgressFn] - Callback(indexed, total)
 * @returns {Promise<void>}
 */
SemanticIndex.prototype.indexPending = function(onProgressFn) {
  if (onProgressFn) {
    this._activeProgressFn = onProgressFn;
  }

  if (this._indexing) {
    return this._indexingPromise || Promise.resolve();
  }
  this._indexing = true;
  var self = this;

  // Garante que o worker existe
  this._ensureWorker();

  // Aguarda o modelo ficar pronto antes de processar (timeout 60s)
  function waitForModel() {
    return /** @type {Promise<void>} */ (new Promise(function(resolve, reject) {
      if (self._modelReady) { resolve(); return; }

      var timeout = setTimeout(function() {
        reject(new Error("Modelo não carregou em 60s"));
      }, 60000);

      self._onReadyCallbacks.push(function() {
        clearTimeout(timeout);
        resolve();
      });

      // Inicia warm-up se necessário
      self._worker.postMessage({ type: "ping" });
    }));
  }

  this._indexingPromise = waitForModel().then(function() {
    return self.getStatus();
  }).then(function(status) {
    var total = status.pending_notes + status.stale_notes;
    var indexed = 0;
    var attempted = new Set(); // <-- Fix: Prevent infinite loop of failing notes

    if (total === 0) { return; }

    function processBatch() {
      return fetch(PENDING_ENDPOINT + "?limit=10")
        .then(function(r) { return r.json(); })
        .then(function(notes) {
          if (!notes || notes.length === 0) { return; }

          // Remove notes already attempted in this indexPending run
          var newNotes = notes.filter(function(n) { return !attempted.has(n.filename); });
          if (newNotes.length === 0) { return; } // Avoid looping on same failing notes

          var i = 0;
          function nextNote() {
            if (i >= newNotes.length) {
              return new Promise(function(r) { setTimeout(r, 200); }).then(processBatch);
            }
            var note = newNotes[i];
            attempted.add(note.filename);

            return self.indexNote(note.filename, note.content)
              .then(function() {
                indexed++;
                if (self._activeProgressFn) self._activeProgressFn(indexed, total);
                i++;
                return new Promise(function(r) { setTimeout(r, 50); }).then(nextNote);
              });
          }
          return nextNote();
        });
    }

    return processBatch();
  }).catch(function(err) {
    console.debug("[SemanticIndex] indexPending error:", err);
  }).then(function() {
    self._indexing = false;
    self._indexingPromise = null;
    self._activeProgressFn = null;
  });

  return this._indexingPromise;
};

/**
 * Reindexação manual (via botão "Indexar tudo" na UI).
 * Usa o mesmo fluxo do indexPending, mas com callbacks de progresso.
 *
 * @param {Function} onProgressFn - Callback(indexed, total)
 * @param {Function} [onDoneFn] - Callback ao terminar (err se falhou)
 */
SemanticIndex.prototype.reindexAll = function(onProgressFn, onDoneFn) {
  var self = this;
  this.indexPending(onProgressFn).then(function() {
    if (onDoneFn) onDoneFn();
  }).catch(function(err) {
    if (onDoneFn) onDoneFn(err);
  });
};

// ── Singleton exposto no window para acesso global ──
window.semanticIndex = new SemanticIndex();

// ── Bridge: hook global para o editor (usa indexNote com chunking) ──
window._semanticIndexNote = function(filename, content) {
  if (!filename || !content) return;
  window.semanticIndex.indexNote(filename, content).catch(function() {});
};

// ── Mobile detection ──
(function() {
  var isMobile = /Android|iPhone|iPad|iPod|Mobile|webOS/i.test(navigator.userAgent)
    || (navigator.maxTouchPoints > 1 && window.innerWidth < 1024);
  window._semanticDesktopOnly = !isMobile;
})();
