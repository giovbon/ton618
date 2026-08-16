package search

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"ton618/core/internal/core/db"
	"ton618/core/internal/features/notes"
)

// makeEmbedding cria um vetor 384-d determinístico; os primeiros valores podem
// ser passados para controlar a proximidade semântica nos testes.
func makeEmbedding(nonZero ...float32) []float32 {
	emb := make([]float32, db.EmbeddingDim)
	for i, v := range nonZero {
		if i >= len(emb) {
			break
		}
		emb[i] = v
	}
	return emb
}

// indexNoteFTS salva a nota pelo NoteService (indexa FTS5/documentos).
func indexNoteFTS(t *testing.T, ctx *HandlerContext, filename, content string) {
	t.Helper()
	svc := notes.NewNoteService(ctx.Store, ctx.Store, ctx.Store, ctx.Store, ctx.Store, ctx.Store, ctx.Cfg.DocsDir)
	if err := svc.Save(filename, content, nil); err != nil {
		t.Fatalf("notes save %s: %v", filename, err)
	}
}

// indexNoteSemantic grava um chunk com embedding (indexa a busca semântica).
func indexNoteSemantic(t *testing.T, ctx *HandlerContext, filename string, emb []float32) {
	t.Helper()
	chunk := db.ChunkInfo{
		ChunkID:    filename + "#0",
		Filename:   filename,
		ChunkIndex: 0,
		Content:    "conteudo",
		Embedding:  emb,
	}
	if err := ctx.Store.SaveNoteChunks(filename, []db.ChunkInfo{chunk}); err != nil {
		t.Fatalf("SaveNoteChunks %s: %v", filename, err)
	}
}

func TestHandleHybridSearch_FusesBothEngines(t *testing.T) {
	ctx := newTestContext(t)

	// python.md: casa no FTS ("python") E na semântica (embedding == query).
	indexNoteFTS(t, ctx, "notes/python.md", "# Python\nPython é ótimo para análise de dados.")
	indexNoteSemantic(t, ctx, "notes/python.md", makeEmbedding(1.0))

	// semantica.md: NÃO casa no FTS (não contém "python"), mas casa na semântica.
	indexNoteFTS(t, ctx, "notes/semantica.md", "# Música\nSobre música clássica e orquestras.")
	indexNoteSemantic(t, ctx, "notes/semantica.md", makeEmbedding(0.9, 0.1))

	body, _ := json.Marshal(map[string]interface{}{
		"query":     "python",
		"embedding": makeEmbedding(1.0),
		"limit":     10,
	})
	req := httptest.NewRequest("POST", "/api/search/hybrid", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	ctx.HandleHybridSearch(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Results []hybridSearchResult `json:"results"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Results) < 2 {
		t.Fatalf("esperava >= 2 resultados (um de cada motor), got %d", len(resp.Results))
	}

	// python.md deve vir primeiro (aparece nos DOIS motores → soma RRF).
	if resp.Results[0].Filename != "notes/python.md" {
		t.Errorf("esperava notes/python.md primeiro, got %q", resp.Results[0].Filename)
	}

	byName := map[string]hybridSearchResult{}
	for _, r := range resp.Results {
		byName[r.Filename] = r
	}

	python := byName["notes/python.md"]
	if python.RankFTS == nil || *python.RankFTS != 1 {
		t.Errorf("python.md: esperava rank_fts=1, got %v", python.RankFTS)
	}
	if python.RankSem == nil || *python.RankSem != 1 {
		t.Errorf("python.md: esperava rank_sem=1, got %v", python.RankSem)
	}
	if !python.HasHighlight {
		t.Errorf("python.md: esperava snippet com highlight (casou no FTS)")
	}

	sem := byName["notes/semantica.md"]
	if sem.RankFTS != nil {
		t.Errorf("semantica.md: não deveria ter rank_fts")
	}
	if sem.RankSem == nil || *sem.RankSem != 2 {
		t.Errorf("semantica.md: esperava rank_sem=2, got %v", sem.RankSem)
	}
	if sem.HasHighlight {
		t.Errorf("semantica.md: não deveria ter highlight (veio só da semântica)")
	}
	if sem.Snippet == "" {
		t.Errorf("semantica.md: snippet (prévia genérica) não deveria estar vazio")
	}
	if sem.SemSimilarity == nil || *sem.SemSimilarity < 50 {
		t.Errorf("semantica.md: esperava sem_similarity alto, got %v", sem.SemSimilarity)
	}
}

func TestHandleHybridSearch_DegradesToFTSWithoutEmbedding(t *testing.T) {
	ctx := newTestContext(t)

	indexNoteFTS(t, ctx, "notes/python.md", "# Python\nPython é ótimo.")
	indexNoteFTS(t, ctx, "notes/semantica.md", "# Música\nMúsica clássica e orquestras.")
	indexNoteSemantic(t, ctx, "notes/semantica.md", makeEmbedding(0.9, 0.1))

	// Embedding zerado → degrada para FTS5 puro (semantica.md não deve aparecer).
	body, _ := json.Marshal(map[string]interface{}{
		"query":     "python",
		"embedding": makeEmbedding(),
		"limit":     10,
	})
	req := httptest.NewRequest("POST", "/api/search/hybrid", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	ctx.HandleHybridSearch(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Results []hybridSearchResult `json:"results"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	for _, r := range resp.Results {
		if r.Filename == "notes/semantica.md" {
			t.Errorf("semantica.md não deveria aparecer sem embedding (FTS puro)")
			break
		}
	}
}

// ── Gate de evidência (Fase 1) ──

// Nota que casa no FTS SÓ via hashtag/tag e que a semântica rejeita não deve
// participar da fusão (match só-em-tag é evidência fraca).
func TestHybrid_GateFiltraMatchSoEmHashtag(t *testing.T) {
	ctx := newTestContext(t)

	// Admin: termo só como hashtag (rótulo) — conteúdo sem menção real.
	indexNoteFTS(t, ctx, "notes/admin.md", "# Prova\nAvaliacao de dados marcada para sexta. #programacao")
	// Real: termo no conteúdo.
	indexNoteFTS(t, ctx, "notes/real.md", "# Python\nProgramacao orientada a objetos e boas praticas.")
	// Semântica rejeita ambas (embedding oposto ao da query).
	indexNoteSemantic(t, ctx, "notes/admin.md", makeEmbedding(-1.0))
	indexNoteSemantic(t, ctx, "notes/real.md", makeEmbedding(-1.0))

	body, _ := json.Marshal(map[string]interface{}{
		"query":     "programacao",
		"embedding": makeEmbedding(1.0),
		"limit":     10,
	})
	req := httptest.NewRequest("POST", "/api/search/hybrid", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	ctx.HandleHybridSearch(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Results []hybridSearchResult `json:"results"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	for _, r := range resp.Results {
		if r.Filename == "notes/admin.md" {
			t.Errorf("admin.md não deveria aparecer: casou só via hashtag e a semântica rejeitou")
			break
		}
	}
	foundReal := false
	for _, r := range resp.Results {
		if r.Filename == "notes/real.md" {
			foundReal = true
			break
		}
	}
	if !foundReal {
		t.Errorf("real.md deveria aparecer (match real no conteúdo)")
	}
}

// Nota tagada (frontmatter) com a palavra fora do conteúdo: o gate mantém o
// match só-em-tag quando a semântica também aceita a nota (ganha os dois ranks).
func TestHybrid_GateMantemTagSoComAvalSemantico(t *testing.T) {
	ctx := newTestContext(t)

	indexNoteFTS(t, ctx, "notes/tagada.md", "---\ntags: [programacao]\n---\n# Aula\nConteudo sobre compiladores e parsers.")
	indexNoteSemantic(t, ctx, "notes/tagada.md", makeEmbedding(1.0)) // semântica aceita

	body, _ := json.Marshal(map[string]interface{}{
		"query":     "programacao",
		"embedding": makeEmbedding(1.0),
		"limit":     10,
	})
	req := httptest.NewRequest("POST", "/api/search/hybrid", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	ctx.HandleHybridSearch(rr, req)

	var resp struct {
		Results []hybridSearchResult `json:"results"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	var found bool
	var rankFTS, rankSem *int
	for _, r := range resp.Results {
		if r.Filename == "notes/tagada.md" {
			found = true
			rankFTS = r.RankFTS
			rankSem = r.RankSem
			break
		}
	}
	if !found {
		t.Fatal("tagada.md deveria aparecer: semântica aprovou o match só-em-tag")
	}
	if rankFTS == nil || *rankFTS != 1 {
		t.Errorf("tagada.md: esperava rank_fts=1, got %v", rankFTS)
	}
	if rankSem == nil || *rankSem != 1 {
		t.Errorf("tagada.md: esperava rank_sem=1, got %v", rankSem)
	}
}

// Query com filtro explícito de tag (tags:xxx) pula o gate: o match por tag é
// intencional, mesmo que a semântica rejeite a nota.
func TestHybrid_GatePuladoComFiltroExplicito(t *testing.T) {
	ctx := newTestContext(t)

	indexNoteFTS(t, ctx, "notes/tagada.md", "---\ntags: [programacao]\n---\n# Aula\nConteudo sobre compiladores.")
	indexNoteSemantic(t, ctx, "notes/tagada.md", makeEmbedding(-1.0)) // semântica rejeita

	body, _ := json.Marshal(map[string]interface{}{
		"query":     "tags:programacao",
		"embedding": makeEmbedding(1.0),
		"limit":     10,
	})
	req := httptest.NewRequest("POST", "/api/search/hybrid", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	ctx.HandleHybridSearch(rr, req)

	var resp struct {
		Results []hybridSearchResult `json:"results"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	for _, r := range resp.Results {
		if r.Filename == "notes/tagada.md" {
			if r.RankFTS == nil {
				t.Errorf("tagada.md: filtro explícito deveria manter o rank FTS")
			}
			return // ok
		}
	}
	t.Errorf("tagada.md deveria aparecer: filtro explícito de tag pula o gate")
}

// Sem IA (embedding zerado) o gate não se aplica — degrada para FTS puro com o
// comportamento atual (match por tag continua valendo).
func TestHybrid_GatePuladoNoModoDegradado(t *testing.T) {
	ctx := newTestContext(t)

	indexNoteFTS(t, ctx, "notes/tagada.md", "---\ntags: [programacao]\n---\n# Aula\nConteudo sobre compiladores.")

	body, _ := json.Marshal(map[string]interface{}{
		"query":     "programacao",
		"embedding": makeEmbedding(), // zerado → degrada para FTS puro
		"limit":     10,
	})
	req := httptest.NewRequest("POST", "/api/search/hybrid", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	ctx.HandleHybridSearch(rr, req)

	var resp struct {
		Results []hybridSearchResult `json:"results"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	for _, r := range resp.Results {
		if r.Filename == "notes/tagada.md" {
			return // ok — sem IA não há gate
		}
	}
	t.Errorf("tagada.md deveria aparecer no modo degradado (sem gate)")
}

// Nota marcada para exclusão (tag "deletar") não participa da fusão, mesmo
// casando no FTS.
func TestHybrid_ExcluiNotaMarcadaParaDeletar(t *testing.T) {
	ctx := newTestContext(t)

	indexNoteFTS(t, ctx, "notes/para-deletar.md", "# Lixo\nProgramacao aqui. #deletar")
	indexNoteFTS(t, ctx, "notes/ok.md", "# Ok\nProgramacao orientada a objetos.")

	body, _ := json.Marshal(map[string]interface{}{
		"query":     "programacao",
		"embedding": makeEmbedding(1.0),
		"limit":     10,
	})
	req := httptest.NewRequest("POST", "/api/search/hybrid", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	ctx.HandleHybridSearch(rr, req)

	var resp struct {
		Results []hybridSearchResult `json:"results"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	for _, r := range resp.Results {
		if r.Filename == "notes/para-deletar.md" {
			t.Errorf("para-deletar.md não deveria aparecer (tag deletar)")
			break
		}
	}
	foundOk := false
	for _, r := range resp.Results {
		if r.Filename == "notes/ok.md" {
			foundOk = true
			break
		}
	}
	if !foundOk {
		t.Errorf("ok.md deveria aparecer")
	}
}

// ── Consenso de chunks (voto majoritário) ──

// indexNoteSemanticMulti grava vários chunks semânticos de uma nota.
func indexNoteSemanticMulti(t *testing.T, ctx *HandlerContext, filename string, embs ...[]float32) {
	t.Helper()
	chunks := make([]db.ChunkInfo, 0, len(embs))
	for i, emb := range embs {
		chunks = append(chunks, db.ChunkInfo{
			ChunkID:    filename + "#" + strconv.Itoa(i),
			Filename:   filename,
			ChunkIndex: i,
			Content:    "conteudo",
			Embedding:  emb,
		})
	}
	if err := ctx.Store.SaveNoteChunks(filename, chunks); err != nil {
		t.Fatalf("SaveNoteChunks %s: %v", filename, err)
	}
}

// Nota longa (3 chunks) com apenas 1 chunk perto da query (sim ~75%, abaixo dos
// 82% excepcionais) e sem match no FTS: o voto majoritário a rejeita.
func TestHybrid_ConsensoRejeitaChunkUnicoDeNotaLonga(t *testing.T) {
	ctx := newTestContext(t)

	// Só-semântica: o conteúdo NÃO tem o termo (não âncora no FTS).
	indexNoteFTS(t, ctx, "notes/longa.md", "# Arquitetura\nSobre clean architecture e microsservicos.")
	indexNoteSemanticMulti(t, ctx, "notes/longa.md", makeEmbedding(0.3), makeEmbedding(-1.0), makeEmbedding(-1.0))

	// Controle: nota curta aceita pela semântica.
	indexNoteFTS(t, ctx, "notes/ok.md", "# Python\nProgramacao orientada a objetos.")
	indexNoteSemantic(t, ctx, "notes/ok.md", makeEmbedding(1.0))

	body, _ := json.Marshal(map[string]interface{}{
		"query":     "programacao",
		"embedding": makeEmbedding(1.0),
		"limit":     10,
	})
	req := httptest.NewRequest("POST", "/api/search/hybrid", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	ctx.HandleHybridSearch(rr, req)

	var resp struct {
		Results []hybridSearchResult `json:"results"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	for _, r := range resp.Results {
		if r.Filename == "notes/longa.md" {
			t.Errorf("longa.md não deveria aparecer: 1 chunk de 3 com sim < 82%% (sem aval do FTS)")
			break
		}
	}
	foundOk := false
	for _, r := range resp.Results {
		if r.Filename == "notes/ok.md" {
			foundOk = true
			break
		}
	}
	if !foundOk {
		t.Errorf("ok.md deveria aparecer")
	}
}

// A mesma nota longa passa quando 2 chunks ficam dentro do corte (match duplo).
func TestHybrid_ConsensoAprovaMatchDuplo(t *testing.T) {
	ctx := newTestContext(t)

	indexNoteFTS(t, ctx, "notes/longa2.md", "# Arquitetura\nSobre clean architecture e microsservicos.")
	indexNoteSemanticMulti(t, ctx, "notes/longa2.md", makeEmbedding(0.3), makeEmbedding(0.3), makeEmbedding(-1.0))

	body, _ := json.Marshal(map[string]interface{}{
		"query":     "programacao",
		"embedding": makeEmbedding(1.0),
		"limit":     10,
	})
	req := httptest.NewRequest("POST", "/api/search/hybrid", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	ctx.HandleHybridSearch(rr, req)

	var resp struct {
		Results []hybridSearchResult `json:"results"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	for _, r := range resp.Results {
		if r.Filename == "notes/longa2.md" {
			return // ok — match em 2 chunks
		}
	}
	t.Errorf("longa2.md deveria aparecer: 2 chunks dentro do corte")
}

// Doc ancorado no FTS (termo no conteúdo) NÃO passa pelo consenso: a evidência
// textual já basta, mesmo com 1 único chunk semântico fraco.
func TestHybrid_ConsensoNaoSeAplicaAAncoradoFTS(t *testing.T) {
	ctx := newTestContext(t)

	// Âncora no FTS: conteúdo contém o termo.
	indexNoteFTS(t, ctx, "notes/ancorada.md", "# Python\nProgramacao em python.")
	indexNoteSemanticMulti(t, ctx, "notes/ancorada.md", makeEmbedding(0.3), makeEmbedding(-1.0), makeEmbedding(-1.0))

	body, _ := json.Marshal(map[string]interface{}{
		"query":     "programacao",
		"embedding": makeEmbedding(1.0),
		"limit":     10,
	})
	req := httptest.NewRequest("POST", "/api/search/hybrid", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	ctx.HandleHybridSearch(rr, req)

	var resp struct {
		Results []hybridSearchResult `json:"results"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	for _, r := range resp.Results {
		if r.Filename == "notes/ancorada.md" {
			if r.RankSem == nil {
				t.Errorf("ancorada.md: esperava rank_sem (âncora FTS não exige consenso)")
			}
			return // ok
		}
	}
	t.Errorf("ancorada.md deveria aparecer: âncora FTS dispensa o consenso")
}

// Nota curta (1 chunk) com sim ~75% passa mesmo sem atingir os 82% excepcionais:
// o chunk único é a nota inteira.
func TestHybrid_ConsensoNaoExcluiNotaCurta(t *testing.T) {
	ctx := newTestContext(t)

	indexNoteFTS(t, ctx, "notes/curta.md", "# Topico\nConteudo sem o termo.")
	indexNoteSemantic(t, ctx, "notes/curta.md", makeEmbedding(0.3)) // sim ~75%

	body, _ := json.Marshal(map[string]interface{}{
		"query":     "programacao",
		"embedding": makeEmbedding(1.0),
		"limit":     10,
	})
	req := httptest.NewRequest("POST", "/api/search/hybrid", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	ctx.HandleHybridSearch(rr, req)

	var resp struct {
		Results []hybridSearchResult `json:"results"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	for _, r := range resp.Results {
		if r.Filename == "notes/curta.md" {
			return // ok — nota curta passa com match único
		}
	}
	t.Errorf("curta.md deveria aparecer: nota de 1 chunk passa com match único")
}

// ── Threshold híbrido dedicado ──

// hybrid_semantic_threshold=80 rejeita candidato a ~75%, mesmo com consenso ok.
func TestHybrid_ThresholdDedicado(t *testing.T) {
	ctx := newTestContext(t)

	indexNoteFTS(t, ctx, "notes/curta.md", "# Topico\nConteudo sem o termo.")
	indexNoteSemantic(t, ctx, "notes/curta.md", makeEmbedding(0.3)) // sim ~75%

	ctx.Store.SetSetting("hybrid_semantic_threshold", "80")

	body, _ := json.Marshal(map[string]interface{}{
		"query":     "programacao",
		"embedding": makeEmbedding(1.0),
		"limit":     10,
	})
	req := httptest.NewRequest("POST", "/api/search/hybrid", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	ctx.HandleHybridSearch(rr, req)

	var resp struct {
		Results []hybridSearchResult `json:"results"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	for _, r := range resp.Results {
		if r.Filename == "notes/curta.md" {
			t.Errorf("curta.md não deveria aparecer: threshold híbrido de 80%% > sim ~75%%")
			return
		}
	}
}

// Sem a setting dedicada, o híbrido usa o threshold da busca semântica pura.
func TestHybrid_ThresholdFallbackParaSemanticaPura(t *testing.T) {
	ctx := newTestContext(t)

	indexNoteFTS(t, ctx, "notes/curta.md", "# Topico\nConteudo sem o termo.")
	indexNoteSemantic(t, ctx, "notes/curta.md", makeEmbedding(0.3)) // sim ~75%

	ctx.Store.SetSetting("semantic_search_threshold", "30")

	body, _ := json.Marshal(map[string]interface{}{
		"query":     "programacao",
		"embedding": makeEmbedding(1.0),
		"limit":     10,
	})
	req := httptest.NewRequest("POST", "/api/search/hybrid", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	ctx.HandleHybridSearch(rr, req)

	var resp struct {
		Results []hybridSearchResult `json:"results"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	for _, r := range resp.Results {
		if r.Filename == "notes/curta.md" {
			return // ok — fallback para 30% aceita os 75%
		}
	}
	t.Errorf("curta.md deveria aparecer: fallback para semantic_search_threshold=30")
}
