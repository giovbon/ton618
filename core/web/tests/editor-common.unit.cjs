const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

// --- Mock Browser Environment ---
global.window = {};
global.localStorage = {
  store: {},
  getItem(key) { return this.store[key] || null; },
  setItem(key, value) { this.store[key] = value.toString(); },
  clear() { this.store = {}; }
};
global.FormData = class FormData {
  constructor() { this.data = new Map(); }
  append(k, v) { this.data.set(k, v); }
};
global.document = {
  getElementById: () => null,
  addEventListener: () => {},
  activeElement: null
};

let fetchCalls = [];
global.fetch = async (url, options) => {
  fetchCalls.push({ url, options });
  return { ok: true, status: 200, json: async () => ({ ok: true }) };
};

// crypto mock for generateHash
global.crypto = {
  subtle: {
    digest: async (algo, data) => {
      // Return a dummy ArrayBuffer of 32 bytes (SHA-256 size)
      return new Uint8Array(32).fill(42).buffer;
    }
  }
};
global.TextEncoder = class TextEncoder {
  encode(text) { return new Uint8Array(Buffer.from(text)); }
};

// Load the file content
const scriptPath = path.join(__dirname, '..', 'static', 'editor-common.js');
const scriptContent = fs.readFileSync(scriptPath, 'utf8');

// Evaluate the script in this context
eval(scriptContent);

const EditorCommon = global.window.EditorCommon;

test('EditorCommon is loaded', (t) => {
  assert.ok(EditorCommon);
  assert.strictEqual(typeof EditorCommon.generateHash, 'function');
});

test('getAuthHeaders handles tokens', (t) => {
  global.localStorage.clear();
  
  // No token
  assert.deepStrictEqual(EditorCommon.getAuthHeaders(), {});

  // With token without basic
  global.localStorage.setItem('ton_auth', 'mysecrettoken');
  assert.deepStrictEqual(EditorCommon.getAuthHeaders(), {
    'Authorization': 'Basic mysecrettoken'
  });

  // With token with basic
  global.localStorage.setItem('ton_auth', 'Basic othertoken');
  assert.deepStrictEqual(EditorCommon.getAuthHeaders(), {
    'Authorization': 'Basic othertoken'
  });
});

test('generateHash generates consistent length hash', async (t) => {
  const hash = await EditorCommon.generateHash('test data');
  assert.strictEqual(typeof hash, 'string');
  assert.strictEqual(hash.length, 64); // 32 bytes * 2 hex chars
});

test('httpSaveNote makes correct fetch call', async (t) => {
  fetchCalls = [];
  global.localStorage.setItem('ton_auth', 'test');
  
  await EditorCommon.httpSaveNote('notes/test.md', 'content', 'tag1,tag2', true);
  
  assert.strictEqual(fetchCalls.length, 1);
  assert.strictEqual(fetchCalls[0].url, '/api/note/save');
  assert.strictEqual(fetchCalls[0].options.method, 'POST');
  assert.strictEqual(fetchCalls[0].options.headers['Authorization'], 'Basic test');
  assert.ok(fetchCalls[0].options.body instanceof global.FormData);
  assert.strictEqual(fetchCalls[0].options.body.data.get('filename'), 'notes/test.md');
  assert.strictEqual(fetchCalls[0].options.body.data.get('silent'), 'true');
});

test('setStatus updates DOM element correctly', (t) => {
  const el = { textContent: '', className: '' };
  
  EditorCommon.setStatus(el, 'saved');
  assert.strictEqual(el.textContent, '\u2713');
  assert.ok(el.className.includes('emerald'));

  EditorCommon.setStatus(el, 'saving');
  assert.strictEqual(el.textContent, '\u27F3');
  assert.ok(el.className.includes('sky'));

  EditorCommon.setStatus(el, 'dirty');
  assert.strictEqual(el.textContent, '\u25CF');
  assert.ok(el.className.includes('amber'));
});

// ── Paste: markdown vs HTML ──

test('shouldPasteAsMarkdown: texto puro com markdown', (t) => {
  assert.strictEqual(EditorCommon.shouldPasteAsMarkdown('**negrito**', ''), true);
  assert.strictEqual(EditorCommon.shouldPasteAsMarkdown('# Titulo', ''), true);
  assert.strictEqual(EditorCommon.shouldPasteAsMarkdown('- item', ''), true);
  assert.strictEqual(EditorCommon.shouldPasteAsMarkdown('[x](http://a.com)', ''), true);
});

test('shouldPasteAsMarkdown: texto puro sem markdown', (t) => {
  assert.strictEqual(EditorCommon.shouldPasteAsMarkdown('texto simples', ''), false);
  assert.strictEqual(EditorCommon.shouldPasteAsMarkdown('', ''), false);
  assert.strictEqual(EditorCommon.shouldPasteAsMarkdown('', '<p>so html</p>'), false);
});

test('shouldPasteAsMarkdown: HTML de fachada (sem formatacao real) usa markdown', (t) => {
  // Caso do bug: editores de codigo / PDFs / paineis de IA colocam o MESMO texto
  // em text/html, embrulhado em div/p, com os asteriscos crus.
  const text = '**Importante**: veja o *trecho* abaixo';
  const flatHtml = '<div>**Importante**: veja o *trecho* abaixo</div>';
  assert.strictEqual(EditorCommon.shouldPasteAsMarkdown(text, flatHtml), true);

  const pWrapped = '<p><span style="font-family: monospace">**Importante**</span></p>';
  assert.strictEqual(EditorCommon.shouldPasteAsMarkdown(text, pWrapped), true);

  assert.strictEqual(EditorCommon.htmlHasRealFormatting(flatHtml), false);
  assert.strictEqual(EditorCommon.htmlHasRealFormatting('<div><p>x<br></p></div>'), false);
});

test('shouldPasteAsMarkdown: HTML com formatacao real tem prioridade', (t) => {
  const text = '**Importante**: veja o trecho';
  const richHtml = '<p><strong>Importante</strong>: veja o trecho</p>';
  assert.strictEqual(EditorCommon.shouldPasteAsMarkdown(text, richHtml), false);

  // Qualquer tag que carregue semantica visual deve manter o HTML como fonte.
  ['<h1>t</h1>', '<ul><li>x</li></ul>', '<blockquote>c</blockquote>', '<code>x</code>',
   '<table><tr><td>c</td></tr></table>', '<a href="http://a.com">l</a>', '<img src="a.png">',
   '<em>it</em>', '<u>u</u>', '<del>d</del>'].forEach((html) => {
    assert.strictEqual(EditorCommon.htmlHasRealFormatting(html), true, html);
    assert.strictEqual(EditorCommon.shouldPasteAsMarkdown(text, html), false, html);
  });
});

test('textHasMarkdownSyntax reconhece marcadores', (t) => {
  assert.strictEqual(EditorCommon.textHasMarkdownSyntax('2 * 3 = 6'), true);
  assert.strictEqual(EditorCommon.textHasMarkdownSyntax('texto'), false);
  assert.strictEqual(EditorCommon.textHasMarkdownSyntax('linha\n- item'), true);
});

// ── normalizePastedHtml: espaço morto do clipboard ──

test('normalizePastedHtml remove blocos vazios', (t) => {
  // Causa clássica das "linhas em branco gigantes" ao colar.
  assert.strictEqual(EditorCommon.normalizePastedHtml('<p>a</p><p></p><p>b</p>'), '<p>a</p><p>b</p>');
  assert.strictEqual(EditorCommon.normalizePastedHtml('<p>a</p><p><br></p><p>b</p>'), '<p>a</p><p>b</p>');
  assert.strictEqual(EditorCommon.normalizePastedHtml('<p>a</p><p>   </p><p>b</p>'), '<p>a</p><p>b</p>');
  assert.strictEqual(EditorCommon.normalizePastedHtml('<p>a</p><p>&nbsp;</p><p>b</p>'), '<p>a</p><p>b</p>');
  // Caso de chats/painéis: separação feita com <div> vazio.
  assert.strictEqual(EditorCommon.normalizePastedHtml('<div>a</div><div></div><div>b</div>'), '<div>a</div><div>b</div>');
  assert.strictEqual(EditorCommon.normalizePastedHtml('<div>a</div><div><br></div><div>b</div>'), '<div>a</div><div>b</div>');
  // Bloco vazio aninhado (o de dentro esvazia o de fora).
  assert.strictEqual(EditorCommon.normalizePastedHtml('<div><p></p></div><p>x</p>'), '<p>x</p>');
});

test('normalizePastedHtml colapsa quebras duplas', (t) => {
  // <br><br> = uma linha em branco inteira no editor.
  assert.strictEqual(EditorCommon.normalizePastedHtml('<p>a<br><br>b</p>'), '<p>a<br>b</p>');
  assert.strictEqual(EditorCommon.normalizePastedHtml('<p>a<br/><br/><br/>b</p>'), '<p>a<br>b</p>');
  assert.strictEqual(EditorCommon.normalizePastedHtml('<p>a<br>&nbsp;<br>b</p>'), '<p>a<br>b</p>');
  // Quebra simples é preservada.
  assert.strictEqual(EditorCommon.normalizePastedHtml('<p>a<br>b</p>'), '<p>a<br>b</p>');
});

test('normalizePastedHtml remove inline vazio (span com nbsp/espaço)', (t) => {
  // Padrão muito comum em painéis de IA/chat: o bloco "vazio" tem um span dentro.
  assert.strictEqual(
    EditorCommon.normalizePastedHtml('<p>a</p><p><span>&nbsp;</span></p><p>b</p>'),
    '<p>a</p><p>b</p>'
  );
  assert.strictEqual(
    EditorCommon.normalizePastedHtml('<p>a</p><p><span style="color:red"></span></p><p>b</p>'),
    '<p>a</p><p>b</p>'
  );
  assert.strictEqual(
    EditorCommon.normalizePastedHtml('<div>a</div><div><font> </font></div><div>b</div>'),
    '<div>a</div><div>b</div>'
  );
  // Span com conteúdo NÃO é removido.
  assert.strictEqual(
    EditorCommon.normalizePastedHtml('<p><span>texto</span></p>'),
    '<p><span>texto</span></p>'
  );
});

test('normalizePastedHtml remove brs órfãos nas bordas e entre blocos', (t) => {
  // <br> solto entre blocos (separador de parágrafo de vários apps).
  assert.strictEqual(EditorCommon.normalizePastedHtml('<p>a</p><br><p>b</p>'), '<p>a</p><p>b</p>');
  assert.strictEqual(EditorCommon.normalizePastedHtml('<p>a</p><br><br><br><p>b</p>'), '<p>a</p><p>b</p>');
  assert.strictEqual(EditorCommon.normalizePastedHtml('<p>a</p>&nbsp;<p>b</p>'), '<p>a</p><p>b</p>');
  // <br> no fim ou no começo de um parágrafo (virava linha extra).
  assert.strictEqual(EditorCommon.normalizePastedHtml('<p>a<br></p><p>b</p>'), '<p>a</p><p>b</p>');
  assert.strictEqual(EditorCommon.normalizePastedHtml('<p><br>a</p><p>b</p>'), '<p>a</p><p>b</p>');
  // Quebra NO MEIO do parágrafo é intencional e fica.
  assert.strictEqual(EditorCommon.normalizePastedHtml('<p>l1<br>l2</p>'), '<p>l1<br>l2</p>');
});

test('normalizePastedHtml aperta listas loose', (t) => {
  assert.strictEqual(
    EditorCommon.normalizePastedHtml('<ol><li><p>a</p></li><li><p>b</p></li></ol>'),
    '<ol><li>a</li><li>b</li></ol>'
  );
  // Item com 2+ parágrafos é preservado (não é wrapper redundante).
  assert.strictEqual(
    EditorCommon.normalizePastedHtml('<li><p>a</p><p>b</p></li>'),
    '<li><p>a</p><p>b</p></li>'
  );
  // Item vazio some.
  assert.strictEqual(EditorCommon.normalizePastedHtml('<ul><li></li><li>x</li></ul>'), '<ul><li>x</li></ul>');
  assert.strictEqual(EditorCommon.normalizePastedHtml('<ul><li><p></p></li><li>x</li></ul>'), '<ul><li>x</li></ul>');
});

test('normalizePastedHtml preserva conteúdo e remove script/style', (t) => {
  // Conteúdo normal passa intacto (garantia de não regressão).
  const normal = '<p><strong>Importante</strong>: veja <code>AGENTS.md</code></p><ul><li>a</li></ul>';
  assert.strictEqual(EditorCommon.normalizePastedHtml(normal), normal);
  assert.strictEqual(EditorCommon.normalizePastedHtml('<p>texto</p>'), '<p>texto</p>');
  // Nunca traz script/style para dentro da nota.
  assert.strictEqual(EditorCommon.normalizePastedHtml('<style>p{color:red}</style><p>ok</p>'), '<p>ok</p>');
  assert.strictEqual(EditorCommon.normalizePastedHtml('<p>ok</p><script>alert(1)</script>'), '<p>ok</p>');
  // Código com marcações HTML escapadas não é tocado.
  assert.strictEqual(
    EditorCommon.normalizePastedHtml('<pre><code>&lt;br&gt;&lt;br&gt;</code></pre>'),
    '<pre><code>&lt;br&gt;&lt;br&gt;</code></pre>'
  );
  // Whitespace de <pre> é significativo: não pode ser achatado.
  assert.strictEqual(EditorCommon.normalizePastedHtml('<pre>  x  </pre>'), '<pre>  x  </pre>');
  // Célula de tabela vazia é ESTRUTURA, não espaço morto.
  assert.strictEqual(
    EditorCommon.normalizePastedHtml('<table><tr><td></td><td>x</td></tr></table>'),
    '<table><tr><td></td><td>x</td></tr></table>'
  );
  assert.strictEqual(EditorCommon.normalizePastedHtml(''), '');
});

test('normalizePastedHtml no cenário real de painel de IA', (t) => {
  // Payload representativo do que um painel de IA coloca no clipboard:
  // HTML com formatação real + blocos vazios de separação + lista loose.
  const colado =
    '<div><p>Esse erro significa que o Node.js <strong>não encontrou</strong> ' +
    '<code>yargs</code>.</p><div><br></div><p>Isso acontece por:</p>' +
    '<ol><li><p>Dependências não instaladas.</p></li><li><p><span>&nbsp;</span></p></li>' +
    '<li><p>Pasta <code>node_modules</code> corrompida.</p></li></ol><p><br></p></div>';

  const esperado =
    '<div><p>Esse erro significa que o Node.js <strong>não encontrou</strong> ' +
    '<code>yargs</code>.</p><p>Isso acontece por:</p>' +
    '<ol><li>Dependências não instaladas.</li>' +
    '<li>Pasta <code>node_modules</code> corrompida.</li></ol></div>';

  assert.strictEqual(EditorCommon.normalizePastedHtml(colado), esperado);
  // Nenhuma linha em branco sobrou.
  assert.ok(!/<p>\s*<\/p>|<p><br\s*\/?><\/p>|<div><br\s*\/?><\/div>/.test(EditorCommon.normalizePastedHtml(colado)));
});

// ── Rename sem recarregar a página ──

test('renameChangesEditor só bloqueia quando muda de editor', (t) => {
  // Nome comum no editor markdown → pode aplicar na página (sem reload).
  assert.strictEqual(EditorCommon.renameChangesEditor('minha-nota', '/editor'), false);
  assert.strictEqual(EditorCommon.renameChangesEditor('notes/minha-nota.md', '/editor'), false);
  assert.strictEqual(EditorCommon.renameChangesEditor('2026-S38', '/editor'), false);
  // Nome que joga para outro editor → precisa do servidor (reload).
  assert.strictEqual(EditorCommon.renameChangesEditor('meu-desenho', '/editor'), true);
  assert.strictEqual(EditorCommon.renameChangesEditor('notes/xyz-drawing.md', '/editor'), true);
  assert.strictEqual(EditorCommon.renameChangesEditor('mapa-mindmap', '/editor'), true);
  assert.strictEqual(EditorCommon.renameChangesEditor('diagrama-markmap', '/editor'), true);
  // Já estou no editor especializado → não muda nada, segue sem reload.
  assert.strictEqual(EditorCommon.renameChangesEditor('meu-desenho', '/drawing'), false);
  assert.strictEqual(EditorCommon.renameChangesEditor('meu-mindmap', '/mindmap'), false);
  // Case-insensitive e prefixo notes/.
  assert.strictEqual(EditorCommon.renameChangesEditor('Notes/MEU-DESENHO.md', '/editor'), true);
  assert.strictEqual(EditorCommon.renameChangesEditor('', '/editor'), false);
});

test('applyRenameToUI atualiza URL, título, input e sidebar sem reload', (t) => {
  // Ambiente mínimo: histórico e body falsos (a função é defensiva por design).
  const chamadas = { replaceState: [], eventos: [] };
  global.window.location = { pathname: '/editor' };
  global.window.history = {
    replaceState: (a, b, url) => chamadas.replaceState.push(url),
  };
  global.document.title = 'Editor - nota-antiga.md';
  global.document.body = {
    dispatchEvent: (ev) => chamadas.eventos.push(ev.type),
  };

  const input = { value: 'nota-antiga.md', dataset: { filename: 'notes/nota-antiga.md' } };

  const retorno = EditorCommon.applyRenameToUI({
    newName: 'nota-nova',
    filenameInput: input,
    base: '/editor',
  });

  // URL trocada sem reload (replaceState, não pushState → histórico limpo).
  assert.strictEqual(retorno, 'notes/nota-nova.md');
  assert.strictEqual(chamadas.replaceState.length, 1);
  assert.strictEqual(chamadas.replaceState[0], '/editor?file=notes%2Fnota-nova.md');

  // Input: value exibido e data-filename (fonte de salvar/excluir/duplicar).
  assert.strictEqual(input.value, 'nota-nova.md');
  assert.strictEqual(input.dataset.filename, 'notes/nota-nova.md');

  // Título da aba: mantém o prefixo do editor atual.
  assert.strictEqual(global.document.title, 'Editor - nota-nova.md');

  // Sidebar (HTMX) recarregada pelo evento existente.
  assert.deepStrictEqual(chamadas.eventos, ['reload-sidebar']);

  // Prefixo preservado de outros editores (ex: Desenho).
  global.document.title = 'Desenho - desenho-antigo.md';
  EditorCommon.applyRenameToUI({ newName: 'desenho-novo', filenameInput: input, base: '/drawing' });
  assert.strictEqual(global.document.title, 'Desenho - desenho-novo.md');

  delete global.window.history;
  delete global.document.body;
  delete global.window.location;
});

test('applyRenameToUI não quebra em ambiente sem DOM/history', (t) => {
  // Defensivo: sem history/body o rename ainda devolve o filename normalizado.
  const input = { value: '', dataset: {} };
  const nome = EditorCommon.applyRenameToUI({ newName: 'sem-dom', filenameInput: input });
  assert.strictEqual(nome, 'notes/sem-dom.md');
  assert.strictEqual(input.value, 'sem-dom.md');
  assert.strictEqual(input.dataset.filename, 'notes/sem-dom.md');
});
