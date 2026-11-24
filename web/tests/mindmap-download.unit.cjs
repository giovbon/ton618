const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

// Fonte única dos testes: o template do editor de markmap (não existe build
// intermediário — os *_templ.go são gerados a partir dele).
const templPath = path.join(__dirname, "..", "..", "internal", "features", "notes", "mindmap.templ");
const templ = fs.readFileSync(templPath, "utf8");

test("mindmap: dois botões de download ao lado do duplicar", () => {
  assert.ok(templ.includes('id="download-md-btn"'), "botão de baixar o Markdown deve existir");
  assert.ok(templ.includes('id="download-image-btn"'), "botão de baixar a imagem deve existir");
  assert.ok(templ.includes('onclick="downloadCurrentNote()"'), "botão .md chama downloadCurrentNote()");
  assert.ok(templ.includes('onclick="downloadMindmapImage()"'), "botão de imagem chama downloadMindmapImage()");

  // Ordem na topbar: duplicar → markdown → imagem → excluir.
  const ids = ["duplicate-editor-btn", "download-md-btn", "download-image-btn", "delete-editor-btn"];
  const pos = ids.map((id) => templ.indexOf(`id="${id}"`));
  pos.forEach((p, i) => assert.ok(p > 0, `botão ${ids[i]} presente`));
  for (let i = 1; i < pos.length; i++) {
    assert.ok(pos[i] > pos[i - 1], `${ids[i]} deve vir depois de ${ids[i - 1]}`);
  }

  // onclick exige handler global.
  assert.ok(templ.includes("window.downloadCurrentNote = downloadCurrentNote;"));
  assert.ok(templ.includes("window.downloadMindmapImage = downloadMindmapImage;"));
});

test("mindmap: o .md baixado é a MESMA fonte do save da nota", () => {
  assert.ok(
    templ.includes("EditorCommon.downloadMarkdown("),
    "usa o downloadMarkdown compartilhado do EditorCommon"
  );
  assert.ok(
    templ.includes("ensureMindmapFrontmatter(jar.toString())"),
    "conteúdo vem de ensureMindmapFrontmatter(jar.toString()) — igual ao doSave/doRename"
  );
});

test("mindmap: a imagem sai do SVG da pré-visualização, com fundo do app", () => {
  assert.ok(
    templ.includes("EditorCommon.downloadSvgPng(svgEl,"),
    "exporta o SVG renderizado pelo markmap (svgEl)"
  );
  assert.ok(
    templ.includes('background: "#09090b"'),
    "o fundo do PNG é o mesmo da pré-visualização (#09090b)"
  );
  // Sem prévia renderizada (mobile) não há o que exportar.
  assert.ok(templ.includes("if (!mmWrapper)"), "guarda quando o markmap não foi inicializado");
});
