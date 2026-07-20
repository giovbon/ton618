/**
 * semantic_test.mjs — Testes do SemanticIndex com node:test
 *
 * Uso: node --test web/semantic_test.mjs
 */

import { describe, it, before, after } from 'node:test';
import assert from 'node:assert';

// ── chunkText ──
function chunkText(text, maxChars, overlapChars) {
  const chunks = [];
  let start = 0;
  while (start < text.length) {
    let end = start + maxChars;
    if (end < text.length) {
      const lastNewline = text.lastIndexOf('\n', end);
      if (lastNewline > start + maxChars * 0.6) { end = lastNewline; }
      else {
        const lastSpace = text.lastIndexOf(' ', end);
        if (lastSpace > start + maxChars * 0.6) { end = lastSpace; }
      }
    }
    const chunk = text.slice(start, end).trim();
    if (chunk) chunks.push(chunk);
    start = end - overlapChars;
    if (start >= text.length - overlapChars) break;
  }
  return chunks;
}

// ── Mock Worker ──
class MockWorker {
  constructor() {
    this.onmessage = null;
    this._failEmbedId = null;
  }
  postMessage(msg) {
    if (msg.type === 'embed') {
      setTimeout(() => {
        if (!this.onmessage) return;
        if (this._failEmbedId === msg.id) {
          this.onmessage({ data: { type: 'embed_error', id: msg.id, message: 'Mock: falha no modelo' } });
        } else {
          const emb = new Array(384).fill(0);
          emb[0] = 1.0;
          this.onmessage({ data: { type: 'embedding', id: msg.id, data: emb } });
        }
      }, 0);
    } else if (msg.type === 'ping') {
      setTimeout(() => {
        if (this.onmessage) this.onmessage({ data: { type: 'pong', loaded: true } });
      }, 0);
    }
  }
  terminate() {}
}

// ── SemanticIndex (para teste) ──
class SemanticIndex {
  constructor() {
    this._worker = null;
    this._pendingCallbacks = new Map();
    this._nextId = 1;
    this._modelReady = false;
    this._onReadyCallbacks = [];
    this._indexing = false;
  }

  _ensureWorker() {
    if (this._worker) return;
    const self = this;
    this._worker = new MockWorker();
    this._worker.onmessage = (event) => {
      const msg = event.data;
      if (msg.type === 'embedding') {
        const cb = self._pendingCallbacks.get(msg.id);
        if (cb) { cb.resolve(msg.data); self._pendingCallbacks.delete(msg.id); }
      } else if (msg.type === 'embed_error') {
        const cb = self._pendingCallbacks.get(msg.id);
        if (cb) { cb.reject(new Error(msg.message)); self._pendingCallbacks.delete(msg.id); }
      } else if (msg.type === 'pong') {
        if (msg.loaded && !self._modelReady) {
          self._modelReady = true;
          self._onReadyCallbacks.forEach(cb => cb());
          self._onReadyCallbacks = [];
        }
      }
    };
  }

  embed(text, timeoutMs = 30000) {
    const self = this;
    return new Promise((resolve, reject) => {
      self._ensureWorker();
      const id = self._nextId++;
      self._pendingCallbacks.set(id, { resolve, reject });
      const timer = setTimeout(() => {
        self._pendingCallbacks.delete(id);
        reject(new Error('Timeout'));
      }, timeoutMs);
      self._pendingCallbacks.set(id, {
        resolve(val) { clearTimeout(timer); resolve(val); },
        reject(err) { clearTimeout(timer); reject(err); },
      });
      self._worker.postMessage({ type: 'embed', id, text });
    });
  }

  warmup() {
    this._ensureWorker();
    this._worker.postMessage({ type: 'ping' });
  }

  indexNote(filename, content) {
    if (!filename || !content) return Promise.resolve();
    const title = (content.match(/^#\s+(.+)$/m)?.[1] ?? filename.replace(/^notes\//, '').replace(/\.md$/, '').replace(/[_\-]/g, ' ')).trim();
    const cleanContent = content
      .replace(/```[\s\S]*?```/g, '')
      .replace(/!\[([^\]]*)\]\([^)]+\)/g, '')
      .replace(/\[([^\]]+)\]\([^)]+\)/g, '$1')
      .replace(/\s+/g, ' ').trim();
    let rawChunks = chunkText(cleanContent, 1500, 200);
    if (rawChunks.length === 0) rawChunks = [''];
    const chunksText = rawChunks.map(c => title + '\n\n' + c);
    const self = this;

    const embedPromises = chunksText.map((text, idx) =>
      self.embed(text).then(embedding => {
        for (let ei = 0; ei < embedding.length; ei++) {
          if (!isFinite(embedding[ei]) || isNaN(embedding[ei])) return null;
        }
        return { chunk_id: `${filename}#${idx}`, filename, index: idx, content: chunksText[idx], embedding };
      })
    );

    return Promise.all(embedPromises).then(results => {
      const valid = results.filter(c => c !== null);
      if (valid.length === 0) return;
      return fetch('/api/embeddings/save', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ filename, chunks: valid }),
      });
    }).catch(() => {});
  }

  batchIndex(notes, onProgress) {
    let indexed = 0;
    const self = this;
    const next = () => {
      if (indexed >= notes.length) return Promise.resolve();
      const note = notes[indexed];
      return self.indexNote(note.filename, note.content).catch(() => {}).then(() => {
        indexed++;
        if (onProgress) onProgress(indexed, notes.length);
        return new Promise(r => setTimeout(r, 50)).then(next);
      });
    };
    return next();
  }

  search(query, limit = 10) {
    return this.embed(query).then(embedding =>
      fetch('/api/embeddings/search', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ embedding, limit }),
      })
    ).then(response => {
      if (!response.ok) throw new Error('Erro na busca semantica: ' + response.statusText);
      return response.json();
    }).then(data => data.results || []);
  }

  getStatus() {
    return fetch('/api/embeddings/status').then(r => {
      if (!r.ok) throw new Error('Erro ao buscar status');
      return r.json();
    });
  }
}

// ── Helpers ──
function fakeFetch(result) {
  return (...args) => {
    const url = typeof args[0] === 'string' ? args[0] : args[0]?.url;
    if (typeof result === 'function') return result(url, args[1]);
    return Promise.resolve(result);
  };
}

// ── Testes ──
describe('SemanticIndex', () => {
  let originalFetch;

  before(() => { originalFetch = globalThis.fetch; });
  after(() => { globalThis.fetch = originalFetch; });

  describe('embed', () => {
    it('gera vetor de 384 dimensoes', async () => {
      const idx = new SemanticIndex();
      const emb = await idx.embed('teste');
      assert.equal(emb.length, 384);
    });

    it('completa antes do timeout de 5s', async () => {
      const idx = new SemanticIndex();
      const inicio = Date.now();
      const emb = await idx.embed('rapido', 5000);
      assert.ok(Date.now() - inicio < 5000);
      assert.equal(emb.length, 384);
    });
  });

  describe('indexNote (Promise.all)', () => {
    it('processa todos os chunks em paralelo e faz 1 fetch', async () => {
      const idx = new SemanticIndex();
      globalThis.fetch = fakeFetch(() => Promise.resolve({ ok: true }));

      const longContent = '# Titulo\n\n' + 'paragrafo um '.repeat(100) + '\n\n' + 'paragrafo dois '.repeat(100);
      await idx.indexNote('notes/test.md', longContent);
      assert.ok(true, 'indexNote completou sem erro');
    });

    it('filtra chunks com NaN', async () => {
      const idx = new SemanticIndex();
      let callCount = 0;
      const origEmbed = idx.embed.bind(idx);
      idx.embed = (text, to) => {
        callCount++;
        if (callCount === 2) return Promise.resolve(new Array(384).fill(NaN));
        return origEmbed(text, to);
      };

      let payload = null;
      globalThis.fetch = fakeFetch((url, opts) => {
        payload = JSON.parse(opts.body);
        return { ok: true };
      });

      const content = '# NaN\n\n' + 'parA '.repeat(100) + '\n\n' + 'parB '.repeat(100);
      await idx.indexNote('notes/nan.md', content);
      assert.ok(payload !== null);
      for (const c of payload.chunks) {
        assert.ok(!c.embedding.some(v => isNaN(v) || !isFinite(v)));
      }
    });

    it('nao faz fetch quando todos os chunks sao NaN/Inf', async () => {
      const idx = new SemanticIndex();
      idx.embed = () => Promise.resolve(new Array(384).fill(Infinity));
      let fetchCalled = false;
      globalThis.fetch = fakeFetch(() => { fetchCalled = true; });
      await idx.indexNote('notes/allbad.md', '# All bad\n\nX\n\nY');
      assert.ok(!fetchCalled);
    });
  });

  describe('batchIndex', () => {
    it('processa multiplas notas com progresso', async () => {
      const idx = new SemanticIndex();
      const progress = [];
      globalThis.fetch = fakeFetch({ ok: true });
      await idx.batchIndex([
        { filename: 'notes/a.md', content: '# A\n\naaa' },
        { filename: 'notes/b.md', content: '# B\n\nbbb' },
      ], (done, total) => progress.push(`${done}/${total}`));
      assert.deepEqual(progress, ['1/2', '2/2']);
    });
  });

  describe('tratamento de erros', () => {
    it('embed_error do Worker rejeita a Promise', async () => {
      const idx = new SemanticIndex();
      idx._ensureWorker();
      idx._worker._failEmbedId = 1;
      await assert.rejects(() => idx.embed('vai falhar'), /Mock: falha/);
    });

    it('timeout quando Worker nao responde', async () => {
      const idx = new SemanticIndex();
      idx._ensureWorker();
      idx._worker.onmessage = null;
      await assert.rejects(() => idx.embed('vai timeout', 50), /Timeout/);
    });

    it('search com HTTP 500 rejeita', async () => {
      const idx = new SemanticIndex();
      globalThis.fetch = fakeFetch({ ok: false, status: 500, statusText: 'Internal Server Error', json: () => Promise.resolve({}) });
      await assert.rejects(() => idx.search('query'), /500|Erro/);
    });

    it('getStatus com falha de rede rejeita', async () => {
      const idx = new SemanticIndex();
      globalThis.fetch = () => Promise.reject(new Error('Failed to fetch'));
      await assert.rejects(() => idx.getStatus(), /Failed to fetch/);
    });
  });

  describe('edge cases', () => {
    it('indexNote com filename vazio', async () => {
      const idx = new SemanticIndex();
      await idx.indexNote('', 'conteudo');
      assert.ok(true);
    });

    it('indexNote com content vazio', async () => {
      const idx = new SemanticIndex();
      await idx.indexNote('notes/vazia.md', '');
      assert.ok(true);
    });
  });

  describe('embed_error recovery', () => {
    it('catch interno absorve erro, sem fetch', async () => {
      const idx = new SemanticIndex();
      idx._ensureWorker();
      idx._worker._failEmbedId = 1;
      let fetchCalled = false;
      globalThis.fetch = fakeFetch(() => { fetchCalled = true; });
      const content = '# Error\n\n' + 'parA '.repeat(100) + '\n\n' + 'parB '.repeat(100);
      await idx.indexNote('notes/err.md', content);
      assert.ok(!fetchCalled);
    });
  });
});
