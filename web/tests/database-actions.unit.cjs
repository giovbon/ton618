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

const { actionsFormatter, isExtractableRow } = global.window.DatabaseUtils;

// Célula mínima no formato que o Tabulator entrega ao formatter.
function cellWith(data) {
  return { getRow: () => ({ getData: () => data }) };
}

test('actionsFormatter sem célula devolve só o botão nativo de excluir registro', () => {
  const html = actionsFormatter();

  assert.ok(html.includes('type="button"'));
  assert.ok(html.includes('class="db-del-btn"'));
  assert.ok(html.includes('title="Excluir registro"'));
  assert.ok(html.includes('&times;'));
  assert.ok(!html.includes('db-extract-btn'));
});

test('actionsFormatter em nota comum não mostra extração', () => {
  const html = actionsFormatter(cellWith({ arquivo: 'notes/ideia.md', type: 'nota' }));

  assert.ok(html.includes('db-del-btn'));
  assert.ok(!html.includes('db-extract-btn'));
});

test('actionsFormatter em PDF mostra extrair + excluir', () => {
  const html = actionsFormatter(cellWith({ arquivo: 'pdfs/manual.pdf', type: 'pdf' }));

  assert.ok(html.includes('class="db-extract-btn"'));
  assert.ok(html.includes('title="Extrair texto para uma nota"'));
  assert.ok(html.includes('db-del-btn'));
});

test('actionsFormatter em EPUB mostra extrair + excluir', () => {
  const html = actionsFormatter(cellWith({ arquivo: 'epubs/livro.epub', type: 'epub' }));

  assert.ok(html.includes('db-extract-btn'));
  assert.ok(html.includes('db-del-btn'));
});

test('isExtractableRow cai na extensão quando o tipo não veio', () => {
  assert.equal(isExtractableRow({ arquivo: 'pdfs/x.PDF' }), true);
  assert.equal(isExtractableRow({ arquivo: 'epubs/y.epub' }), true);
  assert.equal(isExtractableRow({ arquivo: 'attachments/z.zip' }), false);
  assert.equal(isExtractableRow(null), false);
});
