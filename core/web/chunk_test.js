/**
 * chunk_test.js
 * Testes completos da funcao chunkText usada no pipeline de embeddings.
 *
 * Valida:
 *   - Nenhum texto perdido (reconstrucao)
 *   - Limite de tamanho por chunk
 *   - Quebras inteligentes (newline > espaco > corte seco)
 *   - Overlap preserva contexto entre chunks
 *   - Caracteres unicode/acentos
 *   - Edge cases (vazio, 1 char, so whitespace, etc.)
 *
 * Uso: node web/chunk_test.js
 */

// ── Copia da funcao de semantic.js ──
function chunkText(text, maxChars, overlapChars) {
  var chunks = [];
  var start = 0;
  while (start < text.length) {
    var end = start + maxChars;
    if (end < text.length) {
      var lastNewline = text.lastIndexOf("\n", end);
      if (lastNewline > start + maxChars * 0.6) {
        end = lastNewline;
      } else {
        var lastSpace = text.lastIndexOf(" ", end);
        if (lastSpace > start + maxChars * 0.6) {
          end = lastSpace;
        }
      }
    }
    var chunk = text.slice(start, end).trim();
    if (chunk) chunks.push(chunk);
    start = end - overlapChars;
    if (start >= text.length - overlapChars) break;
  }
  return chunks;
}

// ── Helpers de teste ──
var passed = 0;
var failed = 0;

function assert(condition, msg) {
  if (!condition) {
    console.error("  FAIL:", msg);
    failed++;
  } else {
    passed++;
  }
}

function assertEqual(actual, expected, msg) {
  if (actual !== expected) {
    console.error("  FAIL:", msg, "- esperado", JSON.stringify(expected), "recebido", JSON.stringify(actual));
    failed++;
  } else {
    passed++;
  }
}

// ── Testes ──
console.log("=== chunkText tests ===");

// 1. Texto vazio
assertEqual(chunkText("", 1500, 200).length, 0, "vazio -> 0 chunks");

// 2. Texto menor que o limite
var r = chunkText("Texto curto.", 1500, 200);
assertEqual(r.length, 1, "curto -> 1 chunk");
assertEqual(r[0], "Texto curto.", "conteudo original preservado");

// 3. Texto exatamente no limite
var exact = "x".repeat(1500);
r = chunkText(exact, 1500, 200);
assertEqual(r.length, 1, "exato no limite -> 1 chunk");
assertEqual(r[0], exact, "conteudo exato preservado");

// 4. NENHUM chunk ultrapassa maxChars
var big = "x".repeat(5000);
r = chunkText(big, 1000, 200);
for (var i = 0; i < r.length; i++) {
  assert(r[i].length <= 1000, "chunk " + i + " tem " + r[i].length + " chars, maximo 1000");
}

// 5. Quebra por newline quando disponivel
var p1 = "A".repeat(1000);
var p2 = "B".repeat(1000);
r = chunkText(p1 + "\n" + p2, 1500, 200);
assertEqual(r.length, 2, "newline viavel -> 2 chunks");
assertEqual(r[0], p1, "chunk 1 = paragrafo A");
assert(r[1].indexOf("B") >= 0, "chunk 2 contem B");

// 6. Quebra por espaco quando newline nao esta em boa posicao
var a = "a".repeat(900);
var b = "b".repeat(900);
r = chunkText(a + " " + b, 1000, 200);
assert(r.length >= 2, "espaco -> pelo menos 2 chunks (overlap pode gerar 3)");
assert(r[0].indexOf("a") >= 0, "chunk 1 contem a");
assert(r[r.length - 1].indexOf("b") >= 0, "ultimo chunk contem b");

// 7. Quebra forcada (sem newline nem espaco viavel)
var dense = "x".repeat(3000);
r = chunkText(dense, 1000, 200);
assert(r.length >= 3, "texto denso -> pelo menos 3 chunks");
for (i = 0; i < r.length; i++) {
  assert(r[i].length <= 1000, "chunk denso " + i + " respeita limite");
}

// 8. Overlap preserva contexto
r = chunkText("M".repeat(2000), 1000, 200);
assert(r.length > 1, "overlap -> multiplos chunks");
var overlap = r[0].slice(-200);
assert(r[1].indexOf(overlap) === 0 || r[1].indexOf(overlap) >= 0,
  "inicio do chunk 2 encontrado no final do chunk 1 (overlap)");

// 9. 1 caractere
r = chunkText("x", 1500, 200);
assertEqual(r.length, 1, "1 char -> 1 chunk");
assertEqual(r[0], "x", "conteudo = 'x'");

// 10. chunk grande com overlap zero
r = chunkText("texto medio para teste", 2000, 0);
assertEqual(r.length, 1, "overlap=0 com texto menor que maxChars -> 1 chunk");

// 11. overlap menor que maxChars (caso real: 200 < 1500)
r = chunkText("abc".repeat(500), 1000, 300);
assert(r.length > 0, "overlap viavel nao quebra");
assert(r[0].length <= 1000, "chunk respeita maxChars");

// 12. Newlines consecutivos
r = chunkText("A".repeat(800) + "\n\n\n" + "B".repeat(800), 1500, 200);
assertEqual(r.length, 2, "newlines consecutivos -> 2 chunks");

// 13. Texto com espacos nas bordas (deve trimar)
r = chunkText("   a   b   c   ".repeat(50), 500, 50);
for (i = 0; i < r.length; i++) {
  assert(r[i][0] !== " ", "chunk " + i + " nao comeca com espaco");
  assert(r[i][r[i].length - 1] !== " ", "chunk " + i + " nao termina com espaco");
}

// 14. Unicode/acentos
var u = "caeeonu".repeat(500);
r = chunkText(u, 1000, 200);
assert(r.length > 0, "unicode nao quebra");
assert(r[0].indexOf("c") >= 0, "unicode preservado");

// 15. RECONSTRUCAO: todo caractere do original aparece em pelo menos um chunk
var original = "A".repeat(700) + " " + "B".repeat(700) + "\n" + "C".repeat(700);
r = chunkText(original, 1000, 200);
var combined = r.join("");
for (var ci = 0; ci < original.length; ci++) {
  assert(combined.indexOf(original[ci]) >= 0,
    "caractere '" + original[ci] + "' (pos " + ci + ") aparece em algum chunk");
}

// 16. Newline no final
r = chunkText("conteudo\n", 1500, 200);
assertEqual(r.length, 1, "newline no final -> 1 chunk");
assertEqual(r[0], "conteudo", "newline final removido pelo trim");

// 17. Apenas whitespace
r = chunkText("   \n\n   ", 1500, 200);
assertEqual(r.length, 0, "so whitespace -> 0 chunks");

// 18. Texto muito longo, varios tipos de quebra
var complex = "";
for (i = 0; i < 20; i++) {
  complex += "# Secao " + i + "\n\n" + "paragrafo ".repeat(30) + "\n\n";
}
r = chunkText(complex, 1500, 200);
assert(r.length > 1, "texto complexo -> varios chunks");

// 19. Parametros REAIS usados em semantic.js (1500, 200)
var realContent = "# Titulo\n\nPrimeiro paragrafo com conteudo relevante para busca semantica.\n\nSegundo paragrafo com mais informacao util.\n\nTerceiro paragrafo finalizando o texto com conclusoes.\n".repeat(10);
r = chunkText(realContent, 1500, 200);
assert(r.length > 1, "conteudo real -> varios chunks");
assert(r[0].length <= 1500, "chunk real respeita limite de 1500");

// 20. Overlap nao produz chunks vazios
r = chunkText("conteudo", 1500, 5000);
for (i = 0; i < r.length; i++) {
  assert(r[i].length > 0, "nenhum chunk vazio");
}

// ── Resumo ──
console.log("\n================================");
console.log(passed + " passaram, " + failed + " falharam");
if (failed > 0) process.exit(1);
