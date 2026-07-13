import * as esbuild from "esbuild";
import { gzipSync, brotliCompressSync, constants } from "zlib";
import { readFileSync, writeFileSync, readdirSync, statSync, unlinkSync } from "fs";
import { join } from "path";
import { execSync } from "child_process";

console.log("Compilando Tailwind CSS...");
try {
  execSync("npx tailwindcss -c tailwind.config.cjs -i src/input.css -o static/app.css --minify", { stdio: "inherit" });
} catch (error) {
  console.error("Erro ao compilar Tailwind CSS:", error.message);
  process.exit(1);
}

// Validação de sanidade: garante que o Tailwind encontrou as classes do backend (internal/features)
try {
  const cssContent = readFileSync("static/app.css", "utf8");
  const requiredClasses = ["green-500", "prose"];
  for (const cls of requiredClasses) {
    if (!cssContent.includes(cls)) {
      throw new Error(`A classe obrigatória "${cls}" não foi encontrada no static/app.css. Verifique se o Tailwind está escaneando as pastas do backend (internal/features) corretamente.`);
    }
  }
  console.log("Validação de sanidade do CSS concluída com sucesso!");
} catch (error) {
  console.error("FALHA CRÍTICA NO BUILD DO CSS:", error.message);
  process.exit(1);
}


await esbuild.build({
  entryPoints: ["src/editor.js", "src/spreadsheet.js", "src/drawing.jsx", "src/mindmap.js", "src/map.js", "src/semantic.js"],
  bundle: true,
  minify: true,
  outdir: "static",
  format: "iife",
  sourcemap: false,
  target: "es2020",
  loader: {
    ".woff2": "dataurl",
    ".woff": "dataurl",
    ".ttf": "dataurl",
    ".svg": "dataurl"
  },
});

// Build separado para o Web Worker de embeddings semânticos.
// Usa format: "esm" pois Web Workers com módulos precisam de ESM.
await esbuild.build({
  entryPoints: ["src/semantic-worker.js"],
  bundle: true,
  minify: true,
  outfile: "static/semantic-worker.js",
  format: "esm",
  sourcemap: false,
  target: "es2020",
});

console.log("Semantic worker compilado com sucesso!");

console.log("Comprimindo assets estáticos (Gzip & Brotli)...");

function compressFile(filePath) {
  if (filePath.endsWith(".gz") || filePath.endsWith(".br")) return;
  const data = readFileSync(filePath);
  
  // Gzip
  const gzipData = gzipSync(data, { level: 9 });
  writeFileSync(filePath + ".gz", gzipData);
  
  // Brotli
  const brotliData = brotliCompressSync(data, {
    params: {
      [constants.BROTLI_PARAM_QUALITY]: 11,
    },
  });
  writeFileSync(filePath + ".br", brotliData);
  
  console.log(
    `  ${filePath}: ${(data.length / 1024).toFixed(1)}KB → Gzip: ${(gzipData.length / 1024).toFixed(1)}KB, Brotli: ${(brotliData.length / 1024).toFixed(1)}KB`
  );
}

function compressDirectory(dirPath) {
  const files = readdirSync(dirPath);
  for (const file of files) {
    const fullPath = join(dirPath, file);
    const stat = statSync(fullPath);
    if (stat.isDirectory()) {
      compressDirectory(fullPath);
    } else if (file.endsWith(".js") || file.endsWith(".css")) {
      compressFile(fullPath);
    }
  }
}

function cleanCompressedFiles(dirPath) {
  const files = readdirSync(dirPath);
  for (const file of files) {
    const fullPath = join(dirPath, file);
    const stat = statSync(fullPath);
    if (stat.isDirectory()) {
      cleanCompressedFiles(fullPath);
    } else if (file.endsWith(".gz") || file.endsWith(".br")) {
      try {
        unlinkSync(fullPath);
      } catch (err) {
        console.warn(`Erro ao deletar ${fullPath}:`, err.message);
      }
    }
  }
}

const isDev = process.argv.includes("--dev") || process.env.NODE_ENV === "development";

if (isDev) {
  console.log("Modo desenvolvimento: limpando arquivos Gzip e Brotli para evitar cache do servidor...");
  try {
    cleanCompressedFiles("static");
  } catch (error) {
    console.warn("Aviso ao limpar assets compactados:", error.message);
  }
} else {
  try {
    compressDirectory("static");
  } catch (error) {
    console.warn("Aviso durante compressão de assets:", error.message);
  }
}
