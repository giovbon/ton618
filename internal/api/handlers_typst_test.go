package api

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
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

	bodyStr := rr.Body.String()

	if hasTypst {
		// Se o Typst estiver instalado na máquina que roda o teste
		if !strings.Contains(bodyStr, "typst-page") && !strings.Contains(bodyStr, "bg-red-950") {
			t.Errorf("Esperado páginas do Typst ou erro de compilação, obtido: %q", bodyStr)
		}
	} else {
		// Se o Typst NÃO estiver instalado
		if !strings.Contains(bodyStr, "não está instalado") {
			t.Errorf("Esperado erro sobre a falta do typst, obtido: %q", bodyStr)
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

func TestPreprocessTypstImages(t *testing.T) {
	// 1. Cria um servidor HTTP de teste para simular uma imagem remota
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("fake-image-bytes"))
	}))
	defer ts.Close()

	// 2. Prepara um diretório temporário para a compilação
	tmpDir, err := os.MkdirTemp("", "test-preprocess-*")
	if err != nil {
		t.Fatalf("erro ao criar dir temporario: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 3. Monta o conteúdo Typst usando a URL do servidor de teste
	imageURL := ts.URL + "/test.png"
	content := `= Titulo
Aqui esta uma imagem: #image("` + imageURL + `", width: 50%)
E outra chamada de imagem simples: image('` + imageURL + `')`

	// 4. Executa o pré-processamento
	newContent := preprocessTypstImages(content, tmpDir)

	// 5. Verifica se as URLs foram substituídas por nomes de arquivos locais relativos
	if strings.Contains(newContent, imageURL) {
		t.Errorf("A URL da imagem nao deveria mais estar no conteudo. Conteudo obtido: %s", newContent)
	}

	// 6. Verifica se o arquivo correspondente foi gravado no diretório temporário
	hash := sha256.Sum256([]byte(imageURL))
	expectedFilename := fmt.Sprintf("%x.png", hash)
	localPath := filepath.Join(tmpDir, expectedFilename)

	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		t.Errorf("O arquivo local %s nao foi criado no diretorio temporario", localPath)
	}

	// Lê e verifica o conteúdo do arquivo local
	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("erro ao ler arquivo local: %v", err)
	}
	if string(data) != "fake-image-bytes" {
		t.Errorf("Esperado conteudo do arquivo 'fake-image-bytes', obtido '%s'", string(data))
	}
}
