const test = require('node:test');
const assert = require('node:assert');
const path = require('path');
const { pathToFileURL } = require('url');

// calc-engine.js é ES module (package.json type: module) — importado
// dinamicamente, mesmo padrão de editor-slash.unit.cjs.
const modulePath = pathToFileURL(
  path.join(__dirname, '..', 'src', 'calc-engine.js')
).href;

async function engine() {
  return await import(modulePath);
}

function scope() {
  return Object.create(null);
}

test('aritmética básica e precedência', async () => {
  const { evaluateLine } = await engine();
  const s = scope();
  assert.deepStrictEqual(evaluateLine('2 + 3 * 4', s), { value: 14, name: null });
  assert.deepStrictEqual(evaluateLine('(2 + 3) * 4', s), { value: 20, name: null });
  assert.deepStrictEqual(evaluateLine('10 % 4', s), { value: 2, name: null });
  assert.deepStrictEqual(evaluateLine('7 / 2', s), { value: 3.5, name: null });
});

test('^ é associativa à direita e o unário fica abaixo dela', async () => {
  const { evaluateLine } = await engine();
  const s = scope();
  assert.deepStrictEqual(evaluateLine('2 ^ 3 ^ 2', s), { value: 512, name: null });
  assert.deepStrictEqual(evaluateLine('-2 ^ 2', s), { value: -4, name: null });
  assert.deepStrictEqual(evaluateLine('- -3', s), { value: 3, name: null });
  assert.deepStrictEqual(evaluateLine('2 * -3', s), { value: -6, name: null });
});

test('atribuições alimentam as linhas seguintes (exemplo do usuário)', async () => {
  const { evaluateBlock } = await engine();
  const results = evaluateBlock(['var1 = 10', 'var2 = 20', 'var1 + var2']);
  assert.deepStrictEqual(results, [
    { value: 10, name: 'var1' },
    { value: 20, name: 'var2' },
    { value: 30, name: null },
  ]);
});

test('redefinir variável vale para quem usa depois', async () => {
  const { evaluateBlock } = await engine();
  const results = evaluateBlock(['x = 2', 'x * 3', 'x = 5', 'x * 3']);
  assert.deepStrictEqual(results.map((r) => (r ? r.value : null)), [
    2, 6, 5, 15,
  ]);
});

test('erro de variável não definida vem em pt-BR', async () => {
  const { evaluateLine } = await engine();
  const s = scope();
  evaluateLine('var1 = 1', s);
  const r = evaluateLine('var1 + var3', s);
  assert.ok(r.error);
  assert.match(r.error, /não foi definida/);
  assert.match(r.error, /var3/);
});

test('divisão por zero vira erro, não Infinity', async () => {
  const { evaluateLine } = await engine();
  const r = evaluateLine('1 / 0', scope());
  assert.ok(r.error);
  assert.match(r.error, /divisão por zero/);
});

test('linha sem operador e símbolo inválido apontam a posição', async () => {
  const { evaluateLine } = await engine();
  const r = evaluateLine('2 3', scope());
  assert.ok(r.error);
  assert.match(r.error, /falta um operador/);

  const r2 = evaluateLine('2 & 3', scope());
  assert.ok(r2.error);
  assert.match(r2.error, /caractere não permitido/);
  assert.match(r2.error, /posição 3/);
});

test('funções, constantes e notação científica', async () => {
  const { evaluateLine } = await engine();
  const s = scope();
  assert.deepStrictEqual(evaluateLine('sqrt(9) + abs(-2)', s), { value: 5, name: null });
  assert.deepStrictEqual(evaluateLine('max(2, 7)', s), { value: 7, name: null });
  assert.deepStrictEqual(evaluateLine('round(2,6)', s), { value: 3, name: null });
  assert.ok(Math.abs(evaluateLine('sin(0)', s).value) < 1e-12);
  assert.ok(Math.abs(evaluateLine('pi', s).value - Math.PI) < 1e-12);
  assert.deepStrictEqual(evaluateLine('2e3', s), { value: 2000, name: null });
  assert.deepStrictEqual(evaluateLine('1e-3 + 1', s).value, 1.001);
});

test('função desconhecida lista as disponíveis', async () => {
  const { evaluateLine } = await engine();
  const r = evaluateLine('foo(1)', scope());
  assert.ok(r.error);
  assert.match(r.error, /função desconhecida "foo"/);
  assert.match(r.error, /sqrt/);
});

test('med() é a média aritmética com quantos valores quiser', async () => {
  const { evaluateLine, evaluateBlock, formatResult } = await engine();

  // quantidade variável de argumentos (1, 3, 10, 64)
  assert.deepStrictEqual(evaluateLine('med(10)', scope()), { value: 10, name: null });
  assert.deepStrictEqual(evaluateLine('med(10, 20, 30)', scope()), { value: 20, name: null });
  assert.deepStrictEqual(
    evaluateLine('med(1, 2, 3, 4, 5, 6, 7, 8, 9, 10)', scope()),
    { value: 5.5, name: null },
  );
  const maxArgs = 'med(' + Array.from({ length: 64 }, () => '2').join(', ') + ')';
  assert.deepStrictEqual(evaluateLine(maxArgs, scope()), { value: 2, name: null });

  // funciona com expressões, variáveis e composição com outras funções
  assert.deepStrictEqual(evaluateLine('med(2 * 3, sqrt(16), 10)', scope()).value, 20 / 3);
  const results = evaluateBlock(['n1 = 8', 'n2 = 6', 'n3 = 9', 'med(n1, n2, n3)']);
  assert.strictEqual(results[3].value, 23 / 3);
  assert.strictEqual(formatResult(results[3].value), '7,666666666667');
  assert.deepStrictEqual(evaluateLine('round(med(1, 2))', scope()), { value: 2, name: null });
  assert.deepStrictEqual(evaluateLine('med(1, 2, 3, 4) * 10', scope()), { value: 25, name: null });

  // vírgula decimal funciona dentro dos argumentos
  assert.deepStrictEqual(evaluateLine('med(12,5, 7,5)', scope()), { value: 10, name: null });
});

test('med() sem valores é erro em pt-BR; aridade fixa continua valendo', async () => {
  const { evaluateLine } = await engine();

  const empty = evaluateLine('med()', scope());
  assert.ok(empty.error);
  assert.match(empty.error, /"med" espera ao menos 1 valor, recebeu 0/);

  // teto por chamada continua existindo (proteção de parsing)
  const tooMany = 'med(' + Array.from({ length: 80 }, (_, i) => i + 1).join(', ') + ')';
  const r = evaluateLine(tooMany, scope());
  assert.ok(r.error);
  assert.match(r.error, /recebe muitos valores/);

  // funções de aridade fixa não ganharam argumentos extras
  const sqrt2 = evaluateLine('sqrt(9, 2)', scope());
  assert.ok(sqrt2.error);
  assert.match(sqrt2.error, /"sqrt" espera 1 valor, recebeu 2/);
  const min3 = evaluateLine('min(1, 2, 3)', scope());
  assert.ok(min3.error);
  assert.match(min3.error, /"min" espera 2 valores, recebeu 3/);
});

test('aceita vírgula OU ponto como decimal na entrada', async () => {
  const { evaluateLine } = await engine();
  assert.deepStrictEqual(evaluateLine('1,5 + 1', scope()).value, 2.5);
  assert.deepStrictEqual(evaluateLine('1.5 + 1', scope()).value, 2.5);
});

test('linhas vazias e comentários não geram resultado', async () => {
  const { evaluateLine, evaluateBlock } = await engine();
  assert.strictEqual(evaluateLine('', scope()), null);
  assert.strictEqual(evaluateLine('   ', scope()), null);
  assert.strictEqual(evaluateLine('  # comentário', scope()), null);
  const results = evaluateBlock(['a = 1', '# só comentário', '', 'a + 1']);
  assert.deepStrictEqual(results, [
    { value: 1, name: 'a' },
    null,
    null,
    { value: 2, name: null },
  ]);
});

test('limites: aninhamento e símbolos demais viram erro, nunca trava', async () => {
  const { evaluateLine } = await engine();
  const deep = '('.repeat(50) + '1' + ')'.repeat(50);
  const r = evaluateLine(deep, scope());
  assert.ok(r.error);
  assert.match(r.error, /aninhada/);

  const long = Array.from({ length: 300 }, (_, i) => String(i)).join(' + ');
  const r2 = evaluateLine(long, scope());
  assert.ok(r2.error);
  assert.match(r2.error, /muito longa/);
});

test('formatResult: inteiro direto, decimal com vírgula pt-BR', async () => {
  const { formatResult } = await engine();
  assert.strictEqual(formatResult(30), '30');
  assert.strictEqual(formatResult(-7), '-7');
  assert.strictEqual(formatResult(0), '0');
  assert.strictEqual(formatResult(-0), '0');
  assert.strictEqual(formatResult(3.141592653589793), '3,14159265359');
  assert.strictEqual(formatResult(1 / 3), '0,333333333333');
  // Ruído de ponto flutuante clássico não pode vazar para a tela
  assert.strictEqual(formatResult(0.1 + 0.2), '0,3');
  assert.strictEqual(formatResult(1000.5), '1000,5');
});

test('resultado nunca é NaN/Infinity sem virar erro', async () => {
  const { evaluateLine } = await engine();
  const r = evaluateLine('sqrt(-1)', scope());
  assert.ok(r.error);
  const r2 = evaluateLine('9 ^ 999', scope());
  assert.ok(r2.error);
  assert.match(r2.error, /fora do intervalo/);
});
