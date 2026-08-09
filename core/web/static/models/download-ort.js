import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const srcDir = path.join(__dirname, '../../node_modules/onnxruntime-web/dist');
const destDir = path.join(__dirname, 'ort');

// ─────────────────────────────────────────────────────────────────────────
// Variantes do ONNX Runtime WebAssembly que o app realmente usa.
//
// O Transformers.js escolhe o arquivo com base em crossOriginIsolated:
// - "ort-wasm-simd-threaded.asyncify" → usada quando o servidor NÃO envia
//   COOP/COEP (numThreads forçado a 1). É a variante REALMENTE usada para
//   inferência em CPU. OBRIGATÓRIA.
//
// Modo CPU-only (decisão 09/08/2026): o app roda sempre em CPU (WASM),
// então mantemos APENAS a variante asyncify. O device é fixado em "wasm"
// na setting semantic_device. Para reativar WebGPU no futuro, inclua também:
//   'ort-wasm-simd-threaded.jsep.wasm',
//   'ort-wasm-simd-threaded.jsep.mjs',
//
// NÃO copiadas (reduzem a imagem de ~74MB para ~22MB):
// - "ort-wasm-simd-threaded.jsep": WebGPU (removida — CPU-only).
// - "ort-wasm-simd-threaded" (base): exigiria cross-origin isolated.
// - "ort-wasm-simd-threaded.jspi": experimental (JSPI).
// ─────────────────────────────────────────────────────────────────────────
const ALLOWED = [
    'ort-wasm-simd-threaded.asyncify.wasm',
    'ort-wasm-simd-threaded.asyncify.mjs',
];

if (!fs.existsSync(destDir)) {
    fs.mkdirSync(destDir, { recursive: true });
}

if (!fs.existsSync(srcDir)) {
    console.error(`Source directory does not exist: ${srcDir}`);
    console.log("Please run npm install first.");
    process.exit(1);
}

try {
    // Remove variantes antigas que porventura existam no destino
    for (const file of fs.readdirSync(destDir)) {
        if (file.startsWith('ort-wasm')) {
            fs.unlinkSync(path.join(destDir, file));
            console.log(`Removed old ${file}`);
        }
    }

    let copied = 0;
    for (const file of ALLOWED) {
        const srcPath = path.join(srcDir, file);
        if (!fs.existsSync(srcPath)) {
            console.warn(`Skipping missing ${file}`);
            continue;
        }
        const destPath = path.join(destDir, file);
        fs.copyFileSync(srcPath, destPath);
        console.log(`Copied ${file}`);
        copied++;
    }
    console.log(`Successfully copied ${copied} ONNX Runtime WebAssembly assets (apenas as variantes em uso).`);
} catch (err) {
    console.error("FATAL ERROR copying ORT assets:", err);
    process.exit(1);
}
