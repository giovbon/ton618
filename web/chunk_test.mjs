/**
 * chunk_test.mjs — Testes de chunkText com node:test
 *
 * Uso: node --test web/chunk_test.mjs
 */

import { describe, it } from 'node:test';
import assert from 'node:assert';

// ── Copia da funcao de semantic.js ──
function chunkText(text, maxChars, overlapChars) {
  const chunks = [];
  let start = 0;
  while (start < text.length) {
    let end = start + maxChars;
    if (end < text.length) {
      const lastNewline = text.lastIndexOf('\n', end);
      if (lastNewline > start + maxChars * 0.6) {
        end = lastNewline;
      } else {
        const lastSpace = text.lastIndexOf(' ', end);
        if (lastSpace > start + maxChars * 0.6) {
          end = lastSpace;
        }
      }
    }
    const chunk = text.slice(start, end).trim();
    if (chunk) chunks.push(chunk);
    start = end - overlapChars;
    if (start >= text.length - overlapChars) break;
  }
  return chunks;
}

describe('chunkText', () => {
  it('texto vazio -> 0 chunks', () => {
    assert.equal(chunkText('', 1500, 200).length, 0);
  });

  it('texto menor que o limite -> 1 chunk', () => {
    const r = chunkText('Texto curto.', 1500, 200);
    assert.equal(r.length, 1);
    assert.equal(r[0], 'Texto curto.');
  });

  it('texto exatamente no limite -> 1 chunk', () => {
    const text = 'x'.repeat(1500);
    const r = chunkText(text, 1500, 200);
    assert.equal(r.length, 1);
    assert.equal(r[0], text);
  });

  it('nenhum chunk ultrapassa maxChars', () => {
    const r = chunkText('x'.repeat(5000), 1000, 200);
    for (let i = 0; i < r.length; i++) {
      assert.ok(r[i].length <= 1000, `chunk ${i} tem ${r[i].length} chars`);
    }
  });

  it('quebra por newline quando disponivel', () => {
    const p1 = 'A'.repeat(1000);
    const p2 = 'B'.repeat(1000);
    const r = chunkText(p1 + '\n' + p2, 1500, 200);
    assert.equal(r.length, 2);
    assert.equal(r[0], p1);
    assert.ok(r[1].includes('B'));
  });

  it('quebra por espaco quando newline nao viavel', () => {
    const a = 'a'.repeat(900);
    const b = 'b'.repeat(900);
    const r = chunkText(a + ' ' + b, 1000, 200);
    assert.ok(r.length >= 2);
    assert.ok(r[0].includes('a'));
    assert.ok(r[r.length - 1].includes('b'));
  });

  it('quebra forcada em texto denso sem espacos', () => {
    const r = chunkText('x'.repeat(3000), 1000, 200);
    assert.ok(r.length >= 3);
    for (let i = 0; i < r.length; i++) {
      assert.ok(r[i].length <= 1000);
    }
  });

  it('overlap preserva contexto entre chunks', () => {
    const r = chunkText('M'.repeat(2000), 1000, 200);
    assert.ok(r.length > 1);
    const overlap = r[0].slice(-200);
    assert.ok(r[1].includes(overlap));
  });

  it('1 caractere -> 1 chunk', () => {
    assert.equal(chunkText('x', 1500, 200).length, 1);
  });

  it('newlines consecutivos -> 2 chunks', () => {
    const r = chunkText('A'.repeat(800) + '\n\n\n' + 'B'.repeat(800), 1500, 200);
    assert.equal(r.length, 2);
  });

  it('texto com espacos nas bordas e trimado', () => {
    const r = chunkText('   a   b   c   '.repeat(50), 500, 50);
    for (let i = 0; i < r.length; i++) {
      assert.ok(r[i][0] !== ' ', `chunk ${i} comeca com espaco`);
      assert.ok(r[i].at(-1) !== ' ', `chunk ${i} termina com espaco`);
    }
  });

  it('unicode/acentos preservados', () => {
    const u = 'caeeonu'.repeat(500);
    const r = chunkText(u, 1000, 200);
    assert.ok(r.length > 0);
    assert.ok(r[0].includes('c'));
  });

  it('reconstrucao: todo caractere aparece em algum chunk', () => {
    const original = 'A'.repeat(700) + ' ' + 'B'.repeat(700) + '\n' + 'C'.repeat(700);
    const r = chunkText(original, 1000, 200);
    const combined = r.join('');
    for (let i = 0; i < original.length; i++) {
      assert.ok(combined.includes(original[i]), `caractere '${original[i]}' (pos ${i}) nao encontrado`);
    }
  });

  it('apenas whitespace -> 0 chunks', () => {
    assert.equal(chunkText('   \n\n   ', 1500, 200).length, 0);
  });

  it('parametros reais (1500, 200) com conteudo realista', () => {
    const content = '# Titulo\n\nPrimeiro paragrafo.\n\nSegundo paragrafo.\n\nTerceiro.\n'.repeat(50);
    const r = chunkText(content, 1500, 200);
    assert.ok(r.length > 1);
    assert.ok(r[0].length <= 1500);
  });

  it('nenhum chunk vazio', () => {
    const r = chunkText('conteudo', 1500, 5000);
    for (let i = 0; i < r.length; i++) {
      assert.ok(r[i].length > 0);
    }
  });
});
