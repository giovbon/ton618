import { mkdirSync, writeFileSync, existsSync } from 'fs';
import { dirname, join } from 'path';
import { fileURLToPath } from 'url';
import { gzipSync, brotliCompressSync, constants } from 'zlib';
import http from 'https';

const __dirname = dirname(fileURLToPath(import.meta.url));
const MODEL_DIR = join(__dirname, 'static/models/Xenova/paraphrase-multilingual-MiniLM-L12-v2');

const files = [
  'config.json',
  'special_tokens_map.json',
  'tokenizer.json',
  'tokenizer_config.json',
  'onnx/model_quantized.onnx'
];

function downloadFile(url, destPath) {
  return new Promise((resolve, reject) => {
    console.log(`Downloading ${url} -> ${destPath}...`);
    mkdirSync(dirname(destPath), { recursive: true });

    // Handle redirects
    function get(fileUrl) {
      http.get(fileUrl, (res) => {
        if (res.statusCode === 302 || res.statusCode === 301) {
          get(res.headers.location);
          return;
        }
        if (res.statusCode !== 200) {
          reject(new Error(`Failed to download: ${res.statusCode} ${res.statusMessage}`));
          return;
        }

        const data = [];
        res.on('data', (chunk) => data.push(chunk));
        res.on('end', () => {
          const buffer = Buffer.concat(data);
          writeFileSync(destPath, buffer);
          console.log(`Saved ${destPath} (${(buffer.length / 1024 / 1024).toFixed(2)} MB)`);
          
          // Compress Gzip
          console.log(`Compressing with Gzip...`);
          const gz = gzipSync(buffer, { level: 9 });
          writeFileSync(destPath + '.gz', gz);

          // Compress Brotli
          console.log(`Compressing with Brotli...`);
          const br = brotliCompressSync(buffer, {
            params: {
              [constants.BROTLI_PARAM_QUALITY]: 11,
            },
          });
          writeFileSync(destPath + '.br', br);
          console.log(`Compressed files generated.`);
          
          resolve();
        });
      }).on('error', reject);
    }

    get(url);
  });
}

async function run() {
  for (const file of files) {
    const url = `https://huggingface.co/Xenova/paraphrase-multilingual-MiniLM-L12-v2/resolve/main/${file}`;
    const dest = join(MODEL_DIR, file);
    if (existsSync(dest) && existsSync(dest + '.br')) {
      console.log(`${file} already exists, skipping.`);
      continue;
    }
    await downloadFile(url, dest);
  }
  console.log('Model download and compression complete!');
}

run().catch(console.error);
