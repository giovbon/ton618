import { mkdirSync, writeFileSync, existsSync } from 'fs';
import { dirname, join } from 'path';
import { fileURLToPath } from 'url';
import { gzipSync, brotliCompressSync, constants } from 'zlib';

const __dirname = dirname(fileURLToPath(import.meta.url));
const MODEL_DIR = join(__dirname, 'static/models/Xenova/paraphrase-multilingual-MiniLM-L12-v2');

const files = [
  'config.json',
  'special_tokens_map.json',
  'tokenizer.json',
  'tokenizer_config.json',
  'onnx/model_quantized.onnx'
];

/** Aguarda N ms */
function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms));
}

/**
 * Baixa um arquivo usando fetch nativo do Node.js 20+.
 * fetch segue redirects automaticamente e lida com CDNs como XetHub.
 */
async function downloadFile(url, destPath) {
  console.log(`Downloading ${url} -> ${destPath}...`);
  mkdirSync(dirname(destPath), { recursive: true });

  const isLarge = url.includes('model_quantized.onnx') || url.includes('tokenizer.json');
  const timeout = isLarge ? 600000 : 180000;

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeout);

  try {
    const response = await fetch(url, {
      signal: controller.signal,
      headers: {
        'User-Agent': 'Mozilla/5.0 (compatible; ton618-builder)',
        'Accept': '*/*',
      },
      // Segue redirects automaticamente (padrão: true)
      redirect: 'follow',
    });

    if (!response.ok) {
      throw new Error(`Failed to download: ${response.status} ${response.statusText}`);
    }

    const contentLength = parseInt(response.headers.get('content-length') || '0', 10);
    console.log(`Content-Length: ${(contentLength / 1024 / 1024).toFixed(2)} MB`);

    // Leitura com progresso
    const reader = response.body.getReader();
    const chunks = [];
    let downloaded = 0;

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      chunks.push(value);
      downloaded += value.length;
      if (contentLength > 0) {
        const pct = ((downloaded / contentLength) * 100).toFixed(1);
        process.stdout.write(`\r  Progress: ${pct}% (${(downloaded / 1024 / 1024).toFixed(2)} MB)`);
      }
    }
    process.stdout.write('\n');

    // Concatena os chunks em um buffer
    const buffer = Buffer.concat(chunks.map(c => Buffer.from(c)));
    writeFileSync(destPath, buffer);
    console.log(`Saved ${destPath} (${(buffer.length / 1024 / 1024).toFixed(2)} MB)`);

    // Libera referência
    chunks.length = 0;

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

  } finally {
    clearTimeout(timer);
  }
}

/**
 * Tenta baixar um arquivo com até `retries` tentativas.
 */
async function downloadWithRetry(url, destPath, retries = 3) {
  for (let attempt = 1; attempt <= retries; attempt++) {
    try {
      await downloadFile(url, destPath);
      return;
    } catch (err) {
      console.error(`Attempt ${attempt}/${retries} failed: ${err.message}`);
      if (attempt < retries) {
        const wait = Math.min(1000 * Math.pow(2, attempt), 30000);
        console.log(`Retrying in ${wait / 1000}s...`);
        await sleep(wait);
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
