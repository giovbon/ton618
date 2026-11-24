package search

import (
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"ton618/core/internal/core/db"
)

// Reproduz o caso real da busca "lula": a palavra só aparece como substring no
// MEIO de outra ("celular"), nunca como token isolado. O FTS (indexa por
// palavra) não casa "lula"; a busca cai no fallback LIKE (substring), que acha a
// nota. O trecho deve sair do fragmento de TRANSCRIÇÃO (que contém a ocorrência)
// e "lula" precisa aparecer destacada (span search-highlight).
func TestHandleSearch_LulaMidPalavraDestaca(t *testing.T) {
	ctx := newTestContext(t)

	// Nota com cabeçalho curto + seção de transcrição longa (contém "celular").
	header := "# Manual de uma vida FELIZ: ABRACE SUA MEDIOCRIDADE"
	longText := "## Transcricao\n\nUma das coisas que mais te engana é você achar que precisa de mais tempo para começar. " +
		"Você passa muitas horas por dia com a cara no celular, sem perceber. " +
		"Pegue o teu celular e veja quanto tempo você perde todo dia."

	saveTestNote(t, ctx, "notes/imprevisivel-repro.md", header+"\n\n"+longText, "")

	now := time.Now().Format(time.RFC3339)
	// Fragmento 1 (cabeçalho, sem o termo).
	ctx.Store.InsertDocument(db.Document{
		ID: "repro-header", Tipo: "markdown", Arquivo: "notes/imprevisivel-repro.md",
		Secao: "Manual de uma vida FELIZ: ABRACE SUA MEDIOCRIDADE",
		Texto: header, Timestamp: now,
	})
	ctx.Store.IndexFTS("repro-header", "markdown", "notes/imprevisivel-repro.md",
		"Manual de uma vida FELIZ: ABRACE SUA MEDIOCRIDADE", header, "")
	// Fragmento 2 (transcrição, contém "celular" → "lula" como substring).
	ctx.Store.InsertDocument(db.Document{
		ID: "repro-transcricao", Tipo: "markdown", Arquivo: "notes/imprevisivel-repro.md",
		Secao: "Manual de uma vida FELIZ: ABRACE SUA MEDIOCRIDADE › Transcricao",
		Texto: longText, Timestamp: now,
	})
	ctx.Store.IndexFTS("repro-transcricao", "markdown", "notes/imprevisivel-repro.md",
		"Manual de uma vida FELIZ: ABRACE SUA MEDIOCRIDADE › Transcricao", longText, "")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/search?q=lula", nil)
	ctx.HandleSearch(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d, body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	// O resultado deve apontar para a nota.
	if !strings.Contains(body, "imprevisivel-repro") {
		t.Fatalf("esperava a nota no resultado, body:\n%s", body)
	}

	// O trecho precisa conter "celular" (onde "lula" ocorre).
	clean := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(body, "")
	if !strings.Contains(clean, "celular") {
		t.Errorf("esperava o trecho com 'celular', body limpo:\n%s", clean)
	}

	// "lula" deve estar DESTACADA (span search-highlight em volta dela).
	if !strings.Contains(body, "search-highlight") {
		t.Errorf("esperava ao menos um span search-highlight (termo destacado), body:\n%s", body)
	}

	// Confirma que "lula" é destacada DENTRO de "celular" (ca[highlight]lula[r]).
	if !strings.Contains(body, "search-highlight") || !strings.Contains(body, "lula") {
		t.Fatalf("esperava 'lula' presente e destacada, body:\n%s", body)
	}
	// Verifica que há um span cujo texto (limpo) contém "lula".
	if !regexp.MustCompile(`(?i)search-highlight[^>]*>[^<]*lula[^<]*<`).MatchString(body) {
		t.Errorf("não há span search-highlight envolvendo 'lula', body:\n%s", body)
	}
}

// Regressão REAL, ponta a ponta: o termo \"lula\" aparece como substring no meio de
// \"celular\" e DEPOIS de uma longa sequência de palavras acentuadas. O bug (offset
// por removeAccents) fazia o trecho recortado não conter o termo, então o destaque
// sumia. Este teste trava o comportamento no pipeline completo (FTS não casa ->
// LIKE acha -> fragmento do corpo -> trecho com destaque).
func TestHandleSearch_RegressaoOffsetAcentos_LulaDestaca(t *testing.T) {
	ctx := newTestContext(t)

	prefix := strings.Repeat("conversa sobre educação e nação e região e ação, ", 40)
	longText := prefix + "você passa tempo com o celular para estudar."
	header := "# Nota de regressão offset"
	saveTestNote(t, ctx, "notes/regressao-offset.md", header+"\n"+longText, "")

	now := time.Now().Format(time.RFC3339)
	ctx.Store.InsertDocument(db.Document{
		ID: "regoff-header", Tipo: "markdown", Arquivo: "notes/regressao-offset.md",
		Secao: "Header", Texto: header, Timestamp: now,
	})
	ctx.Store.IndexFTS("regoff-header", "markdown", "notes/regressao-offset.md", "Header", header, "")
	ctx.Store.InsertDocument(db.Document{
		ID: "regoff-body", Tipo: "markdown", Arquivo: "notes/regressao-offset.md",
		Secao: "Corpo", Texto: longText, Timestamp: now,
	})
	ctx.Store.IndexFTS("regoff-body", "markdown", "notes/regressao-offset.md", "Corpo", longText, "")

	rec := httptest.NewRecorder()
	ctx.HandleSearch(rec, httptest.NewRequest("POST", "/search?q=lula", nil))
	if rec.Code != 200 {
		t.Fatalf("status %d, body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	if !strings.Contains(body, "search-highlight") {
		t.Fatalf("esperava destaque de 'lula' (dentro de celular) apesar do offset de acentos, body:\n%s", body)
	}
	// Com o span, "celular" fica quebrado em ce[span]lula[/span]r — "celular"
	// contíguo não existe no HTML. Confirmamos que o destaque está no meio dela.
	if !strings.Contains(body, "ce<span") || !strings.Contains(body, "lula</span>r") {
		t.Fatalf("esperava 'lula' destacada no meio de 'celular' (ce[highlight]lula[/highlight]r), body:\n%s", body)
	}
	if !regexp.MustCompile(`(?i)search-highlight[^>]*>[^<]*lula[^<]*<`).MatchString(body) {
		t.Fatalf("esperava span search-highlight envolvendo 'lula', body:\n%s", body)
	}
}
