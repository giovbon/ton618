/**
 * todos-badge.unit.cjs — Guarda de regressão do badge de contagem de Tasks do cabeçalho.
 *
 * Uso: node --test tests/todos-badge.unit.cjs
 *
 * Contexto (bugs reais, 15/09/2026):
 *
 * 1. `src/app.js` tinha uma `updateTodosCount()` legada que sobrescrevia o conteúdo
 *    de `#nav-todos` via `innerHTML = ...`. Isso DESTRUÍA o `<span id="todos-badge">`
 *    (e seus atributos hx-*) em toda carga de página. Consequências: o número exibido
 *    vinha de `/api/todos?type=all&status=pending&format=json`, que ignora
 *    `todo_markers.count_in_badge`/`active` (o checkbox "Contar" não tinha efeito no
 *    desktop), o swap out-of-band não encontrava mais o alvo (htmx:oobErrorNoTarget) e
 *    o badge era o único elemento que nunca se atualizava sem reload. No mobile o
 *    número ainda era injetado no rótulo "TASK N", duplicando a contagem.
 *
 * 2. Os handlers usavam `hx-on::send-error`, mas o htmx 1.9.12 dispara
 *    `htmx:sendError` (camelCase). O handler ficava registrado num evento que nunca
 *    ocorre, então o aviso "?" de servidor fora do ar não aparecia em falha de rede.
 *
 * 3. O `hx-swap` do container do badge precisa ser `innerHTML`: com `outerHTML` o
 *    elemento (e seus atributos hx-*) é substituído e o badge atualiza só uma vez.
 *
 * A contagem em si vive no backend (`todoBadgeCount` → `db.CountTodosByMarkers`) e é
 * testada em core/internal/features/todos/markers_count_test.go.
 */
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

// ── Config ──
const ROOT = path.resolve(__dirname, '..');
const SRC_DIR = path.join(ROOT, 'src');
const STATIC_DIR = path.join(ROOT, 'static');
const LAYOUT_DIR = path.join(ROOT, 'layout');
const FEATURES_DIR = path.resolve(ROOT, '..', 'internal', 'features');
const TODOS_TEMPL_DIR = path.join(FEATURES_DIR, 'todos');
const NOTES_TEMPL_DIR = path.join(FEATURES_DIR, 'notes');

function read(file) {
  return fs.readFileSync(file, 'utf8');
}

// Remove comentários (linha e bloco) preservando strings.
// Necessário porque o PRÓPRIO comentário que documenta o bug cita os nomes
// proibidos ("#todos-badge", "/api/todos?type=..."): sem isso, a guarda não
// encontraria o bug de verdade e ainda quebraria por causa da documentação.
function stripComments(code) {
  let out = '';
  let quote = null;
  let i = 0;
  while (i < code.length) {
    const c = code[i];
    const next = code[i + 1];
    if (quote) {
      out += c;
      if (c === '\\') {
        out += next === undefined ? '' : next;
        i += 2;
        continue;
      }
      if (c === quote) quote = null;
      i++;
      continue;
    }
    if (c === '"' || c === "'" || c === '`') {
      quote = c;
      out += c;
      i++;
      continue;
    }
    if (c === '/' && next === '/') {
      while (i < code.length && code[i] !== '\n') i++;
      continue;
    }
    if (c === '/' && next === '*') {
      i += 2;
      while (i < code.length && !(code[i] === '*' && code[i + 1] === '/')) i++;
      i += 2;
      continue;
    }
    out += c;
    i++;
  }
  return out;
}

// Recorta uma janela do HTML/template a partir de um seletor (id="x" ou id={ "x" }),
// para checar atributos daquele elemento específico sem depender do resto do arquivo.
function elementWindow(src, id, size = 500) {
  const idx = src.indexOf(`id="${id}"`);
  assert.notStrictEqual(idx, -1, `elemento #${id} não encontrado`);
  return src.slice(idx, idx + size);
}

// ── 1. Ninguém pode destruir o container do badge pelo cliente ──

test('src/app.js não mexe no DOM do badge/links de Tasks (regressão do innerHTML)', () => {
  const code = stripComments(read(path.join(SRC_DIR, 'app.js')));

  const proibidos = [
    ['id="todos-badge"', 'o badge é responsabilidade do HTMX (hx-get/hx-trigger), não de JS imperativo'],
    ['todos-badge', 'qualquer referência ao badge no app.js pode acabar sobrescrevendo o elemento'],
    ['nav-todos', 'sobrescrever #nav-todos por innerHTML destrói o span do badge'],
    ['mobile-nav-todos', 'no mobile o número era injetado no rótulo, duplicando o badge'],
    ['/api/todos?type=', 'endpoint de contagem legado: ignora count_in_badge/active (use /api/todos/count)'],
  ];

  for (const [trecho, motivo] of proibidos) {
    assert.ok(
      !code.includes(trecho),
      `src/app.js não deve conter ${JSON.stringify(trecho)} — ${motivo}.\n` +
        'Atualize a contagem disparando o evento "todos-updated" no body ' +
        '(window.tonRefreshTodosBadge), que o HTMX escuta em #todos-badge.'
    );
  }
});

test('bundle estático static/js/app.js não contém o código legado de contagem', () => {
  const bundle = path.join(STATIC_DIR, 'js', 'app.js');
  assert.ok(fs.existsSync(bundle), `bundle não encontrado: ${bundle} (rode npm run build)`);

  const code = read(bundle);
  for (const trecho of ['todos-badge', 'nav-todos', '/api/todos?type=']) {
    assert.ok(!code.includes(trecho), `bundle não deve conter ${JSON.stringify(trecho)}`);
  }
});

// ── 2. O container do badge no navbar ──

test('navbar tem os dois containers de badge carregados pelo HTMX', () => {
  const navbar = read(path.join(LAYOUT_DIR, 'navbar.templ'));

  for (const id of ['todos-badge', 'mobile-todos-badge']) {
    const win = elementWindow(navbar, id);

    assert.ok(win.includes('hx-get={ "/api/todos/count?r=" + badgeCacheBuster() }'),
      `#${id} deve buscar a contagem em /api/todos/count com cache-buster (?r=)`);
    assert.ok(win.includes('hx-swap="innerHTML"'),
      `#${id} deve usar hx-swap="innerHTML": com outerHTML o elemento (e os atributos hx-*) é ` +
      'substituído e o badge só atualiza uma vez');
    assert.ok(win.includes('hx-trigger="load, todos-updated from:body"'),
      `#${id} deve carregar no load e recontar quando "todos-updated" for disparado no body`);
    assert.ok(win.includes('hx-request=') && win.includes('timeout'),
      `#${id} deve ter timeout na requisição para avisar quando o servidor não responder`);
  }
});

test('handlers de falha do HTMX usam os nomes de evento corretos (camelCase)', () => {
  const navbar = read(path.join(LAYOUT_DIR, 'navbar.templ'));

  assert.ok(navbar.includes('hx-on::sendError='),
    'o htmx 1.9.12 dispara "htmx:sendError": hx-on::send-error registra um listener ' +
    'em "htmx:send-error", que nunca é disparado (o "?" de servidor fora do ar não aparecia)');
  assert.ok(!navbar.includes('hx-on::send-error='),
    'não usar hx-on::send-error (evento inexistente no htmx 1.9.12)');
  assert.ok(navbar.includes('hx-on::timeout='),
    'hx-on::timeout precisa avisar quando a requisição da contagem estoura o timeout');
  assert.ok(navbar.includes('todosBadgeOffline'),
    'deve existir o handler que troca o badge por "?" quando a requisição falha');
});

test('navbar expõe tonRefreshTodosBadge para ações que mudam a tabela de tasks', () => {
  const navbar = read(path.join(LAYOUT_DIR, 'navbar.templ'));
  assert.ok(navbar.includes('window.tonRefreshTodosBadge'),
    'window.tonRefreshTodosBadge deve existir (dispara todos-updated no body)');

  // Excluir nota pela sidebar e arquivar/excluir em massa removem linhas de `todos`:
  // sem o disparo do evento, o badge ficava com o número antigo até um reload.
  const fontes = [
    path.join(NOTES_TEMPL_DIR, 'sidebar_tree.templ'),
    path.join(NOTES_TEMPL_DIR, 'archive_preview.templ'),
    path.join(NOTES_TEMPL_DIR, 'archives_list.templ'),
  ];
  for (const f of fontes) {
    const src = read(f);
    assert.ok(src.includes('tonRefreshTodosBadge()'),
      `${path.relative(ROOT, f)} deve chamar tonRefreshTodosBadge() (hx-on::after-request) ` +
      'porque a ação apaga/cria linhas na tabela `todos`');
  }
});

// ── 3. Badge e atualização out-of-band ──

test('badge.templ renderiza o conteúdo e o swap out-of-band dos dois badges', () => {
  const badge = read(path.join(TODOS_TEMPL_DIR, 'badge.templ'));

  assert.ok(badge.includes('templ TodoBadge(') && badge.includes('templ TodoBadgeOOB('),
    'badge.templ deve ter TodoBadge (conteúdo) e TodoBadgeOOB (swap out-of-band)');

  for (const id of ['#todos-badge', '#mobile-todos-badge']) {
    assert.ok(badge.includes(`hx-swap-oob="innerHTML:${id}"`),
      `TodoBadgeOOB deve atualizar ${id} na mesma resposta (sem depender de evento/reload)`);
  }

  // Contagem 0 não desenha nada (o container continua no DOM, vazio).
  assert.ok(/if count > 0 \{/.test(badge),
    'TodoBadge só deve renderizar a pílula quando count > 0');
});

test('o save do editor avisa o badge para recontar', () => {
  const editor = read(path.join(SRC_DIR, 'editor-init.js'));
  assert.ok(editor.includes('dispatchEvent(new Event("todos-updated"))'),
    'editor-init.js deve disparar "todos-updated" no body após salvar ' +
    '(a nota pode ter criado/removido tarefas)');
});
