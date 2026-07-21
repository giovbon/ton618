/**
 * auth-headers.unit.cjs — Verifica se todos os fetch() calls em endpoints
 * POST protegidos incluem headers de autenticação.
 *
 * Uso: node --test tests/auth-headers.unit.cjs
 *
 * Contexto: O servidor usa BasicAuth (admin:ton618). Fetch calls que não
 * incluirem headers: EditorCommon.getAuthHeaders() receberão 401 quando
 * o cookie de auth expirar (max-age=86400 = 24h).
 */
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

// ── Config ──
const ROOT = path.resolve(__dirname, '..');
const STATIC_DIR = path.join(ROOT, 'static');
const SRC_DIR = path.join(ROOT, 'src');
const TEMPL_DIR = path.resolve(ROOT, '..', 'internal', 'features', 'notes');

// Endpoints POST que exigem autenticação
const AUTH_ENDPOINTS = [
  '/file/save',
  '/api/note/save',
  '/file/rename',
  '/file/delete',
  '/api/notes/delete',
  '/api/notes/rename',
  '/api/note/duplicate',
  '/api/upload-image',
  '/api/embeddings/save',
  '/api/sync',
];

// Padrões de fetch que serão ignorados (GET, públicos, ou que têm tratamento especial)
const SKIP_PATTERNS = [
  '/api/notes',          // GET
  '/api/tags',           // GET
  '/api/status',         // usada no login
  '/api/health',         // pública
  '/api/sidebar',        // GET
  '/api/archives',       // GET
  '/api/todos',          // GET
  '/api/embeddings/status', // GET
  '/api/embeddings/pending', // GET
  '/api/settings/',      // GET
  '/api/epub/position',  // GET
  'htmx:',               // HTMX
  '/static/',            // estático
  '/login',              // página de login
];

function isSkippedEndpoint(url) {
  return SKIP_PATTERNS.some(p => url.includes(p));
}

/**
 * Extrai URLs de fetch() calls de um trecho de código.
 * Ignora comentários de linha (//) e blocos (/* ... * /).
 */
function extractFetchUrls(code, filePath) {
  const urls = [];
  const lines = code.split('\n');

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const trimmed = line.trim();

    // Pula comentários de linha
    if (trimmed.startsWith('//')) continue;

    // Pula linhas dentro de comentários multi-linha (detecta início/fim)
    if (trimmed.startsWith('*') || trimmed.startsWith('/*') || trimmed.startsWith('* /')) continue;

    // Procura por fetch("URL" ou fetch('URL'
    const fetchMatch = trimmed.match(/fetch\s*\(\s*["']([^"']+)["']/);
    if (!fetchMatch) continue;

    const url = fetchMatch[1];

    // Ignora endpoints GET ou públicos
    if (isSkippedEndpoint(url)) continue;

    // Ignora se não for um endpoint que requer auth
    const needsAuth = AUTH_ENDPOINTS.some(ep => url.startsWith(ep));
    if (!needsAuth) continue;

    urls.push({ url, line: i + 1, code: trimmed });
  }

  return urls;
}

/**
 * Verifica se a linha (ou linhas próximas) contém headers de autenticação.
 * Procura por: headers: EditorCommon.getAuthHeaders() ou headers: getAuthHeaders()
 */
function hasAuthHeaders(lines, lineIndex) {
  const contextLines = [];
  // Pega algumas linhas ao redor para capturar objetos multi-linha
  for (let i = Math.max(0, lineIndex - 1); i < Math.min(lines.length, lineIndex + 6); i++) {
    contextLines.push(lines[i]);
  }
  const context = contextLines.join('\n');

  // Verifica presença de auth headers no contexto
  const authPatterns = [
    /headers:\s*EditorCommon\.getAuthHeaders\(\)/,
    /headers:\s*getAuthHeaders\(\)/,
    /headers:\s*this\.getAuthHeaders\(\)/,
  ];

  return authPatterns.some(p => p.test(context));
}

// ── Arquivos a verificar ──
function* scanFiles() {
  // 1. Arquivos .templ (backend templates com JS inline)
  if (fs.existsSync(TEMPL_DIR)) {
    const templFiles = fs.readdirSync(TEMPL_DIR)
      .filter(f => f.endsWith('.templ'))
      .map(f => path.join(TEMPL_DIR, f));
    for (const f of templFiles) yield f;
  }

  // 2. Arquivos .js em web/src/
  if (fs.existsSync(SRC_DIR)) {
    const srcFiles = fs.readdirSync(SRC_DIR)
      .filter(f => f.endsWith('.js') || f.endsWith('.jsx'))
      .map(f => path.join(SRC_DIR, f));
    for (const f of srcFiles) yield f;
  }

  // 3. editor-common.js em web/static/
  const commonJs = path.join(STATIC_DIR, 'editor-common.js');
  if (fs.existsSync(commonJs)) yield commonJs;
}

// ── Testes ──

test('todos os fetch calls a endpoints POST protegidos incluem auth headers', (t) => {
  let totalErrors = 0;
  const errors = [];

  for (const filePath of scanFiles()) {
    const relativePath = path.relative(ROOT, filePath);
    const code = fs.readFileSync(filePath, 'utf8');
    const lines = code.split('\n');

    const fetchUrls = extractFetchUrls(code, filePath);

    for (const { url, line, code: lineCode } of fetchUrls) {
      const lineIndex = line - 1; // 0-based
      if (!hasAuthHeaders(lines, lineIndex)) {
        totalErrors++;
        errors.push(
          `  ${relativePath}:${line} — fetch("${url}") sem auth headers\n` +
          `    Código: ${lineCode.trim()}`
        );
      }
    }
  }

  if (totalErrors > 0) {
    const msg = `\n${totalErrors} fetch call(s) sem headers de autenticação:\n\n${errors.join('\n\n')}\n\n` +
      `Adicione , headers: EditorCommon.getAuthHeaders() (ou getAuthHeaders() no map.templ) ` +
      `a cada fetch() para endpoint POST protegido por BasicAuth.`;
    assert.fail(msg);
  }
});

test('doRenameContent em editor-common.js inclui auth headers', (t) => {
  const filePath = path.join(STATIC_DIR, 'editor-common.js');
  const code = fs.readFileSync(filePath, 'utf8');
  const lines = code.split('\n');

  // Verifica se a função doRenameContent tem auth headers nos fetch calls internos
  // Procura a seção da função
  const renameSection = code.indexOf('doRenameContent:');
  assert.ok(renameSection >= 0, 'doRenameContent deve existir em editor-common.js');

  const relevantCode = code.substring(renameSection);

  // Dentro de doRenameContent: fetch("/file/rename", ...) deve ter headers
  const renameMatch = relevantCode.match(/fetch\s*\(\s*["']\/file\/rename["'][^)]*\)/);
  if (renameMatch) {
    assert.ok(
      renameMatch[0].includes('getAuthHeaders'),
      'fetch("/file/rename", ...) dentro de doRenameContent deve incluir headers de auth'
    );
  }

  // fetch("/api/note/save", ...) deve ter headers
  const saveMatch = relevantCode.match(/fetch\s*\(\s*["']\/api\/note\/save["'][^)]*\)/);
  if (saveMatch) {
    assert.ok(
      saveMatch[0].includes('getAuthHeaders'),
      'fetch("/api/note/save", ...) dentro de doRenameContent deve incluir headers de auth'
    );
  }
});

test('http wrappers em editor-common.js incluem auth headers', (t) => {
  const filePath = path.join(STATIC_DIR, 'editor-common.js');
  const code = fs.readFileSync(filePath, 'utf8');

  const wrappers = ['httpSaveNote', 'httpSaveFile', 'httpRename', 'httpDelete', 'httpDuplicate'];

  for (const wrapper of wrappers) {
    const idx = code.indexOf(`${wrapper}:`);
    assert.ok(idx >= 0, `editor-common.js deve exportar ${wrapper}`);

    // Pega o trecho da função (até o próximo fechamento de chave ou vírgula de separação)
    const snippet = code.substring(idx, idx + 500);

    assert.ok(
      snippet.includes('getAuthHeaders'),
      `${wrapper} em editor-common.js deve incluir getAuthHeaders()`
    );
  }
});
