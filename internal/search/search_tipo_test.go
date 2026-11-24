package search

import (
	"context"
	"strings"
	"testing"
	"time"

	"ton618/core/internal/core/db"
)

// ── extractNoteTipo ─────────────────────────────────────────────────

func TestExtractNoteTipo(t *testing.T) {
	tests := []struct {
		name          string
		raw           string
		wantTipos     []string
		wantRemaining string
	}{
		{
			name:          "filtro simples",
			raw:           "tipo:pdf relatorio",
			wantTipos:     []string{"pdf"},
			wantRemaining: " relatorio",
		},
		{
			name:          "alias type: com valor normalizado",
			raw:           "type:markmap assunto",
			wantTipos:     []string{"mapa_mental"},
			wantRemaining: " assunto",
		},
		{
			name:          "valor com espaço entre aspas",
			raw:           `tipo:"mapa mental" node`,
			wantTipos:     []string{"mapa_mental"},
			wantRemaining: " node",
		},
		{
			name:          "com prefixo + e maiúsculas",
			raw:           "+TIPO:Desenho",
			wantTipos:     []string{"desenho"},
			wantRemaining: " ",
		},
		{
			name:          "múltiplos filtros (OR)",
			raw:           "tipo:anexo tipo:arquivo zip",
			wantTipos:     []string{"anexo", "arquivo"},
			wantRemaining: "  zip",
		},
		{
			name:          "sem filtro devolve query intacta",
			raw:           "busca comum",
			wantTipos:     nil,
			wantRemaining: "busca comum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tipos, remaining := extractNoteTipo(tt.raw)
			if strings.Join(tipos, ",") != strings.Join(tt.wantTipos, ",") {
				t.Errorf("tipos = %v, want %v", tipos, tt.wantTipos)
			}
			if strings.TrimSpace(remaining) != strings.TrimSpace(tt.wantRemaining) {
				t.Errorf("remaining = %q, want %q", remaining, tt.wantRemaining)
			}
		})
	}
}

// ── buildFTSQuery com filtro de tipo ────────────────────────────────

func TestBuildFTSQuery_TipoFilter(t *testing.T) {
	got := buildFTSQuery("tipo:mapa_mental node")
	if !strings.Contains(got, `note_type:"mapa_mental"`) {
		t.Errorf("esperado predicado note_type, got %q", got)
	}
	if !strings.Contains(got, "node") {
		t.Errorf("termo de conteúdo deveria ser preservado, got %q", got)
	}
	// "tipo" não deve virar termo de conteúdo
	if strings.Contains(got, `texto:tipo`) {
		t.Errorf("'tipo' não deveria virar termo de conteúdo, got %q", got)
	}
}

func TestBuildFTSQuery_TipoSozinho(t *testing.T) {
	// Só o filtro, sem termos: o FTS ainda precisa de um MATCH não-vazio.
	got := buildFTSQuery("tipo:desenho")
	if got != `note_type:"desenho"` {
		t.Errorf("query = %q, want %q", got, `note_type:"desenho"`)
	}
}

func TestBuildFTSQuery_TiposMultiplos(t *testing.T) {
	got := buildFTSQuery("tipo:pdf tipo:epub leitura")
	if !strings.Contains(got, `note_type:"pdf" OR note_type:"epub"`) {
		t.Errorf("esperado OR entre tipos, got %q", got)
	}
}

// ── ponta a ponta (FTS + exclusões relaxadas) ───────────────────────

func TestSearch_FiltroTipo(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	now := time.Now()

	// Nota comum (note_type=nota) — InsertDocument/IndexFTS derivam de tags+caminho.
	store.InsertDocument(db.Document{
		ID: "doc-nota", Tipo: "markdown", Arquivo: "notes/comum.md", Secao: "Geral",
		Texto: "texto sobre produto", Tags: "", Timestamp: now.Format(time.RFC3339),
	})
	store.IndexFTS("doc-nota", "markdown", "notes/comum.md", "Geral", "texto sobre produto", "")

	// Desenho — normalmente EXCLUÍDO da listagem geral, mas alcançável por tipo.
	store.InsertDocument(db.Document{
		ID: "doc-desenho", Tipo: "markdown", Arquivo: "notes/desenho.md", Secao: "Geral",
		Texto: "texto sobre produto num canvas", Tags: "drawing", Timestamp: now.Format(time.RFC3339),
	})
	store.IndexFTS("doc-desenho", "markdown", "notes/desenho.md", "Geral", "texto sobre produto num canvas", "drawing")

	// PDF — também excluído da listagem geral.
	store.InsertDocument(db.Document{
		ID: "doc-pdf", Tipo: "pdf", Arquivo: "pdfs/produto.pdf", Secao: "Geral",
		Texto: "texto sobre produto em pdf", Tags: "", Timestamp: now.Format(time.RFC3339),
	})
	store.IndexFTS("doc-pdf", "pdf", "pdfs/produto.pdf", "Geral", "texto sobre produto em pdf", "")

	search := func(q string) *SearchResults {
		r, err := Search(context.Background(), store, q, 0, 20,
			func(s string) int { return 0 },
			func(s string) float64 { return 0.0 },
		)
		if err != nil {
			t.Fatalf("Search(%q) error: %v", q, err)
		}
		return r
	}
	ids := func(r *SearchResults) map[string]bool {
		m := make(map[string]bool)
		for _, h := range r.Hits {
			m[h.ID] = true
		}
		return m
	}

	// 1. Busca geral "produto": desenho e pdf continuam escondidos.
	geral := search("produto")
	if !ids(geral)["doc-nota"] {
		t.Errorf("busca geral deveria achar a nota comum: %v", ids(geral))
	}
	if ids(geral)["doc-desenho"] || ids(geral)["doc-pdf"] {
		t.Errorf("busca geral não deveria expor desenho/pdf: %v", ids(geral))
	}

	// 2. tipo:desenho — exclusão relaxada, alcança o desenho.
	porTipo := search("tipo:desenho")
	if !ids(porTipo)["doc-desenho"] {
		t.Errorf("tipo:desenho deveria achar o desenho: %v", ids(porTipo))
	}

	// 3. tipo:pdf — alcança o PDF.
	porPDF := search("tipo:pdf")
	if !ids(porPDF)["doc-pdf"] {
		t.Errorf("tipo:pdf deveria achar o pdf: %v", ids(porPDF))
	}

	// 4. Filtro + termo: interseção (nota comum não é desenho).
	comTermo := search("tipo:desenho produto")
	if !ids(comTermo)["doc-desenho"] {
		t.Errorf("'tipo:desenho produto' deveria achar o desenho: %v", ids(comTermo))
	}
	if ids(comTermo)["doc-nota"] {
		t.Errorf("'tipo:desenho produto' não deveria incluir a nota comum: %v", ids(comTermo))
	}

	// 5. Filtro sem match: vazio, não erro.
	if r := search("tipo:tabulator"); r.Total != 0 {
		t.Errorf("tipo:tabulator não deveria casar nada, total=%d", r.Total)
	}

	// 6. Alias: type:markmap ≡ tipo:mapa_mental (nenhuma nota indexada → vazio).
	if r := search("type:markmap"); r.Total != 0 {
		t.Errorf("type:markmap não deveria casar nada, total=%d", r.Total)
	}
}
