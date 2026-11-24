const test = require("node:test");
const assert = require("node:assert/strict");
const path = require("node:path");

// --- Mock Browser Environment ---
// tabulator-note.js é um IIFE que só toca `window` (sem DOM no carregamento),
// então basta um `window` falso para exercitar os helpers puros.
global.window = {};
// tabulator-note.js usa EditorCommon.escapeHtml (SSOT). editor-common.js
// registra um listener de document no load — stub suficiente para o Node.
global.document = { addEventListener: function () {} };
require(path.join(__dirname, "..", "static", "editor-common.js"));
global.EditorCommon = global.window.EditorCommon;

require(path.join(__dirname, "..", "src", "tabulator-note.js"));

const U = global.window.TabulatorNoteUtils;

test("tabulator-note: normalizeType aceita sinônimos em pt/en", () => {
  assert.equal(U.normalizeType("texto"), "string");
  assert.equal(U.normalizeType(""), "string");
  assert.equal(U.normalizeType("qualquer coisa"), "string");
  assert.equal(U.normalizeType("número"), "number");
  assert.equal(U.normalizeType("NÚMERO"), "number");
  assert.equal(U.normalizeType("num"), "number");
  assert.equal(U.normalizeType("data"), "date");
  assert.equal(U.normalizeType("date"), "date");
  assert.equal(U.normalizeType("sim/não"), "boolean");
  assert.equal(U.normalizeType("booleano"), "boolean");
});

test("tabulator-note: sorterForType mapeia tipos para comparadores do Tabulator", () => {
  assert.equal(U.sorterForType("string"), "string");
  assert.equal(U.sorterForType(undefined), "string");
  assert.equal(U.sorterForType("number"), "number");
  assert.equal(U.sorterForType("date"), "date");
  assert.equal(U.sorterForType("boolean"), "boolean");
  assert.equal(U.sorterForType("file"), "string");
});

test("tabulator-note: editorForType mapeia tipos para editores do Tabulator", () => {
  assert.equal(U.editorForType("string"), "input");
  assert.equal(U.editorForType(undefined), "input");
  assert.equal(U.editorForType("number"), "number");
  assert.equal(U.editorForType("date"), "date");
  assert.equal(U.editorForType("boolean"), "tickCross");
  // Colunas "arquivo" não têm editor inline (o upload é feito no clique).
  assert.equal(U.editorForType("file"), false);
});

test("tabulator-note: normalizeType reconhece 'arquivo'/'file'/'anexo'", () => {
  assert.equal(U.normalizeType("arquivo"), "file");
  assert.equal(U.normalizeType("ARQUIVO"), "file");
  assert.equal(U.normalizeType("file"), "file");
  assert.equal(U.normalizeType("anexo"), "file");
  assert.equal(U.isFileType("file"), true);
  assert.equal(U.isFileType("string"), false);
  assert.equal(U.typeLabel("file"), "arquivo");
});

test("tabulator-note: seletor de tipo cobre os 5 tipos lógicos (sem digitar)", () => {
  const values = U.columnTypes.map((c) => c.value);
  assert.deepEqual(values, ["string", "number", "date", "boolean", "file"]);
  for (const c of U.columnTypes) {
    // O `value` do submenu é o mesmo gravado no JSON (normalizeType idempotente).
    assert.equal(U.normalizeType(c.value), c.value, `normalizeType(${c.value})`);
    assert.ok(c.label && c.icon, `opção incompleta: ${c.value}`);
  }
});

test("tabulator-note: helpers da coluna de arquivo", () => {
  assert.equal(U.fileNameFromPath("datasets/relatorio final.pdf"), "relatorio final.pdf");
  assert.equal(U.fileNameFromPath(""), "");
  assert.equal(
    U.fileDownloadURL("datasets/relatorio final.pdf"),
    "/file/download?name=datasets/relatorio%20final.pdf",
  );
  assert.equal(U.escapeHtml('<a href="x">&'), "&lt;a href=&quot;x&quot;&gt;&amp;");
});

test("tabulator-note: genField nunca colide com campos internos (_idx/_rownum/_actions)", () => {
  const seen = new Set();
  for (let i = 0; i < 200; i++) {
    const f = U.genField();
    assert.ok(f.startsWith("c"), `field inesperado: ${f}`);
    assert.ok(!f.startsWith("_"), `field não pode começar com _: ${f}`);
    assert.ok(!seen.has(f), `field duplicado: ${f}`);
    seen.add(f);
  }
});
