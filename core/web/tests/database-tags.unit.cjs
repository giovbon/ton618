const test = require('node:test');
const assert = require('node:assert');
const path = require('path');

// database.js é um IIFE de browser. Para testar só o formatter puro:
// - window/document mínimos;
// - os assets (CSS/Tabulator) nunca "carregam", então o bootstrap não roda;
// - editor-common.js fornece o escapeHtml real (SSOT).
global.window = {};
global.document = {
  querySelector: () => null,
  createElement: () => ({}),
  head: { appendChild: () => {} },
  getElementById: () => null,
  addEventListener: () => {},
};

require(path.join(__dirname, '..', 'static', 'editor-common.js'));
// No browser window.EditorCommon também é global; no Node, expõe explicitamente.
global.EditorCommon = global.window.EditorCommon;
require(path.join(__dirname, '..', 'src', 'database.js'));

const formatTags = global.window.DatabaseUtils.tagsFormatter;

test('tagsFormatter escapa HTML e mantém o prefixo #', () => {
  const html = formatTags({
    getValue: () => 'segura,<img src=x onerror=alert(1)>',
  });

  assert.ok(html.includes('class="tag-pill"'));
  assert.ok(html.includes('#segura'));
  assert.ok(html.includes('&lt;img'));
  assert.ok(!html.includes('<img'), 'tag não pode injetar elemento HTML');
});

test('tagsFormatter escapa &, aspas e fecha o span corretamente', () => {
  const html = formatTags({ getValue: () => 'a&b,"aspas"' });

  assert.ok(html.includes('&amp;'));
  assert.ok(html.includes('&quot;'));
  assert.ok(html.endsWith('</span>'));
});

test('tagsFormatter ignora tags técnicas e vazias', () => {
  assert.strictEqual(formatTags({ getValue: () => '' }), '');
  assert.strictEqual(formatTags({ getValue: () => 'drawing,mindmap,markmap' }), '');
});
