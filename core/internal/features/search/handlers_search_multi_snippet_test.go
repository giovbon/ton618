package search

import (
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"ton618/core/internal/core/db"
)

// stripTags remove as tags HTML para conferir apenas o texto visível (os
// trechos trazem os spans de destaque embutidos).
func stripTags(html string) string {
	return regexp.MustCompile(`<[^>]*>`).ReplaceAllString(html, "")
}

// indexaFragmentos insere fragmentos com o MESMO arquivo direto no índice — é
// o formato de uma nota Tabulator (uma linha da planilha = um fragmento com
// secao "Registro N") e também de um markdown com várias seções.
func indexaFragmentos(t *testing.T, ctx *HandlerContext, arquivo string, textos []string) {
	t.Helper()
	now := time.Now().Format(time.RFC3339)
	for i, texto := range textos {
		id := arquivo + "#frag-" + strconv.Itoa(i)
		secao := "Registro " + strconv.Itoa(i+1)
		ctx.Store.InsertDocument(db.Document{
			ID:        id,
			Tipo:      "markdown",
			Arquivo:   arquivo,
			Secao:     secao,
			Texto:     texto,
			Timestamp: now,
		})
		ctx.Store.IndexFTS(id, "markdown", arquivo, secao, texto, "")
	}
}

// O card de uma nota lista TODOS os trechos que casaram (antes só o melhor
// fragmento aparecia e os outros eram descartados em silêncio); o total
// "X resultados" conta NOTAS distintas, não fragmentos.
func TestHandleSearch_TodosOsFragmentosDaNota_NoCard(t *testing.T) {
	ctx := newTestContext(t)

	// Nota A: 3 linhas com o termo; nota B: 2 linhas. Total esperado: 2 notas.
	indexaFragmentos(t, ctx, "notes/plano-a.md", []string{
		"orcamento zefrafon trimestral",
		"revisao zefrafon do contrato",
		"zefrafon arquivado em 2025",
	})
	indexaFragmentos(t, ctx, "notes/plano-b.md", []string{
		"zefrafon em teste final",
		"resultado zefrafon anterior",
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/search?q=zefrafon", nil)
	ctx.HandleSearch(rec, req)
	if rec.Code != 200 {
		t.Fatalf("esperado status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	// Os trechos trazem os spans de destaque embutidos — limpa as tags antes
	// de conferir os textos (mesmo critério do teste de integração existente).
	clean := stripTags(body)

	for _, texto := range []string{
		"orcamento zefrafon trimestral",
		"revisao zefrafon do contrato",
		"zefrafon arquivado em 2025",
		"zefrafon em teste final",
		"resultado zefrafon anterior",
	} {
		if !strings.Contains(clean, texto) {
			t.Errorf("trecho %q deveria aparecer no card da nota", texto)
		}
	}
	if !strings.Contains(body, "2 resultado") {
		t.Errorf("total deveria contar notas distintas (2), body: %s", body)
	}
	if strings.Contains(body, "nesta nota") {
		t.Error("sem excedente não deve haver a linha '+N ocorrências'")
	}
}

// Quando os trechos de UMA nota excedem o teto, os renderizados param em
// maxSnippetsPerNote e o resto vira "+N ocorrências nesta nota".
func TestHandleSearch_ExcedenteViraContador(t *testing.T) {
	ctx := newTestContext(t)

	textos := make([]string, 7)
	for i := range textos {
		textos[i] = "zefrafon linha " + strconv.Itoa(i+1)
	}
	indexaFragmentos(t, ctx, "notes/plano-grande.md", textos)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/search?q=zefrafon", nil)
	ctx.HandleSearch(rec, req)
	if rec.Code != 200 {
		t.Fatalf("esperado status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	clean := stripTags(body)

	mostrados := 0
	for _, texto := range textos {
		if strings.Contains(clean, texto) {
			mostrados++
		}
	}
	if mostrados != maxSnippetsPerNote {
		t.Errorf("esperado %d trechos renderizados, apareceram %d", maxSnippetsPerNote, mostrados)
	}
	if !strings.Contains(body, "+2 ocorrência") {
		t.Errorf("esperado '+2 ocorrência' no card, body: %s", body)
	}
}
