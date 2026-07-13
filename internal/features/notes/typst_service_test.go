package notes

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewTypstService(t *testing.T) {
	service := NewTypstService()
	if service == nil {
		t.Fatal("NewTypstService retornou nil")
	}
}

func TestTypstService_CheckAvailability(t *testing.T) {
	service := NewTypstService()
	// CheckAvailability não deve causar pânico
	_ = service.CheckAvailability()
}

func TestTypstService_ResolvePath(t *testing.T) {
	service := NewTypstService()
	resolved := service.resolvePath()
	if resolved == "" {
		t.Error("resolvePath retornou string vazia")
	}
}

func TestTypstService_PrepareContent(t *testing.T) {
	service := NewTypstService()

	// Caso 1: Sem frontmatter
	content1 := "= Titulo\nConteudo da nota"
	expected1 := "#set page(paper: \"a4\")\n= Titulo\nConteudo da nota"
	res1 := service.prepareContent(content1)
	if res1 != expected1 {
		t.Errorf("prepareContent falhou para conteúdo sem frontmatter.\nObtido: %q\nEsperado: %q", res1, expected1)
	}

	// Caso 2: Com frontmatter
	content2 := `---
type: typst
title: Meu Documento
---
= Titulo com Frontmatter
Outro conteudo.`
	expected2 := `#set page(paper: "a4")
= Titulo com Frontmatter
Outro conteudo.`
	res2 := service.prepareContent(content2)
	if res2 != expected2 {
		t.Errorf("prepareContent falhou para conteúdo com frontmatter.\nObtido: %q\nEsperado: %q", res2, expected2)
	}
}

func TestTypstService_RenderToSVG(t *testing.T) {
	service := NewTypstService()
	if err := service.CheckAvailability(); err != nil {
		t.Skip("Typst não está instalado no servidor, pulando teste de renderização")
	}

	// Caso 1: Renderização válida
	content := "= Titulo de Teste\nOlá Mundo!"
	res := service.RenderToSVG(content)
	if res.Error != "" {
		t.Errorf("RenderToSVG falhou com erro: %s", res.Error)
	}
	if len(res.Pages) == 0 {
		t.Error("RenderToSVG não gerou nenhuma página")
	} else {
		// Verifica se a página contém tag SVG básica
		if !strings.Contains(res.Pages[0], "<svg") {
			t.Errorf("Página renderizada não parece conter código SVG: %s", res.Pages[0])
		}
	}

	// Caso 2: Renderização inválida (erro de sintaxe)
	// Typst inválido (por exemplo, usar função inexistente)
	badContent := "#funcaoInexistente()"
	resBad := service.RenderToSVG(badContent)
	if resBad.Error == "" {
		t.Error("Esperava erro de compilação para sintaxe inválida, mas obteve sucesso")
	}
}

func TestTypstService_RenderToPDF(t *testing.T) {
	service := NewTypstService()
	if err := service.CheckAvailability(); err != nil {
		t.Skip("Typst não está instalado no servidor, pulando teste de renderização PDF")
	}

	// Caso 1: Renderização PDF válida
	content := "= Titulo PDF\nConteúdo para PDF."
	pdfData, err := service.RenderToPDF(content)
	if err != nil {
		t.Fatalf("RenderToPDF falhou com erro: %v", err)
	}
	if len(pdfData) == 0 {
		t.Fatal("RenderToPDF gerou arquivo de 0 bytes")
	}
	// Verifica cabeçalho PDF clássico %PDF-
	if !strings.HasPrefix(string(pdfData), "%PDF-") {
		t.Errorf("Dados gerados não possuem assinatura de arquivo PDF válida")
	}

	// Caso 2: Renderização inválida
	badContent := "#outraFuncaoInexistente()"
	_, errBad := service.RenderToPDF(badContent)
	if errBad == nil {
		t.Error("Esperava erro ao compilar Typst inválido para PDF")
	}
}

func TestTypstService_PreprocessTypstImages_Extended(t *testing.T) {
	// Servidor HTTP simulando imagens remotas
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/404.png") {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("image-data-mock"))
	}))
	defer ts.Close()

	tmpDir, err := os.MkdirTemp("", "test-typst-images-*")
	if err != nil {
		t.Fatalf("erro ao criar dir temporario: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	imageURL1 := ts.URL + "/img1.png"
	imageURL2 := ts.URL + "/img2.jpg"
	imageURL404 := ts.URL + "/404.png"

	// Conteúdo misturando diferentes sintaxes
	content := fmt.Sprintf(`
= Imagens
1. #image("%s")
2. image('%s')
3. #image("%s", width: 40%%)
4. #image("local_image.png")
5. #image("%s")
`, imageURL1, imageURL2, imageURL1, imageURL404)

	newContent := preprocessTypstImages(content, tmpDir)

	// Verificações das URLs substituídas
	hash1 := sha256.Sum256([]byte(imageURL1))
	expectedName1 := fmt.Sprintf("%x.png", hash1)

	hash2 := sha256.Sum256([]byte(imageURL2))
	expectedName2 := fmt.Sprintf("%x.jpg", hash2)

	if !strings.Contains(newContent, expectedName1) {
		t.Errorf("Imagem 1 não foi pré-processada corretamente. Esperado nome local %s no conteúdo.", expectedName1)
	}
	if !strings.Contains(newContent, expectedName2) {
		t.Errorf("Imagem 2 não foi pré-processada corretamente. Esperado nome local %s no conteúdo.", expectedName2)
	}
	if strings.Contains(newContent, imageURL1) || strings.Contains(newContent, imageURL2) {
		t.Error("URLs remotas ainda estão presentes no conteúdo pré-processado")
	}

	// Caminho local de imagem não deve ser modificado
	if !strings.Contains(newContent, `image("local_image.png")`) {
		t.Error("Caminho de imagem local foi alterado incorretamente")
	}

	// Imagem 404 falhará no download, logo a URL remota deve ser mantida
	if !strings.Contains(newContent, imageURL404) {
		t.Error("Imagem com erro 404 foi incorretamente alterada no conteúdo")
	}

	// Verifica se os arquivos foram salvos localmente
	localPath1 := filepath.Join(tmpDir, expectedName1)
	if data, err := os.ReadFile(localPath1); err != nil {
		t.Errorf("Falha ao ler arquivo local da Imagem 1: %v", err)
	} else if string(data) != "image-data-mock" {
		t.Errorf("Conteúdo do arquivo local da Imagem 1 inválido: %s", string(data))
	}

	localPath2 := filepath.Join(tmpDir, expectedName2)
	if _, err := os.Stat(localPath2); os.IsNotExist(err) {
		t.Error("Arquivo local da Imagem 2 não foi criado")
	}
}
