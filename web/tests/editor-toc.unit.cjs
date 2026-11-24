const test = require('node:test');
const assert = require('node:assert');
const path = require('path');
const { pathToFileURL } = require('url');

// editor-toc.js é ES module (package.json type: module) — importado dinamicamente.
const modulePath = pathToFileURL(path.join(__dirname, '..', 'src', 'editor-toc.js')).href;

// Mock mínimo de um editor TipTap: só o que collectHeadings/jumpToTocLine usam.
function makeEditor(headings) {
  const calls = [];
  const doc = {
    descendants(cb) {
      for (const h of headings) {
        cb({ type: { name: 'heading' }, textContent: h.text, attrs: { level: h.level } }, h.pos);
      }
    }
  };
  const chain = {
    focus() { calls.push('focus'); return chain; },
    setTextSelection(pos) { calls.push(['selection', pos]); return chain; },
    scrollIntoView() { calls.push('scrollIntoView'); return chain; },
    run() { calls.push('run'); return true; }
  };
  return { editor: { isDestroyed: false, state: { doc }, chain: () => chain }, calls };
}

test('tocLineIndex conta as quebras de linha antes do cursor', async () => {
  const { tocLineIndex } = await import(modulePath);
  assert.strictEqual(tocLineIndex('a\nb\nc', 0), 0); // início da 1ª linha
  assert.strictEqual(tocLineIndex('a\nb\nc', 2), 1); // início da 2ª linha
  assert.strictEqual(tocLineIndex('a\nb\nc', 4), 2); // início da 3ª linha
  assert.strictEqual(tocLineIndex('a\nb\nc', 5), 2); // fim do texto
});

test('tocLineIndex grampeia entradas fora dos limites', async () => {
  const { tocLineIndex } = await import(modulePath);
  assert.strictEqual(tocLineIndex('a\nb', 999), 1); // caret além do fim
  assert.strictEqual(tocLineIndex('a\nb', -5), 0);  // caret negativo
  assert.strictEqual(tocLineIndex(null, 3), 0);     // texto nulo
  assert.strictEqual(tocLineIndex('a\nb', NaN), 0); // caret inválido
});

test('jumpToTocLine seleciona e rola até o título da linha', async () => {
  const { jumpToTocLine } = await import(modulePath);
  const { editor, calls } = makeEditor([
    { pos: 0, level: 1, text: 'Um' },
    { pos: 10, level: 2, text: 'Dois' },
    { pos: 30, level: 3, text: 'Três' }
  ]);
  assert.strictEqual(jumpToTocLine(editor, 1), true);
  // pos + 1 = logo após a marcação do nó (início do texto do título).
  assert.deepStrictEqual(calls, ['focus', ['selection', 11], 'scrollIntoView', 'run']);
});

test('jumpToTocLine não faz nada fora do intervalo', async () => {
  const { jumpToTocLine } = await import(modulePath);
  const { editor, calls } = makeEditor([{ pos: 0, level: 1, text: 'Um' }]);
  assert.strictEqual(jumpToTocLine(editor, 5), false);
  assert.deepStrictEqual(calls, []);
});

test('jumpToTocLine ignora editor ausente ou destruído', async () => {
  const { jumpToTocLine } = await import(modulePath);
  assert.strictEqual(jumpToTocLine(null, 0), false);
  assert.strictEqual(jumpToTocLine({ isDestroyed: true }, 0), false);
});
