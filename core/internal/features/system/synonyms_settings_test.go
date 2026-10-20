package system

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// A aba "Sinônimos": GET devolve [] quando nada salvo; POST persiste; o GET
// seguinte reflete o que foi salvo (em JSON, consumido pela aba e pelo lookup
// da busca em internal/search).
func TestGetPostSynonymSettings(t *testing.T) {
	ctx := newTestContext(t)

	// GET sem nada salvo → []
	rr := httptest.NewRecorder()
	ctx.HandleGetSynonymSettings(rr, httptest.NewRequest("GET", "/api/settings/synonyms", nil))
	if rr.Body.String() != "[]" {
		t.Fatalf("esperava [], got %s", rr.Body.String())
	}

	// POST salva as regras (termo + sinônimos).
	body := `[{"termo":"pc","sinonimos":["computador","maquina"]},{"termo":"ideia","sinonimos":["conceito"]}]`
	rr2 := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/settings/synonyms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.HandlePostSynonymSettings(rr2, req)
	if rr2.Code != 200 {
		t.Fatalf("status %d, body: %s", rr2.Code, rr2.Body.String())
	}

	// GET reflete o salvo.
	rr3 := httptest.NewRecorder()
	ctx.HandleGetSynonymSettings(rr3, httptest.NewRequest("GET", "/api/settings/synonyms", nil))
	got := rr3.Body.String()
	for _, want := range []string{"\"termo\":\"pc\"", "computador", "ideia", "conceito"} {
		if !strings.Contains(got, want) {
			t.Errorf("GET deveria conter %q, got: %s", want, got)
		}
	}
}
