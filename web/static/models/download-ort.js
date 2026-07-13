import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const srcDir = path.join(__dirname, '../../node_modules/onnxruntime-web/dist');
const destDir = path.join(__dirname, 'ort');

if (!fs.existsSync(destDir)) {
    fs.mkdirSync(destDir, { recursive: true });
}

if (!fs.existsSync(srcDir)) {
    console.error(`Source directory does not exist: ${srcDir}`);
    console.log("Please run npm install first.");
    process.exit(1);
}

try {
    const files = fs.readdirSync(srcDir);
    let copied = 0;
    for (const file of files) {
        if (file.startsWith('ort-wasm')) {
            const srcPath = path.join(srcDir, file);
            const destPath = path.join(destDir, file);
            fs.copyFileSync(srcPath, destPath);
            console.log(`Copied ${file}`);
            copied++;
        }
    }
    console.log(`Successfully copied ${copied} ONNX Runtime WebAssembly assets!`);
} catch (err) {
    console.error("FATAL ERROR copying ORT assets:", err);
    process.exit(1);
}
