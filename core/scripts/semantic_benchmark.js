import { pipeline, env } from "@huggingface/transformers";
import path from "path";
import { fileURLToPath } from "url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// Configuração do ambiente para carregar o modelo localmente da pasta web/static/models
const localModelDir = path.resolve(__dirname, "../web/static/models");
console.log(`📂 Utilizando diretório de modelos locais: ${localModelDir}`);

env.allowLocalModels = true;
env.allowRemoteModels = false;
env.localModelPath = localModelDir;

const MODEL_NAME = "Xenova/paraphrase-multilingual-MiniLM-L12-v2";

// ── Dataset de Teste (Golden Dataset) ──
const dataset = [
  // Base
  { id: 1, category: "technology", text: "O aprendizado de máquina e a inteligência artificial estão revolucionando o desenvolvimento de software e a análise de dados." },
  { id: 2, category: "nature", text: "A preservação da Amazônia e de suas florestas tropicais é vital para combater as mudanças climáticas globais." },
  { id: 3, category: "food", text: "Uma receita tradicional de bolo de chocolate fofinho leva ovos, farinha, cacau em pó e açúcar refinado." },
  { id: 4, category: "technology", text: "Novas arquiteturas de computadores quânticos prometem resolver problemas matemáticos complexos em segundos." },
  { id: 5, category: "nature", text: "Os recifes de corais abrigam uma biodiversidade marinha imensa e são altamente sensíveis ao aquecimento da água do mar." },
  { id: 6, category: "food", text: "O preparo culinário do macarrão italiano artesanal exige uma boa massa de sêmola e molho de tomates frescos." },
  // Curtos e Longos
  { id: 7, category: "nature", text: "Gato dormindo." },
  { id: 8, category: "finance", text: "Investimentos em bolsa de valores, renda fixa, tesouro direto e criptomoedas são fundamentais para o sucesso financeiro a longo prazo, considerando a inflação e taxas de juros do mercado econômico globalizado." },
  // Multilingue
  { id: 9, category: "finance", text: "The stock market crashed today causing panic among investors." },
  { id: 10, category: "science", text: "Quantum mechanics is a fundamental theory in physics that provides a description of the physical properties of nature." },
  // Ambíguos
  { id: 11, category: "mixed_tech_food", text: "O desenvolvedor comeu uma maçã enquanto programava o algoritmo de inteligência artificial." },
  { id: 12, category: "science", text: "O buraco negro supermassivo no centro da Via Láctea devora estrelas vizinhas." }
];

// ── Consultas de Teste (Queries) ──
const queries = [
  { text: "desenvolvimento de algoritmos e computação quântica", expectedCategory: "technology" },
  { text: "preservação da fauna, flora e ecossistemas marinhos", expectedCategory: "nature" },
  { text: "ingredientes culinários e receitas para cozinhar", expectedCategory: "food" },
  { text: "animal de estimação felino", expectedCategory: "nature" }, // Testa "Gato dormindo"
  { text: "mercado financeiro e dinheiro", expectedCategory: "finance" },
  { text: "financial market and investments", expectedCategory: "finance" },
  { text: "black holes and galaxies", expectedCategory: "science" },
  { text: "física quântica e universo", expectedCategory: "science" },
  { text: "dev trabalhando e comendo", expectedCategory: "mixed_tech_food" },
  { text: "investimentos", expectedCategory: "finance" }
];

// Função simples para calcular o produto escalar (dot product)
// Como o pipeline do transformers gera embeddings normalizados (L2), o produto escalar é igual à similaridade de cosseno.
function cosineSimilarity(a, b) {
  let sum = 0;
  for (let i = 0; i < a.length; i++) {
    sum += a[i] * b[i];
  }
  return sum;
}

async function runBenchmark() {
  console.log("🔄 Inicializando pipeline de embeddings...");
  
  const pipe = await pipeline("feature-extraction", MODEL_NAME, {
    dtype: "q8",
    device: "cpu"
  });
  
  console.log("✅ Pipeline carregada com sucesso.");
  
  // 1. Gerar embeddings para todos os documentos do dataset
  console.log("\n📦 Gerando embeddings dos documentos do Golden Dataset...");
  const docEmbeddings = [];
  for (const doc of dataset) {
    const output = await pipe(doc.text, { pooling: "mean", normalize: true });
    docEmbeddings.push({
      ...doc,
      embedding: Array.from(output.data)
    });
  }
  console.log(`✅ ${docEmbeddings.length} documentos indexados.`);

  // 2. Executar as consultas e avaliar os resultados
  console.log("\n🔎 Executando busca semântica para avaliação...");
  let totalQueries = queries.length;
  let correctHits = 0;

  for (const q of queries) {
    console.log(`\n--------------------------------------------------`);
    console.log(`❓ Query: "${q.text}"`);
    console.log(`🎯 Categoria Esperada: ${q.expectedCategory}`);

    const qOutput = await pipe(q.text, { pooling: "mean", normalize: true });
    const qEmbedding = Array.from(qOutput.data);

    // Calcular similaridade com todos os documentos
    const scores = docEmbeddings.map(doc => ({
      id: doc.id,
      category: doc.category,
      text: doc.text,
      similarity: cosineSimilarity(qEmbedding, doc.embedding)
    }));

    // Ordenar por similaridade decrescente
    scores.sort((a, b) => b.similarity - a.similarity);

    // Print top 3 resultados
    console.log("🏆 Top 3 Resultados obtidos:");
    scores.slice(0, 3).forEach((res, index) => {
      console.log(`   ${index + 1}. [Sim: ${res.similarity.toFixed(4)}] [Cat: ${res.category}] "${res.text.substring(0, 70)}..."`);
    });

    // Validar se o melhor resultado corresponde à categoria esperada
    const topResult = scores[0];
    if (topResult.category === q.expectedCategory) {
      console.log("🟢 ACERTO! O resultado mais relevante pertence à categoria correta.");
      correctHits++;
    } else {
      console.log("🔴 ERRO! O resultado mais relevante pertence a uma categoria inesperada.");
    }
  }

  // 3. Exibir Score Final
  const finalScore = (correctHits / totalQueries) * 100;
  console.log(`\n==================================================`);
  console.log(`📊 RESULTADO DO BENCHMARK DE BUSCA SEMÂNTICA`);
  console.log(`✅ Acertos: ${correctHits}/${totalQueries}`);
  console.log(`📈 Precisão (Accuracy): ${finalScore.toFixed(2)}%`);
  console.log(`==================================================`);
}

runBenchmark().catch(err => {
  console.error("❌ Falha crítica ao rodar o benchmark:", err);
});
