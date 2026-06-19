package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"strings"
	"testing"
)

func TestHandleTypst(t *testing.T) {
	ctx := newTestContext(t)

	// Caso 1: Acessar nota Typst que ainda não existe
	req := httptest.NewRequest("GET", "/typst?file=notes/nova-nota.md", nil)
	rr := httptest.NewRecorder()
	ctx.HandleTypst(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("HandleTypst retornou status %d, esperado %d", rr.Code, http.StatusOK)
	}

	// Caso 2: Acessar nota Typst existente
	saveTestNote(t, ctx, "notes/minha-nota-typst.md", "---\ntype: typst\n---\n= Ola", "typst")
	req2 := httptest.NewRequest("GET", "/typst?file=notes/minha-nota-typst.md", nil)
	rr2 := httptest.NewRecorder()
	ctx.HandleTypst(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("HandleTypst retornou status %d para nota existente, esperado %d", rr2.Code, http.StatusOK)
	}
}

func TestHandleTypstRender(t *testing.T) {
	ctx := newTestContext(t)

	// Testa comportamento quando o compilador 'typst' pode ou não estar instalado
	_, err := exec.LookPath("typst")
	hasTypst := err == nil

	formData := url.Values{}
	formData.Set("content", "= Ola do Typst")

	req := httptest.NewRequest("POST", "/api/notes/render-typst", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	ctx.HandleTypstRender(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("HandleTypstRender retornou status %d, esperado %d", rr.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Erro ao decodificar JSON: %v", err)
	}

	if hasTypst {
		// Se o Typst estiver instalado na máquina que roda o teste
		if resp["status"] != "success" {
			t.Errorf("Esperado status 'success', obtido '%v'. Erro: %v", resp["status"], resp["error"])
		}
		pages, ok := resp["pages"].([]interface{})
		if !ok || len(pages) == 0 {
			t.Errorf("Esperado array de páginas não vazio, obtido: %v", resp["pages"])
		}
	} else {
		// Se o Typst NÃO estiver instalado
		if resp["status"] != "error" {
			t.Errorf("Esperado status 'error' devido a falta do Typst, obtido '%v'", resp["status"])
		}
		errStr, ok := resp["error"].(string)
		if !ok || !strings.Contains(errStr, "não está instalado") {
			t.Errorf("Esperado erro sobre a falta do typst, obtido: %v", resp["error"])
		}
	}
}

func TestHandleTypstPDF(t *testing.T) {
	ctx := newTestContext(t)

	// Caso 1: Testar com arquivo inexistente
	req := httptest.NewRequest("GET", "/api/notes/download-typst-pdf?file=notes/nao-existe.md", nil)
	rr := httptest.NewRecorder()
	ctx.HandleTypstPDF(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("Esperado status %d para arquivo inexistente, obtido %d", http.StatusNotFound, rr.Code)
	}

	// Caso 2: Criar arquivo e testar
	saveTestNote(t, ctx, "notes/relatorio.md", "= Meu Relatório", "typst")
	req2 := httptest.NewRequest("GET", "/api/notes/download-typst-pdf?file=notes/relatorio.md", nil)
	rr2 := httptest.NewRecorder()
	ctx.HandleTypstPDF(rr2, req2)

	_, err := exec.LookPath("typst")
	hasTypst := err == nil

	if hasTypst {
		if rr2.Code != http.StatusOK {
			t.Errorf("Esperado status %d com typst instalado, obtido %d", http.StatusOK, rr2.Code)
		}
		ct := rr2.Header().Get("Content-Type")
		if ct != "application/pdf" {
			t.Errorf("Esperado Content-Type 'application/pdf', obtido '%s'", ct)
		}
		cd := rr2.Header().Get("Content-Disposition")
		if !strings.Contains(cd, "filename=\"relatorio.pdf\"") {
			t.Errorf("Esperado Content-Disposition contendo 'filename=\"relatorio.pdf\"', obtido '%s'", cd)
		}
	} else {
		// Sem typst, deve retornar erro
		if rr2.Code != http.StatusInternalServerError {
			t.Errorf("Esperado status %d (Internal Server Error) sem typst instalado, obtido %d", http.StatusInternalServerError, rr2.Code)
		}
	}
}
