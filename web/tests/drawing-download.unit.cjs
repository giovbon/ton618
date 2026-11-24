const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

// Fontes únicas: o template da nota de desenho e o bundle-fonte do Excalidraw.
const templ = fs.readFileSync(
  path.join(__dirname, "..", "..", "internal", "features", "notes", "drawing.templ"),
  "utf8"
);
const jsx = fs.readFileSync(path.join(__dirname, "..", "src", "drawing.jsx"), "utf8");

test("desenho: dois botões de download ao lado do duplicar", () => {
  assert.ok(templ.includes('id="download-excalidraw-btn"'), "botão .excalidraw deve existir");
  assert.ok(templ.includes('id="download-png-btn"'), "botão .png deve existir");
  assert.ok(
    templ.includes('onclick="downloadCurrentDrawing()"'),
    "botão .excalidraw chama downloadCurrentDrawing()"
  );
  assert.ok(
    templ.includes('onclick="downloadDrawingImage()"'),
    "botão .png chama downloadDrawingImage()"
  );

  // Ordem na topbar: duplicar → .excalidraw → .png → excluir.
  const ids = [
    "duplicate-editor-btn",
    "download-excalidraw-btn",
    "download-png-btn",
    "delete-editor-btn",
  ];
  const pos = ids.map((id) => templ.indexOf(`id="${id}"`));
  pos.forEach((p, i) => assert.ok(p > 0, `botão ${ids[i]} presente`));
  for (let i = 1; i < pos.length; i++) {
    assert.ok(pos[i] > pos[i - 1], `${ids[i]} deve vir depois de ${ids[i - 1]}`);
  }

  assert.ok(templ.includes("window.downloadCurrentDrawing = downloadCurrentDrawing;"));
  assert.ok(templ.includes("window.downloadDrawingImage = downloadDrawingImage;"));
});

test("desenho: o .excalidraw usa o estado vivo do editor", () => {
  assert.ok(
    templ.includes("api.getSceneElements()") &&
      templ.includes("api.getAppState()") &&
      templ.includes("api.getFiles()"),
    "exporta elementos, appState e arquivos atuais (não o último snapshot salvo)"
  );
  assert.ok(
    templ.includes('drawingBaseName() + ".excalidraw"'),
    "o arquivo baixado tem extensão .excalidraw"
  );
  assert.ok(
    templ.includes("application/vnd.excalidraw+json"),
    "MIME nativo do formato .excalidraw"
  );
});

test("desenho: o PNG usa exportToBlob na resolução padrão", () => {
  assert.ok(templ.includes("exp.exportToBlob({"), "usa o exportToBlob do Excalidraw");
  assert.ok(templ.includes('mimeType: "image/png"'), "exporta PNG");
  assert.ok(templ.includes('drawingBaseName() + ".png"'), "arquivo .png");
  // Resolução padrão = escala natural (1x): nada de maxWidthOrHeight/scale.
  assert.ok(
    !templ.includes("maxWidthOrHeight") && !templ.includes("scale:"),
    "não amplia nem limita: mantém a resolução padrão do Excalidraw"
  );
});

test("desenho: o bundle publica os helpers que a API imperativa não expõe", () => {
  assert.ok(
    jsx.includes("exportToBlob") && jsx.includes("serializeAsJSON"),
    "drawing.jsx importa exportToBlob e serializeAsJSON"
  );
  assert.ok(
    jsx.includes("window.excalidrawExport = { exportToBlob, serializeAsJSON };"),
    "helpers ficam disponíveis no window para o template"
  );
});
