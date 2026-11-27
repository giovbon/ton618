package system

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestHandleGetSidebar_ExtractButtonOnlyForPDFAndEPUB garante que a lista de
// Notas (busca/sidebar) mostra o botão de extrair texto apenas nas linhas de
// PDF/EPUB — nunca em notas markdown comuns. O botão é a contraparte do "⤓"
// do Acervo e dispara POST /api/extract-text.
func TestHandleGetSidebar_ExtractButtonOnlyForPDFAndEPUB(t *testing.T) {
	ctx := newTestContext(t)

	now := time.Now().Format(time.RFC3339)
	for _, f := range []string{"pdfs/extraivel-pdf.pdf", "epubs/extraivel-epub.epub", "notes/nota-comum.md", "attachments/anexo.pdf"} {
		if err := ctx.Store.SetFileMod(f, now); err != nil {
			t.Fatalf("SetFileMod %s: %v", f, err)
		}
	}

	req := httptest.NewRequest("GET", "/api/sidebar", nil)
	rr := httptest.NewRecorder()
	ctx.HandleGetSidebar(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status esperado 200, got %d (%s)", rr.Code, rr.Body.String())
	}

	// Cada linha começa com a mesma classe do container de SidebarNoteItem;
	// dividir por ela isola o HTML de cada item para não confundir o botão de
	// uma linha com o de outra.
	rows := strings.Split(rr.Body.String(), `class="flex items-center gap-3 px-4 py-2`)
	cases := []struct {
		needle string
		want   bool
	}{
		{"extraivel-pdf.pdf", true},
		{"extraivel-epub.epub", true},
		{"nota-comum", false},
		{"anexo.pdf", false}, // fora de pdfs/: o backend não extrai anexos
	}

	for _, tc := range cases {
		t.Run(tc.needle, func(t *testing.T) {
			for _, row := range rows[1:] {
				if !strings.Contains(row, tc.needle) {
					continue
				}
				got := strings.Contains(row, "sidebar-extract-btn")
				if got != tc.want {
					t.Errorf("botão de extrair em %q = %v, want %v", tc.needle, got, tc.want)
				}
				return
			}
			t.Fatalf("linha de %q não encontrada na sidebar", tc.needle)
		})
	}
}
