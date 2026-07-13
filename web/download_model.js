import { mkdirSync, writeFileSync, readFileSync, existsSync } from 'fs';
import { dirname, join } from 'path';
import { fileURLToPath } from 'url';
import { gzipSync, brotliCompressSync, constants } from 'zlib';
import { execSync } from 'child_process';

const __dirname = dirname(fileURLToPath(import.meta.url));
const MODEL_DIR = join(__dirname, 'static/models/Xenova/paraphrase-multilingual-MiniLM-L12-v2');

const files = [
  'config.json',
  'special_tokens_map.json',
  'tokenizer.json',
  'tokenizer_config.json',
  'onnx/model_quantized.onnx'
];

/**
 * Baixa um arquivo usando wget (disponível em Alpine e Ubuntu).
 * wget lida melhor com CDNs como XetHub/CAS Bridge do HuggingFace.
 */
function downloadFile(url, destPath) {
  console.log(`Downloading ${url} -> ${destPath}...`);
  mkdirSync(dirname(destPath), { recursive: true });

  const isLarge = url.includes('model_quantized.onnx') || url.includes('tokenizer.json');
  const timeout = isLarge ? 600 : 180;

  try {
    execSync(
      `wget ` +
      `--timeout=${timeout} --tries=3 --waitretry=5 ` +
      `--user-agent="Mozilla/5.0 (compatible; ton618-builder)" ` +
      `-O "${destPath}" ` +
      `"${url}"`,
      { stdio: 'inherit', timeout: (timeout + 30) * 1000 }
    );
  } catch (err) {
    throw new Error(`Failed to download: wget exited with error`);
  }

  // Lê o arquivo baixado para compactar
  const buffer = readFileSync(destPath);
  console.log(`Saved ${destPath} (${(buffer.length / 1024 / 1024).toFixed(2)} MB)`);

  // ── Compressão Gzip ──
  console.log(`Compressing with Gzip...`);
  const gz = gzipSync(buffer, { level: 9 });
  writeFileSync(destPath + '.gz', gz);
  console.log(`  Gzip: ${(gz.length / 1024 / 1024).toFixed(2)} MB`);

  // ── Compressão Brotli (qualidade moderada p/ economizar memória) ──
  console.log(`Compressing with Brotli (quality 4)...`);
  const br = brotliCompressSync(buffer, {
    params: {
      [constants.BROTLI_PARAM_QUALITY]: 4,
    },
  });
  writeFileSync(destPath + '.br', br);
  console.log(`  Brotli: ${(br.length / 1024 / 1024).toFixed(2)} MB`);
  console.log(`Compressed files generated.`);
}

/**
 * Tenta baixar um arquivo com até `retries` tentativas.
 */
async function downloadWithRetry(url, destPath, retries = 3) {
  for (let attempt = 1; attempt <= retries; attempt++) {
    try {
      downloadFile(url, destPath);
      return;
    } catch (err) {
      console.error(`Attempt ${attempt}/${retries} failed: ${err.message}`);
      if (attempt < retries) {
        const wait = Math.min(1000 * Math.pow(2, attempt), 30000);
        console.log(`Retrying in ${wait / 1000}s...`);
        await new Promise((r) => setTimeout(r, wait));
      } else {
        throw new Error(`All ${retries} attempts failed for ${url.split('/').pop()}: ${err.message}`);
      }
    }
  }
}

async function run() {
  for (const file of files) {
    const url = `https://huggingface.co/Xenova/paraphrase-multilingual-MiniLM-L12-v2/resolve/main/${file}`;
    const dest = join(MODEL_DIR, file);
    if (existsSync(dest) && existsSync(dest + '.br')) {
      console.log(`${file} already exists, skipping.`);
      continue;
    }
    await downloadWithRetry(url, dest);
  }
  console.log('Model download and compression complete!');
}

run().catch((err) => {
  console.error("FATAL ERROR:", err);
  process.exit(1);
});
