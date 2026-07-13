import { mkdirSync, writeFileSync, existsSync } from 'fs';
import { dirname, join } from 'path';
import { fileURLToPath } from 'url';
import { gzipSync, brotliCompressSync, constants } from 'zlib';
import https from 'https';

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
 * Baixa um arquivo com timeout e retry.
 * Timeout maior para arquivos grandes (>= 10 MB), menor para os pequenos.
 */
function downloadFile(url, destPath) {
  return new Promise((resolve, reject) => {
    console.log(`Downloading ${url} -> ${destPath}...`);
    mkdirSync(dirname(destPath), { recursive: true });

    const req = https.get(url, (res) => {
      // Handle redirects — evita race condition com req.destroy()
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        const redirectUrl = new URL(res.headers.location, url).toString();
        console.log(`Redirected to ${redirectUrl}`);
        res.resume(); // Consome a resposta para liberar memória
        // Chama recursivamente sem destruir o req, apenas ignorando-o
        downloadFile(redirectUrl, destPath).then(resolve).catch(reject);
        return;
      }
      if (res.statusCode !== 200) {
        reject(new Error(`Failed to download: ${res.statusCode} ${res.statusMessage}`));
        return;
      }

      const contentLength = parseInt(res.headers['content-length'] || '0', 10);
      console.log(`Content-Length: ${(contentLength / 1024 / 1024).toFixed(2)} MB`);

      const data = [];
      let downloaded = 0;
      res.on('data', (chunk) => {
        data.push(chunk);
        downloaded += chunk.length;
        if (contentLength > 0) {
          const pct = ((downloaded / contentLength) * 100).toFixed(1);
          process.stdout.write(`\r  Progress: ${pct}% (${(downloaded / 1024 / 1024).toFixed(2)} MB)`);
        }
      });
      res.on('end', () => {
        process.stdout.write('\n');
        const buffer = Buffer.concat(data);
        writeFileSync(destPath, buffer);
        console.log(`Saved ${destPath} (${(buffer.length / 1024 / 1024).toFixed(2)} MB)`);

        // Libera a referência do data array antes de comprimir
        data.length = 0;

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

        resolve();
      });
    });

    // Timeout: 10 min para arquivos grandes, 3 min para pequenos
    const isLarge = url.includes('model_quantized.onnx') || url.includes('tokenizer.json');
    req.setTimeout(isLarge ? 600000 : 180000, () => {
      req.destroy();
      reject(new Error(`Request timed out downloading ${url.split('/').pop()}`));
    });

    req.on('error', reject);
  });
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
