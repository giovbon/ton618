/**
 * query-panel.unit.cjs — Guarda de contrato do painel de Consultas.
 *
 * Uso: node --test tests/query-panel.unit.cjs
 *
 * Contexto (DECISIONS §6.29): o painel é um container `#query-panel` carregado
 * pelo HTMX (`hx-get="/api/query?file=…"`) e recarregado quando o corpo recebe o
 * evento `query-blocks-updated`, disparado pelo editor DEPOIS de salvar a nota.
 *
 * O contrato tem três pontas em arquivos diferentes:
 *   1. o template do editor declara o container e o `hx-trigger` do evento;
 *   2. o JS do editor dispara o evento;
 *   3. o handler Go registra a rota usada pelo `hx-get`.
 * Renomear o evento (ou a rota) só num lado faz o painel parar de atualizar em
 * silêncio — exatamente o tipo de regressão que este teste pega.
 */
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

// ── Config ──
const ROOT = path.resolve(__dirname, '..');
const SRC_DIR = path.join(ROOT, 'src');
const STATIC_DIR = path.join(ROOT, 'static');
const FEATURES_DIR = path.resolve(ROOT, '..', 'internal', 'features', 'notes');
const ROUTES = path.resolve(ROOT, '..', 'cmd', 'server', 'routes.go');

const EVENTO = 'query-blocks-updated';
const ROTA = '/api/query';
const ALVO = '#query-panel';

function ler(p) {
  return fs.readFileSync(p, 'utf8');
}

test('editor.templ declara o container #query-panel com o evento de atualização', () => {
  const templ = ler(path.join(FEATURES_DIR, 'editor.templ'));

  assert.match(templ, /id="query-panel"/, 'o container #query-panel sumiu de editor.templ');
  assert.ok(
    templ.includes(`hx-trigger="load, ${EVENTO} from:body"`),
    `editor.templ precisa escutar "load, ${EVENTO} from:body"`
  );
  assert.ok(
    templ.includes(`hx-get={ "${ROTA}?file=" + SafeFileQueryEscape(data.Filename) }`),
    `o container deve buscar "${ROTA}?file=…" (com SafeFileQueryEscape)`
  );
  assert.ok(
    templ.includes('hx-swap="innerHTML"'),
    'o swap precisa ser innerHTML: com outerHTML o container deixa de escutar o evento'
  );
});

test('editor-init.js dispara o evento depois de salvar', () => {
  const js = ler(path.join(SRC_DIR, 'editor-init.js'));

  assert.ok(
    js.includes(`new Event("${EVENTO}")`),
    `editor-init.js precisa disparar "${EVENTO}" ao salvar a nota`
  );
  // O evento é disparado no mesmo bloco do todos-updated (padrão do §6.15).
  assert.ok(
    js.includes('new Event("todos-updated")'),
    'o dispatch do todos-updated é a referência do padrão — não remova'
  );
});

test('o bundle servido contém o evento (estático rebuildado)', () => {
  const estatico = ler(path.join(STATIC_DIR, 'editor-init.js'));
  assert.ok(
    estatico.includes(EVENTO),
    `web/static/editor-init.js está desatualizado: rode "node build.js" (o evento ${EVENTO} não está lá)`
  );
});

test('a rota /api/query está registrada em routes.go', () => {
  const rotas = ler(ROUTES);
  assert.ok(
    rotas.includes(`Get("${ROTA}", notesCtx.HandleQueryPanel)`),
    `routes.go precisa registrar GET ${ROTA} → HandleQueryPanel`
  );
});

test('o painel usa o mesmo alvo de swap nos dois lados (container × botão)', () => {
  const templ = ler(path.join(FEATURES_DIR, 'query_block.templ'));
  assert.ok(
    templ.includes(`hx-target="${ALVO}"`),
    `o botão "atualizar" precisa mirar ${ALVO}`
  );
  assert.match(templ, /templ QueryPanel\(/, 'QueryPanel é o fragmento do painel');
});
