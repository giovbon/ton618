const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

// --- Mock Browser Environment ---
global.window = {};
global.localStorage = {
  store: {},
  getItem(key) { return this.store[key] || null; },
  setItem(key, value) { this.store[key] = value.toString(); },
  clear() { this.store = {}; }
};
global.FormData = class FormData {
  constructor() { this.data = new Map(); }
  append(k, v) { this.data.set(k, v); }
};
global.document = {
  getElementById: () => null,
  addEventListener: () => {},
  activeElement: null
};

let fetchCalls = [];
global.fetch = async (url, options) => {
  fetchCalls.push({ url, options });
  return { ok: true, status: 200, json: async () => ({ ok: true }) };
};

// crypto mock for generateHash
global.crypto = {
  subtle: {
    digest: async (algo, data) => {
      // Return a dummy ArrayBuffer of 32 bytes (SHA-256 size)
      return new Uint8Array(32).fill(42).buffer;
    }
  }
};
global.TextEncoder = class TextEncoder {
  encode(text) { return new Uint8Array(Buffer.from(text)); }
};

// Load the file content
const scriptPath = path.join(__dirname, '..', 'static', 'editor-common.js');
const scriptContent = fs.readFileSync(scriptPath, 'utf8');

// Evaluate the script in this context
eval(scriptContent);

const EditorCommon = global.window.EditorCommon;

test('EditorCommon is loaded', (t) => {
  assert.ok(EditorCommon);
  assert.strictEqual(typeof EditorCommon.generateHash, 'function');
});

test('getAuthHeaders handles tokens', (t) => {
  global.localStorage.clear();
  
  // No token
  assert.deepStrictEqual(EditorCommon.getAuthHeaders(), {});

  // With token without basic
  global.localStorage.setItem('ton_auth', 'mysecrettoken');
  assert.deepStrictEqual(EditorCommon.getAuthHeaders(), {
    'Authorization': 'Basic mysecrettoken'
  });

  // With token with basic
  global.localStorage.setItem('ton_auth', 'Basic othertoken');
  assert.deepStrictEqual(EditorCommon.getAuthHeaders(), {
    'Authorization': 'Basic othertoken'
  });
});

test('generateHash generates consistent length hash', async (t) => {
  const hash = await EditorCommon.generateHash('test data');
  assert.strictEqual(typeof hash, 'string');
  assert.strictEqual(hash.length, 64); // 32 bytes * 2 hex chars
});

test('httpSaveNote makes correct fetch call', async (t) => {
  fetchCalls = [];
  global.localStorage.setItem('ton_auth', 'test');
  
  await EditorCommon.httpSaveNote('notes/test.md', 'content', 'tag1,tag2', true);
  
  assert.strictEqual(fetchCalls.length, 1);
  assert.strictEqual(fetchCalls[0].url, '/api/note/save');
  assert.strictEqual(fetchCalls[0].options.method, 'POST');
  assert.strictEqual(fetchCalls[0].options.headers['Authorization'], 'Basic test');
  assert.ok(fetchCalls[0].options.body instanceof global.FormData);
  assert.strictEqual(fetchCalls[0].options.body.data.get('filename'), 'notes/test.md');
  assert.strictEqual(fetchCalls[0].options.body.data.get('silent'), 'true');
});

test('setStatus updates DOM element correctly', (t) => {
  const el = { textContent: '', className: '' };
  
  EditorCommon.setStatus(el, 'saved');
  assert.strictEqual(el.textContent, '\u2713');
  assert.ok(el.className.includes('emerald'));

  EditorCommon.setStatus(el, 'saving');
  assert.strictEqual(el.textContent, '\u27F3');
  assert.ok(el.className.includes('sky'));

  EditorCommon.setStatus(el, 'dirty');
  assert.strictEqual(el.textContent, '\u25CF');
  assert.ok(el.className.includes('amber'));
});
