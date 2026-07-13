// Test file

// Copia local da função chunkText de semantic.js para testar isoladamente
function chunkText(text, maxChars, overlapChars) {
  var chunks = [];
  var start = 0;
  while (start < text.length) {
    var end = start + maxChars;
    if (end < text.length) {
      // Tenta quebrar no limite de um parágrafo ou frase
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

function assert(condition, message) {
    if (!condition) {
        throw new Error("Assertion failed: " + message);
    }
}

console.log("Running chunk_test.js...");

// Teste 1: Texto menor que o limite
let shortText = "Este é um texto curto.";
let chunks1 = chunkText(shortText, 1500, 200);
assert(chunks1.length === 1, "Deveria ter 1 chunk");
assert(chunks1[0] === shortText, "Conteudo incorreto");

// Teste 2: Texto grande sendo dividido
let longText = "a".repeat(1500) + " " + "b".repeat(1500);
let chunks2 = chunkText(longText, 1500, 200);
assert(chunks2.length === 3, "Deveria ter 3 chunks devido ao overlap");
assert(chunks2[0].startsWith("a"), "Chunk 1 comeca com a");
assert(chunks2[1].includes("a") && chunks2[1].includes("b"), "Chunk 2 (overlap) contem a e b");

// Teste 3: Respeita o parágrafo (newline > 60%)
let p1 = "A".repeat(1000);
let p2 = "B".repeat(1000);
let textWithNewlines = p1 + "\n" + p2;
let chunks3 = chunkText(textWithNewlines, 1500, 200);
assert(chunks3.length === 2, "Deveria quebrar no newline e fazer 2 chunks");
assert(chunks3[0] === p1, "O primeiro chunk quebrou no newline perfeitamente");
assert(chunks3[1].includes("B"), "O segundo chunk contem p2 e o overlap");

console.log("All chunkText tests passed!");
