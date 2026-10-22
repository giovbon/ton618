const test = require('node:test');
const assert = require('node:assert');
const path = require('path');
const { pathToFileURL } = require('url');

// editor-slash.js é ES module (package.json type: module) — importado
// dinamicamente. Em ESM o modo estrito é implícito: atribuir a uma variável
// não declarada lança ReferenceError (foi o que acontecia com _organizeHeadings).
const modulePath = pathToFileURL(
  path.join(__dirname, '..', 'src', 'editor-slash.js')
).href;

function fakeDeps(overrides = {}) {
  return {
    getEditor: () => null,
    getSlashPos: () => null,
    setSlashPos: () => {},
    getSlashFilter: () => '',
    setSlashFilter: () => {},
    hideSlashMenu: () => {},
    organizeHeadings: () => {},
    ...overrides,
  };
}

test('initSlash aceita organizeHeadings sem ReferenceError (ESM strict)', async () => {
  const { initSlash } = await import(modulePath);
  assert.doesNotThrow(() => initSlash(fakeDeps()));
});

test('buildSlashActions funciona após initSlash', async () => {
  const { initSlash, buildSlashActions } = await import(modulePath);
  initSlash(fakeDeps());
  assert.doesNotThrow(() => buildSlashActions());
});

test('scoreSlashCommand prioriza palavra-chave exata', async () => {
  const { scoreSlashCommand } = await import(modulePath);
  const cmd = {
    id: 'organize',
    label: 'Organizar títulos',
    keywords: ['organize', 'organizar', 'titulos'],
  };
  assert.ok(
    scoreSlashCommand(cmd, 'organize') > scoreSlashCommand(cmd, 'org')
  );
});
