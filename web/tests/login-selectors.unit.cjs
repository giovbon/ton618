/**
 * login-selectors.unit.cjs — Guarda do formulário de login × specs E2E.
 *
 * Uso: node --test tests/login-selectors.unit.cjs
 *
 * Contexto: o app é de usuário único, então o formulário pede **só a senha**
 * (`#pass`) e o usuário é implícito. Durante um tempo os specs E2E continuaram
 * preenchendo `#user` — um campo que não existe — e TODOS falhavam no login,
 * antes de testar qualquer coisa (o Playwright ficava 30 s esperando o seletor).
 *
 * Esta guarda amarra as duas pontas: se o campo voltar ao formulário (ou se
 * alguém reintroduzir `#user` num spec), o teste avisa.
 */
const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const TESTS_DIR = __dirname;
const LOGIN_TEMPL = path.resolve(
  TESTS_DIR, '..', '..', 'internal', 'features', 'system', 'login.templ'
);

function ler(p) {
  return fs.readFileSync(p, 'utf8');
}

test('o formulário de login pede senha e não tem campo de usuário', () => {
  const templ = ler(LOGIN_TEMPL);

  assert.ok(templ.includes('id="pass"'), 'o campo #pass do login sumiu');
  assert.ok(templ.includes('id="login-btn"'), 'o botão #login-btn do login sumiu');
  assert.ok(
    !/id="user"/.test(templ),
    'o login passou a ter campo de usuário: atualize os specs E2E para preenchê-lo'
  );
});

test('nenhum spec E2E preenche #user', () => {
  const specs = fs.readdirSync(TESTS_DIR).filter(f => f.endsWith('.spec.ts'));
  assert.ok(specs.length > 0, 'nenhum spec encontrado');

  for (const spec of specs) {
    const src = ler(path.join(TESTS_DIR, spec));
    assert.ok(
      !src.includes("'#user'") && !src.includes('"#user"'),
      `${spec} preenche #user, que não existe no formulário (use só #pass)`
    );
  }
});

test('todo spec que abre o editor faz login antes', () => {
  const specs = fs.readdirSync(TESTS_DIR).filter(f => f.endsWith('.spec.ts'));

  for (const spec of specs) {
    const src = ler(path.join(TESTS_DIR, spec));
    if (!src.includes("goto('/editor") && !src.includes('goto(`/editor')) continue;
    assert.ok(
      src.includes("fill('#pass'"),
      `${spec} abre o editor mas não faz login (#pass) em nenhum ponto`
    );
  }
});
