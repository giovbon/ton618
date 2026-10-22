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
global.EditorCommon = global.window.EditorCommon;
require(path.join(__dirname, '..', 'src', 'database.js'));

const actionsFormatter = global.window.DatabaseUtils.actionsFormatter;

test('actionsFormatter devolve o botão nativo de excluir registro', () => {
  const html = actionsFormatter();

  assert.ok(html.includes('type="button"'));
  assert.ok(html.includes('class="db-del-btn"'));
  assert.ok(html.includes('title="Excluir registro"'));
  assert.ok(html.includes('&times;'));
});
