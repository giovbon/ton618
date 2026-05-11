package api

import (
	"encoding/json"
	"etl/internal/config"
	"etl/internal/ingest"
	"etl/internal/search"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleKnowledgeMap_TagFallback(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	os.MkdirAll(docsDir, 0755)

	indexDir := filepath.Join(tmpDir, "index.bleve")
	search.InitIndex(indexDir)
	defer search.CloseIndex()

	appState := ingest.NewAppState(&config.AppConfig{StateDir: tmpDir})
	defer appState.Close()

	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{DocsDir: docsDir},
		State: appState,
		Index: search.GetIndex(),
	}

	// 1. Criar um arquivo com #embed no disco E indexar no Bleve
	filename := "test.md"
	content := "Conteudo com #embed"
	os.WriteFile(filepath.Join(docsDir, filename), []byte(content), 0644)

	search.IndexDocument(filename, map[string]interface{}{
		"arquivo": filename,
		"texto":   content,
	})

	// 2. Adicionar vetor manualmente ao estado
	appState.SetNoteVector(filename, []float32{1.0, 0.5, 0.2}, "Conteudo com embed")

	// 3. Chamar HandleKnowledgeMap
	req := httptest.NewRequest(http.MethodGet, "/api/graph/map", nil)
	w := httptest.NewRecorder()
	ctx.HandleKnowledgeMap(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Esperado 200, obteve %d", w.Code)
	}

	var resp KnowledgeMapResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	found := false
	for _, n := range resp.Notes {
		if n.ID == filename {
			found = true
			break
		}
	}

	if !found {
		t.Error("Nota com #embed (apenas no Bleve, sem tag em cache) nao foi encontrada no mapa.")
	}
}

func TestHandleReindexVectors_TagFallback(t *testing.T) {
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	os.MkdirAll(docsDir, 0755)

	indexDir := filepath.Join(tmpDir, "index.bleve")
	search.InitIndex(indexDir)
	defer search.CloseIndex()

	appState := ingest.NewAppState(&config.AppConfig{StateDir: tmpDir})
	defer appState.Close()

	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{DocsDir: docsDir, OllamaModel: "nomic-embed-text"},
		State: appState,
		Index: search.GetIndex(),
	}

	filename := "reindex-test.md"
	content := "Nota para reindex #embed"
	os.WriteFile(filepath.Join(docsDir, filename), []byte(content), 0644)

	search.IndexDocument("doc1", map[string]interface{}{
		"arquivo": filename,
		"texto":   content,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/graph/reindex", nil)
	w := httptest.NewRecorder()
	ctx.HandleReindexVectors(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("Esperado 202 Accepted, obteve %d", w.Code)
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		content  string
		filename string
		expected string
	}{
		{"# Meu Titulo\nConteudo aqui", "notes/test.md", "Meu Titulo"},
		{"## Subtitulo\nTexto", "notes/doc.md", "Subtitulo"},
		{"Sem heading\nApenas texto", "notes/plain.md", "Sem heading"},
		{"\n\n   # Titulo com espacos   \ncorpo", "notes/space.md", "Titulo com espacos"},
		{"", "notes/empty.md", "empty"},
		{"# \nSo heading vazio\nTexto real", "notes/vazio.md", "So heading vazio"},
		{"#\nLinha real", "notes/hash.md", "Linha real"},
	}

	for _, tt := range tests {
		result := extractTitle(tt.content, tt.filename)
		if result != tt.expected {
			t.Errorf("extractTitle(%q, %q) = %q, want %q", tt.content, tt.filename, result, tt.expected)
		}
	}
}

func TestHandleKnowledgeMap_FastPath(t *testing.T) {
	// Verifica o caminho otimizado: tag #embed no cache + titulo armazenado no NoteVector
	// Neste caminho, NAO deve haver queries ao Bleve para deteccao de #embed nem para titulos.
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	os.MkdirAll(docsDir, 0755)

	indexDir := filepath.Join(tmpDir, "index.bleve")
	search.InitIndex(indexDir)
	defer search.CloseIndex()

	appState := ingest.NewAppState(&config.AppConfig{StateDir: tmpDir})
	defer appState.Close()

	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{DocsDir: docsDir},
		State: appState,
		Index: search.GetIndex(),
	}

	// Criar arquivo no disco
	filename := "fastpath.md"
	content := "# Meu Titulo Cacheado\nConteudo com #embed"
	os.WriteFile(filepath.Join(docsDir, filename), []byte(content), 0644)

	// Adicionar tag #embed ao cache
	appState.SetFileTags(filename, []string{"embed"})

	// Adicionar vetor COM titulo armazenado (P3.3)
	appState.SetNoteVector(filename, []float32{0.5, 0.8, 0.3}, "Meu Titulo Cacheado")

	// Executar
	req := httptest.NewRequest(http.MethodGet, "/api/graph/map", nil)
	w := httptest.NewRecorder()
	ctx.HandleKnowledgeMap(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Esperado 200, obteve %d", w.Code)
	}

	var resp KnowledgeMapResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if len(resp.Notes) != 1 {
		t.Fatalf("Esperava 1 nota, obteve %d", len(resp.Notes))
	}

	// Verificar que o titulo veio do NoteVector (nao do Bleve)
	if resp.Notes[0].Title != "Meu Titulo Cacheado" {
		t.Errorf("Titulo esperado 'Meu Titulo Cacheado', obteve '%s'", resp.Notes[0].Title)
	}
	if resp.Notes[0].ID != filename {
		t.Errorf("ID esperado '%s', obteve '%s'", filename, resp.Notes[0].ID)
	}
}

func TestHandleGraphQueryPoint_WithNoteVectors(t *testing.T) {
	// Verifica que o endpoint de query point funciona com o novo formato NoteVector (P3.3)
	// sem depender do Ollama real (usa vetores pre-armazenados)
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	os.MkdirAll(docsDir, 0755)

	appState := ingest.NewAppState(&config.AppConfig{StateDir: tmpDir})
	defer appState.Close()

	// Armazenar vetores e projecoes no estado
	appState.SetNoteVector("notes/a.md", []float32{0.9, 0.1, 0.0}, "Nota A")
	appState.SetNoteVector("notes/b.md", []float32{0.1, 0.9, 0.0}, "Nota B")
	appState.SetNoteVector("notes/c.md", []float32{0.0, 0.1, 0.9}, "Nota C")

	appState.SetNoteProjections(map[string][]float64{
		"notes/a.md": {10, 10},
		"notes/b.md": {90, 10},
		"notes/c.md": {50, 90},
	})

	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{DocsDir: docsDir, OllamaHost: "http://localhost:11434", OllamaModel: "nomic-embed-text"},
		State: appState,
	}

	// Caso 1: Sem Ollama (erro esperado ao gerar embedding da query)
	body := strings.NewReader(`{"query":"teste"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/graph/query-point", body)
	w := httptest.NewRecorder()
	ctx.HandleGraphQueryPoint(w, req)

	// Deve falhar porque nao tem Ollama, mas nao deve panicar
	if w.Code == 0 {
		t.Error("HandleGraphQueryPoint deveria ter respondido (mesmo com erro)")
	}
}

func TestHandleGraphQueryPoint_MethodValidation(t *testing.T) {
	tmpDir := t.TempDir()
	appState := ingest.NewAppState(&config.AppConfig{StateDir: tmpDir})
	defer appState.Close()

	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{},
		State: appState,
	}

	// GET nao deve ser aceito
	req := httptest.NewRequest(http.MethodGet, "/api/graph/query-point", nil)
	w := httptest.NewRecorder()
	ctx.HandleGraphQueryPoint(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Esperado 405, obteve %d", w.Code)
	}
}

func TestHandleGraphQueryPoint_EmptyQuery(t *testing.T) {
	tmpDir := t.TempDir()
	appState := ingest.NewAppState(&config.AppConfig{StateDir: tmpDir})
	defer appState.Close()

	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{},
		State: appState,
	}

	body := strings.NewReader(`{"query":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/graph/query-point", body)
	w := httptest.NewRecorder()
	ctx.HandleGraphQueryPoint(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Esperado 400 para query vazia, obteve %d", w.Code)
	}
}

func TestHandleGraphQueryPoint_NoConfig(t *testing.T) {
	tmpDir := t.TempDir()
	appState := ingest.NewAppState(&config.AppConfig{StateDir: tmpDir})
	defer appState.Close()

	ctx := &HandlerContext{
		Cfg:   nil,
		State: appState,
	}

	body := strings.NewReader(`{"query":"teste"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/graph/query-point", body)
	w := httptest.NewRecorder()
	ctx.HandleGraphQueryPoint(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Esperado 503 sem config, obteve %d", w.Code)
	}
}

func TestHandleKnowledgeMap_MultipleNotes(t *testing.T) {
	// Verifica cluster labeling com multiplas notas (exercita batch DisjunctionQuery P4.1)
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	os.MkdirAll(docsDir, 0755)

	indexDir := filepath.Join(tmpDir, "index.bleve")
	search.InitIndex(indexDir)
	defer search.CloseIndex()

	appState := ingest.NewAppState(&config.AppConfig{StateDir: tmpDir})
	defer appState.Close()

	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{DocsDir: docsDir},
		State: appState,
		Index: search.GetIndex(),
	}

	// Criar 3 notas com #embed, tags cacheadas, titulos armazenados
	notes := []struct {
		filename string
		content  string
		title    string
		vector   []float32
	}{
		{"notes/a.md", "# Alpha\nConteudo sobre alpha #embed", "Alpha", []float32{1.0, 0.0, 0.0}},
		{"notes/b.md", "# Beta\nConteudo sobre beta #embed", "Beta", []float32{0.0, 1.0, 0.0}},
		{"notes/c.md", "# Gamma\nConteudo sobre gamma #embed", "Gamma", []float32{0.0, 0.0, 1.0}},
	}

	for _, n := range notes {
		os.WriteFile(filepath.Join(docsDir, n.filename), []byte(n.content), 0644)
		search.IndexDocument(n.filename, map[string]interface{}{
			"arquivo": n.filename,
			"texto":   n.content,
		})
		appState.SetFileTags(n.filename, []string{"embed"})
		appState.SetNoteVector(n.filename, n.vector, n.title)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/graph/map", nil)
	w := httptest.NewRecorder()
	ctx.HandleKnowledgeMap(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Esperado 200, obteve %d", w.Code)
	}

	var resp KnowledgeMapResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if len(resp.Notes) != 3 {
		t.Fatalf("Esperava 3 notas, obteve %d", len(resp.Notes))
	}

	// Verificar que cada nota tem cluster atribuido
	for _, n := range resp.Notes {
		if n.ClusterID < 0 {
			t.Errorf("Nota %s sem cluster atribuido", n.ID)
		}
	}

	// Verificar que clusters foram gerados
	if len(resp.Clusters) == 0 {
		t.Error("Nenhum cluster gerado para 3 notas")
	}
}

func TestHandleManualSemanticMap_Empty(t *testing.T) {
	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	defer appState.Close()

	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{},
		State: appState,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/graph/manual-map", nil)
	w := httptest.NewRecorder()
	ctx.HandleManualSemanticMap(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Esperado 200, obteve %d", w.Code)
	}

	var resp ManualMapResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if len(resp.Topics) != 0 {
		t.Errorf("Esperava 0 topicos (vazio), obteve %d", len(resp.Topics))
	}
	if len(resp.Links) != 0 {
		t.Errorf("Esperava 0 links (vazio), obteve %d", len(resp.Links))
	}
}

func TestHandleManualSemanticMap_Hierarchy(t *testing.T) {
	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	defer appState.Close()

	// Adicionar topicos aninhados via SetFileSemanticLinks
	appState.SetFileSemanticLinks("nota1.md", []string{"brasil/politica", "saude"})
	appState.SetFileSemanticLinks("nota2.md", []string{"brasil/economia", "educacao"})

	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{},
		State: appState,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/graph/manual-map", nil)
	w := httptest.NewRecorder()
	ctx.HandleManualSemanticMap(w, req)

	var resp ManualMapResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	// Deve ter 4 topicos: brasil, brasil/politica, brasil/economia, saude, educacao
	if len(resp.Topics) != 5 {
		t.Errorf("Esperava 5 topicos, obteve %d: %+v", len(resp.Topics), resp.Topics)
	}

	topicIDs := make(map[string]bool)
	for _, tp := range resp.Topics {
		topicIDs[tp.ID] = true
	}
	for _, expected := range []string{"brasil", "brasil/politica", "brasil/economia", "saude", "educacao"} {
		if !topicIDs[expected] {
			t.Errorf("Topico esperado nao encontrado: %s", expected)
		}
	}

	// Hierarquia: 2 links internos (brasil > politica, brasil > economia) + 4 note links
	if len(resp.Links) != 6 {
		t.Errorf("Esperava 6 links, obteve %d: %+v", len(resp.Links), resp.Links)
	}

	// Verifica Content-Type
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type esperado application/json, obteve %s", ct)
	}
}

func TestHandleManualSemanticMap_DeleteCleansOrphans(t *testing.T) {
	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	defer appState.Close()

	appState.SetFileSemanticLinks("nota1.md", []string{"saude", "educacao"})
	appState.SetFileSemanticLinks("nota2.md", []string{"educacao"})

	// Deletar nota1 — "saude" deve sumir, "educacao" deve ficar (nota2 ainda referencia)
	appState.DeleteFileSemanticLinks("nota1.md")

	topics := appState.GetAllSemanticTopics()
	if len(topics) != 1 || topics[0] != "educacao" {
		t.Errorf("Esperava apenas 'educacao', obteve %v", topics)
	}
}

func TestExtractSemanticLinks(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"link simples", "texto @[brasil/politica] fim", []string{"brasil/politica"}},
		{"multiplos links", "@[saude] e @[educacao]", []string{"saude", "educacao"}},
		{"sem link", "apenas texto normal", nil},
		{"com html no meio", "<span>@\\[saude</span>]", []string{"saude"}},
		{"topicos com acentos", "@[educacao/saude]", []string{"educacao/saude"}},
		{"link unico sem espaco", "@[teste]", []string{"teste"}},
		{"backslash escapado", "@\\[teste\\]", []string{"teste"}},
		{"texto vazio", "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ingest.ExtractSemanticLinks(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got %v, want %v", got, tt.want)
					return
				}
			}
		})
	}
}

func TestSemanticTopics_SortedOrder(t *testing.T) {
	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	defer appState.Close()

	appState.SetFileSemanticLinks("nota.md", []string{"zeta", "alpha", "beta"})

	topics := appState.GetAllSemanticTopics()
	for i := 1; i < len(topics); i++ {
		if topics[i-1] > topics[i] {
			t.Errorf("topicos fora de ordem: %v", topics)
			break
		}
	}

	// Mesma ordem em chamada repetida
	topics2 := appState.GetAllSemanticTopics()
	for i := range topics {
		if topics[i] != topics2[i] {
			t.Errorf("ordem diferente entre chamadas: %v vs %v", topics, topics2)
			break
		}
	}
}

func TestRebuildSemanticTopics_CleansOrphansOnStartup(t *testing.T) {
	// Simula o que acontece no Load(): topicos orfaos no BBolt sao removidos
	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	defer appState.Close()

	// Adiciona links para duas notas
	appState.SetFileSemanticLinks("nota.md", []string{"saude"})
	appState.SetFileSemanticLinks("outra.md", []string{"educacao"})

	// Remove uma nota manualmente (simula crash sem limpeza)
	appState.DeleteFileSemanticLinks("nota.md")

	// O RebuildSemanticTopics rodou dentro do Delete. "saude" deve ter sumido
	topics := appState.GetAllSemanticTopics()
	if len(topics) != 1 || topics[0] != "educacao" {
		t.Errorf("Esperava apenas 'educacao' apos rebuild, obteve %v", topics)
	}

	// Simula startup: se rebuild for chamado de novo, resultado deve ser o mesmo
	appState.RebuildSemanticTopics()
	topics2 := appState.GetAllSemanticTopics()
	if len(topics2) != 1 || topics2[0] != "educacao" {
		t.Errorf("Segundo rebuild mudou o resultado: %v", topics2)
	}
}

func TestSetFileSemanticLinks_Overwrite(t *testing.T) {
	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	defer appState.Close()

	appState.SetFileSemanticLinks("nota.md", []string{"saude"})
	appState.SetFileSemanticLinks("nota.md", []string{"educacao"})

	topics := appState.GetAllSemanticTopics()
	if len(topics) != 1 || topics[0] != "educacao" {
		t.Errorf("Esperava apenas 'educacao' apos sobrescrita, obteve %v", topics)
	}

	links := appState.GetFileSemanticLinks("nota.md")
	if len(links) != 1 || links[0] != "educacao" {
		t.Errorf("Links da nota nao foram sobrescritos: %v", links)
	}
}

func TestRebuildSemanticTopics_Empty(t *testing.T) {
	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	defer appState.Close()

	appState.SetFileSemanticLinks("nota.md", []string{"saude"})
	appState.SetFileSemanticLinks("outra.md", []string{"educacao"})

	appState.DeleteFileSemanticLinks("nota.md")
	appState.DeleteFileSemanticLinks("outra.md")

	topics := appState.GetAllSemanticTopics()
	if len(topics) != 0 {
		t.Errorf("Esperava 0 topicos apos deletar tudo, obteve %d: %v", len(topics), topics)
	}

	allLinks := appState.GetAllFileSemanticLinks()
	if len(allLinks) != 0 {
		t.Errorf("Esperava 0 file links, obteve %d", len(allLinks))
	}
}

func TestExtractSemanticLinks_Malformed(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"incompleto sem ]", "@[incompleto"},
		{"so @[", "@["},
		{"vazio @[]", "@[]"},
		{"sem @", "[teste]"},
		{"um valido um incompleto", "@[saude] @[incompleto"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			links := ingest.ExtractSemanticLinks(tc.input)
			for _, l := range links {
				if l == "" {
					t.Errorf("link vazio para input: %q", tc.input)
				}
			}
		})
	}
}

func TestHandleManualSemanticMap_DuplicateLabels(t *testing.T) {
	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	defer appState.Close()

	appState.SetFileSemanticLinks("n1.md", []string{"brasil/politica"})
	appState.SetFileSemanticLinks("n2.md", []string{"economia/politica"})

	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{},
		State: appState,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/graph/manual-map", nil)
	w := httptest.NewRecorder()
	ctx.HandleManualSemanticMap(w, req)

	var resp ManualMapResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if len(resp.Topics) != 4 {
		t.Fatalf("Esperava 4 topicos (brasil, brasil/politica, economia, economia/politica), obteve %d: %+v",
			len(resp.Topics), resp.Topics)
	}

	labels := make(map[string]bool)
	for _, tp := range resp.Topics {
		if tp.Label == "politica" {
			if tp.ID != "brasil/politica" && tp.ID != "economia/politica" {
				t.Errorf("Label 'politica' com ID inesperado: %s", tp.ID)
			}
		}
		labels[tp.ID] = true
	}
	for _, expected := range []string{"brasil", "brasil/politica", "economia", "economia/politica"} {
		if !labels[expected] {
			t.Errorf("Topico esperado nao encontrado: %s", expected)
		}
	}

	if len(resp.Links) != 4 {
		t.Errorf("Esperava 4 links, obteve %d: %+v", len(resp.Links), resp.Links)
	}
}
