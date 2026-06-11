import * as esbuild from "esbuild";
import { gzipSync } from "zlib";
import { readFileSync, writeFileSync } from "fs";

await esbuild.build({
  entryPoints: ["src/editor.js", "src/spreadsheet.js", "src/drawing.jsx"],
  bundle: true,
  minify: true,
  outdir: "static",
  format: "iife",
  sourcemap: false,
  target: "es2020",
});

// Gera versão .gz para servir com Content-Encoding: gzip
const dataEditor = readFileSync("static/editor.js");
const compressedEditor = gzipSync(dataEditor, { level: 9 });
writeFileSync("static/editor.js.gz", compressedEditor);
console.log(
  `editor.js: ${(dataEditor.length / 1024).toFixed(1)}KB → ${(compressedEditor.length / 1024).toFixed(1)}KB gzip`,
);

const dataSheet = readFileSync("static/spreadsheet.js");
const compressedSheet = gzipSync(dataSheet, { level: 9 });
writeFileSync("static/spreadsheet.js.gz", compressedSheet);
console.log(
  `spreadsheet.js: ${(dataSheet.length / 1024).toFixed(1)}KB → ${(compressedSheet.length / 1024).toFixed(1)}KB gzip`,
);

try {
  const dataDrawing = readFileSync("static/drawing.js");
  const compressedDrawing = gzipSync(dataDrawing, { level: 9 });
  writeFileSync("static/drawing.js.gz", compressedDrawing);
  console.log(
    `drawing.js: ${(dataDrawing.length / 1024).toFixed(1)}KB → ${(compressedDrawing.length / 1024).toFixed(1)}KB gzip`,
  );
} catch (e) {
  console.warn("Aviso: static/drawing.js não pôde ser lido ou comprimido.", e.message);
}
