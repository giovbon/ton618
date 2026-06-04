import * as esbuild from "esbuild";
import { gzipSync } from "zlib";
import { readFileSync, writeFileSync } from "fs";

await esbuild.build({
  entryPoints: ["src/editor.js"],
  bundle: true,
  minify: true,
  outfile: "static/editor.js",
  format: "iife",
  sourcemap: false,
  target: "es2020",
});

// Gera versão .gz para servir com Content-Encoding: gzip
const data = readFileSync("static/editor.js");
const compressed = gzipSync(data, { level: 9 });
writeFileSync("static/editor.js.gz", compressed);
console.log(
  `editor.js: ${(data.length / 1024).toFixed(1)}KB → ${(compressed.length / 1024).toFixed(1)}KB gzip`,
);
