package system

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── Regressão: ordenação por mtime com formatos mistos ──
//
// O upload de anexos salvava o mtime em formato LOCAL (ex: -03:00) enquanto o
// watcher e o restante salvam em UTC ("Z"). Como a sidebar ordena por mtime,
// a comparação de strings misturava formatos e colocava anexos novos
// (15:55-03:00) como mais antigos que PDFs antigos (18:50Z).
//
// A correção (1) passa a salvar o mtime de anexos em UTC e (2) ordena pelo
// instante real (parsed time.Time) em vez da string. Estes testes garantem
// que a ordenação compara o instante real, não o texto.

// TestMtimeNewer_ComparesInstantNotString garante que a ordenação compara o
// instante real. Sem isso, um anexo em formato local (-03:00) seria ordenado
// como mais antigo que um PDF em UTC (Z) mesmo sendo mais novo no relógio.
func TestMtimeNewer_ComparesInstantNotString(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{
			// Cenário real do bug: anexo 15:55 -03:00 (== 18:55Z) é mais novo
			// que PDF 18:50Z, mas a string "15:55..." < "18:50..." faria o PDF
			// parecer mais novo.
			name: "anexo local mais novo que PDF UTC (mesmo dia)",
			a:    "2026-08-17T15:55:00-03:00",
			b:    "2026-08-17T18:50:00Z",
			want: true,
		},
		{
			name: "mesmo instante em formatos diferentes nao e estritamente mais novo",
			a:    "2026-08-17T18:55:00Z",
			b:    "2026-08-17T15:55:00-03:00",
			want: false,
		},
		{
			name: "UTC mais novo que local antigo",
			a:    "2026-08-17T19:00:00Z",
			b:    "2026-08-17T15:55:00-03:00",
			want: true,
		},
		{
			name: "mesmo offset ordena pelo instante",
			a:    "2026-08-17T15:55:00Z",
			b:    "2026-08-17T15:50:00Z",
			want: true,
		},
		{
			name: "a invalido nunca fica antes",
			a:    "nao-e-data",
			b:    "2026-08-17T15:55:00Z",
			want: false,
		},
		{
			name: "b invalido faz a (valido) ficar antes",
			a:    "2026-08-17T15:55:00Z",
			b:    "nao-e-data",
			want: true,
		},
		{
			name: "ambos invalidos empatam",
			a:    "nao-e-data",
			b:    "outra-coisa",
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mtimeNewer(tc.a, tc.b); got != tc.want {
				t.Errorf("mtimeNewer(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// TestHandleGetSidebar_OrdersNewerUploadFirst reproduz o cenário do bug:
// PDF antigo (mtime UTC) + anexo novo (mtime em formato local legado).
// O anexo, mesmo com string menor, deve aparecer ANTES do PDF na sidebar.
func TestHandleGetSidebar_OrdersNewerUploadFirst(t *testing.T) {
	ctx := newTestContext(t)

	// PDF criado às 18:50Z (watcher grava UTC)
	if err := ctx.Store.SetFileMod("pdfs/meu-pdf.pdf", "2026-08-17T18:50:00Z"); err != nil {
		t.Fatalf("SetFileMod pdf: %v", err)
	}
	// Anexo criado às 15:55 -03:00 (== 18:55Z) — MAIS NOVO, porém string menor
	if err := ctx.Store.SetFileMod("attachments/novo-anexo.zip", "2026-08-17T15:55:00-03:00"); err != nil {
		t.Fatalf("SetFileMod zip: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/sidebar", nil)
	rr := httptest.NewRecorder()
	ctx.HandleGetSidebar(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status esperado 200, got %d (%s)", rr.Code, rr.Body.String())
	}

	body := rr.Body.String()
	idxPdf := strings.Index(body, "meu-pdf")
	idxZip := strings.Index(body, "novo-anexo")
	if idxPdf < 0 || idxZip < 0 {
		t.Fatalf("itens não renderizados na sidebar (pdf=%d zip=%d)\nbody: %s", idxPdf, idxZip, body)
	}
	if idxZip > idxPdf {
		t.Errorf("anexo (mais novo) deveria aparecer antes do PDF; zip em %d, pdf em %d", idxZip, idxPdf)
	}
}

// TestHandleGetAllNotes_OrdersNewestFirst valida a mesma ordenação pelo
// endpoint JSON usado pelo Tabulator/database.
func TestHandleGetAllNotes_OrdersNewestFirst(t *testing.T) {
	ctx := newTestContext(t)

	if err := ctx.Store.SetFileMod("pdfs/meu-pdf.pdf", "2026-08-17T18:50:00Z"); err != nil {
		t.Fatalf("SetFileMod pdf: %v", err)
	}
	if err := ctx.Store.SetFileMod("attachments/novo-anexo.zip", "2026-08-17T15:55:00-03:00"); err != nil {
		t.Fatalf("SetFileMod zip: %v", err)
	}
	// Nota markdown recente deve vir primeiro
	if err := ctx.Store.SetFileMod("notes/nota-recente.md", "2026-08-17T20:00:00Z"); err != nil {
		t.Fatalf("SetFileMod nota: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/notes", nil)
	rr := httptest.NewRecorder()
	ctx.HandleGetAllNotes(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status esperado 200, got %d", rr.Code)
	}

	var resp struct {
		Notes []struct {
			Arquivo string `json:"arquivo"`
			Mtime   string `json:"mtime"`
		} `json:"notes"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Notes) != 3 {
		t.Fatalf("esperava 3 notas, got %d", len(resp.Notes))
	}

	got := []string{resp.Notes[0].Arquivo, resp.Notes[1].Arquivo, resp.Notes[2].Arquivo}
	want := []string{
		"notes/nota-recente.md",   // 20:00Z — mais novo
		"attachments/novo-anexo.zip", // 18:55Z real (15:55-03:00) — segundo
		"pdfs/meu-pdf.pdf",        // 18:50Z — mais antigo
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("posição %d: got %q, want %q (ordem completa: %v)", i, got[i], want[i], got)
		}
	}
}
