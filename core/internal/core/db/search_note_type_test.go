package db

import (
	"testing"

	"ton618/core/internal/core/domain"
)

// ── SearchSimilar: NoteType nos resultados ──────────────────────────────────
//
// Estes testes verificam que SearchSimilar popula o campo NoteType em cada
// SimilarResult de acordo com as tags persistidas no banco, garantindo que
// o frontend receba o tipo correto para gerar o link/rota certa para cada
// tipo de nota (markmap → /mindmap, drawing → /drawing, etc.).

// setupSearchNote é um helper que salva uma nota, atribui tags e indexa
// um único chunk com o valor val na posição [0] do embedding.
func setupSearchNote(s *Store, t *testing.T, filename string, tags []string, val float32) {
	t.Helper()
	if err := s.SaveNote(filename, "# Conteúdo de "+filename, "2024-01-01T00:00:00Z"); err != nil {
		t.Fatalf("SaveNote(%q): %v", filename, err)
	}
	if err := s.SetFileTags(filename, tags); err != nil {
		t.Fatalf("SetFileTags(%q): %v", filename, err)
	}
	chunks := []ChunkInfo{makeChunk(filename, 0, "conteudo", val)}
	if err := s.SaveNoteChunks(filename, chunks); err != nil {
		t.Fatalf("SaveNoteChunks(%q): %v", filename, err)
	}
}

// TestSearchSimilar_NoteTypeMarkdown verifica que uma nota markdown normal
// retorna NoteType = "nota".
func TestSearchSimilar_NoteTypeMarkdown(t *testing.T) {
	s := newTestStore(t)
	setupSearchNote(s, t, "notes/normal.md", []string{}, 1.0)

	query := make([]float32, EmbeddingDim)
	query[0] = 1.0

	results, err := s.SearchSimilar(query, 10)
	if err != nil {
		t.Fatalf("SearchSimilar: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("esperado 1 resultado, got %d", len(results))
	}
	if results[0].NoteType != domain.NoteTypeMarkdown {
		t.Errorf("NoteType esperado %q (markdown), got %q", domain.NoteTypeMarkdown, results[0].NoteType)
	}
}

// TestSearchSimilar_NoteTypeMarkmap verifica que uma nota com tag "markmap"
// retorna NoteType = "markmap" (NoteTypeMindmap), viabilizando o link /mindmap.
func TestSearchSimilar_NoteTypeMarkmap(t *testing.T) {
	s := newTestStore(t)
	setupSearchNote(s, t, "notes/meu-mapa.md", []string{"markmap"}, 1.0)

	query := make([]float32, EmbeddingDim)
	query[0] = 1.0

	results, err := s.SearchSimilar(query, 10)
	if err != nil {
		t.Fatalf("SearchSimilar: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("esperado 1 resultado, got %d", len(results))
	}
	if results[0].NoteType != domain.NoteTypeMindmap {
		t.Errorf("NoteType esperado %q (markmap), got %q", domain.NoteTypeMindmap, results[0].NoteType)
	}
}

// TestSearchSimilar_NoteTypeMindmapAlias verifica que a tag "mindmap"
// (alias de "markmap") também retorna NoteTypeMindmap.
func TestSearchSimilar_NoteTypeMindmapAlias(t *testing.T) {
	s := newTestStore(t)
	setupSearchNote(s, t, "notes/mindmap.md", []string{"mindmap"}, 1.0)

	query := make([]float32, EmbeddingDim)
	query[0] = 1.0

	results, err := s.SearchSimilar(query, 10)
	if err != nil {
		t.Fatalf("SearchSimilar: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("esperado 1 resultado, got %d", len(results))
	}
	if results[0].NoteType != domain.NoteTypeMindmap {
		t.Errorf("NoteType esperado %q (mindmap alias), got %q", domain.NoteTypeMindmap, results[0].NoteType)
	}
}

// TestSearchSimilar_NoteTypeYoutube verifica que uma nota com tag "youtube"
// retorna NoteTypeYoutube.
func TestSearchSimilar_NoteTypeYoutube(t *testing.T) {
	s := newTestStore(t)
	setupSearchNote(s, t, "notes/video.md", []string{"youtube"}, 1.0)

	query := make([]float32, EmbeddingDim)
	query[0] = 1.0

	results, err := s.SearchSimilar(query, 10)
	if err != nil {
		t.Fatalf("SearchSimilar: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("esperado 1 resultado, got %d", len(results))
	}
	if results[0].NoteType != domain.NoteTypeYoutube {
		t.Errorf("NoteType esperado %q (youtube), got %q", domain.NoteTypeYoutube, results[0].NoteType)
	}
}

// TestSearchSimilar_NoteTypeArticle verifica que nota com tag "artigo"
// retorna NoteTypeArticle.
func TestSearchSimilar_NoteTypeArticle(t *testing.T) {
	s := newTestStore(t)
	setupSearchNote(s, t, "notes/artigo.md", []string{"artigo"}, 1.0)

	query := make([]float32, EmbeddingDim)
	query[0] = 1.0

	results, err := s.SearchSimilar(query, 10)
	if err != nil {
		t.Fatalf("SearchSimilar: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("esperado 1 resultado, got %d", len(results))
	}
	if results[0].NoteType != domain.NoteTypeArticle {
		t.Errorf("NoteType esperado %q (artigo), got %q", domain.NoteTypeArticle, results[0].NoteType)
	}
}

// TestSearchSimilar_NoteTypeNaoIndexavelFiltrado garante que notas não-indexáveis
// (drawing) são FILTRADAS dos resultados mesmo que tenham sido indexadas
// previamente (regressão: tipo muda após indexação).
func TestSearchSimilar_NoteTypeNaoIndexavelFiltrado(t *testing.T) {
	s := newTestStore(t)

	// Indexa nota como markdown puro (sem tags)
	filename := "notes/virou-desenho.md"
	if err := s.SaveNote(filename, "# Conteúdo", "2024-01-01T00:00:00Z"); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	s.SetFileTags(filename, []string{})
	s.SaveNoteChunks(filename, []ChunkInfo{makeChunk(filename, 0, "conteudo", 1.0)})

	// Agora a nota vira drawing (tags atualizadas)
	if err := s.SetFileTags(filename, []string{"drawing"}); err != nil {
		t.Fatalf("SetFileTags: %v", err)
	}

	query := make([]float32, EmbeddingDim)
	query[0] = 1.0

	results, err := s.SearchSimilar(query, 10)
	if err != nil {
		t.Fatalf("SearchSimilar: %v", err)
	}
	for _, r := range results {
		if r.Filename == filename {
			t.Errorf("nota drawing não deveria aparecer nos resultados semânticos, got NoteType=%q", r.NoteType)
		}
	}
}

// TestSearchSimilar_MultiplostiposNoteType verifica que uma busca com múltiplas
// notas de tipos diferentes retorna o NoteType correto para cada uma.
func TestSearchSimilar_MultiplostiposNoteType(t *testing.T) {
	s := newTestStore(t)

	casos := []struct {
		filename string
		tags     []string
		val      float32
		wantType domain.NoteType
	}{
		{"notes/md.md", []string{}, 0.1, domain.NoteTypeMarkdown},
		{"notes/markmap.md", []string{"markmap"}, 0.2, domain.NoteTypeMindmap},
		{"notes/youtube.md", []string{"youtube"}, 0.3, domain.NoteTypeYoutube},
		{"notes/captura.md", []string{"captura"}, 0.4, domain.NoteTypeCapture},
	}

	for _, c := range casos {
		setupSearchNote(s, t, c.filename, c.tags, c.val)
	}

	query := make([]float32, EmbeddingDim)
	query[0] = 0.2 // próximo de markmap (0.2)

	results, err := s.SearchSimilar(query, 10)
	if err != nil {
		t.Fatalf("SearchSimilar: %v", err)
	}
	if len(results) != len(casos) {
		t.Fatalf("esperado %d resultados, got %d", len(casos), len(results))
	}

	// Monta mapa filename → NoteType obtido
	got := make(map[string]domain.NoteType, len(results))
	for _, r := range results {
		got[r.Filename] = r.NoteType
	}

	for _, c := range casos {
		if got[c.filename] != c.wantType {
			t.Errorf("filename=%q: NoteType esperado %q, got %q", c.filename, c.wantType, got[c.filename])
		}
	}
}

// TestSearchSimilar_NoteTypeNaoVazioParaIndexaveis garante que nenhum resultado
// indexável retorna NoteType vazio.
func TestSearchSimilar_NoteTypeNaoVazioParaIndexaveis(t *testing.T) {
	s := newTestStore(t)

	indexaveis := []struct {
		filename string
		tags     []string
	}{
		{"notes/a.md", []string{}},
		{"notes/b.md", []string{"markmap"}},
		{"notes/c.md", []string{"markmap"}},
		{"notes/d.md", []string{"artigo"}},
	}

	for i, n := range indexaveis {
		setupSearchNote(s, t, n.filename, n.tags, float32(i+1)*0.1)
	}

	query := make([]float32, EmbeddingDim)
	query[0] = 0.2

	results, err := s.SearchSimilar(query, 10)
	if err != nil {
		t.Fatalf("SearchSimilar: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("esperado pelo menos 1 resultado")
	}
	for _, r := range results {
		if r.NoteType == "" {
			t.Errorf("NoteType não pode ser vazio para resultado indexável: filename=%q", r.Filename)
		}
	}
}

// TestEditorRouteForTipo valida a correspondência entre NoteType e rota de editor.
// Este teste é agnóstico de banco — verifica apenas a lógica de domínio que
// o frontend replica em editorRouteForTipo().
func TestEditorRouteForTipo(t *testing.T) {
	casos := []struct {
		noteType  domain.NoteType
		wantRoute string
	}{
		{domain.NoteTypeMarkdown, "/editor"},
		{domain.NoteTypeMindmap, "/mindmap"},     // markmap → /mindmap
		{domain.NoteTypeDrawing, "/drawing"},
		{domain.NoteTypeYoutube, "/editor"},      // youtube abre no editor markdown
		{domain.NoteTypeArticle, "/editor"},      // artigo abre no editor markdown
		{domain.NoteTypeCapture, "/editor"},      // captura abre no editor markdown
	}

	for _, c := range casos {
		got := c.noteType.EditorRoute()
		if got != c.wantRoute {
			t.Errorf("NoteType %q: EditorRoute() = %q, queria %q", c.noteType, got, c.wantRoute)
		}
	}
}
