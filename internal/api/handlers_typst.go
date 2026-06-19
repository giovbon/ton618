package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ton618/internal/processor"
	"ton618/internal/service"
	"ton618/internal/template"
)

// HandleTypst renderiza a página do editor split-pane do Typst.
func (ctx *HandlerContext) HandleTypst(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		filename = "notes/" + processor.GenerateCUID2() + ".md"
	}

	sanitized := noteFilename(filename)
	if sanitized != filename {
		canonical := "/typst?file=" + url.QueryEscape(sanitized)
		http.Redirect(w, r, canonical, http.StatusFound)
		return
	}

	var content string
	var tags []string

	if data, err := ctx.Store.GetNote(filename); err == nil && data != "" {
		content = data
		ctx.Store.IncrementPopularity(filename)
	} else {
		// Conteúdo default para uma nova nota Typst com frontmatter
		content = "---\ntype: typst\n---\n= Minha Nota Typst\n\nComece a digitar aqui..."
	}

	fileTags, err := ctx.Store.GetFileTags(filename)
	if err == nil {
		tags = fileTags
	}

	// Redireciona planilhas ou desenhos para seus respectivos editores se esta nota mudou de tipo
	if content != "" {
		isTypst := false
		isDrawing := false
		isSpreadsheet := false
		for _, t := range fileTags {
			lowerT := strings.ToLower(t)
			if lowerT == "typst" {
				isTypst = true
			} else if lowerT == "drawing" {
				isDrawing = true
			} else if lowerT == "spreadsheet" {
				isSpreadsheet = true
			}
		}
		if isSpreadsheet || strings.Contains(content, "type: spreadsheet") {
			http.Redirect(w, r, "/spreadsheet?file="+url.QueryEscape(filename), http.StatusFound)
			return
		}
		if isDrawing || strings.Contains(content, "type: drawing") {
			http.Redirect(w, r, "/drawing?file="+url.QueryEscape(filename), http.StatusFound)
			return
		}
		if !isTypst && !strings.Contains(content, "type: typst") {
			http.Redirect(w, r, "/editor?file="+url.QueryEscape(filename), http.StatusFound)
			return
		}
	}

	// Filtra as tags internas para não mostrar na UI do editor
	var userTags []string
	for _, t := range fileTags {
		lt := strings.ToLower(t)
		if lt != "spreadsheet" && lt != "drawing" && lt != "typst" {
			userTags = append(userTags, t)
		}
	}
	tags = userTags



	allTags, err := ctx.Store.GetAllTags()
	if err != nil {
		allTags = nil
	}

	backlinks, err := ctx.Notes.GetBacklinks(filename)
	if err != nil {
		backlinks = &service.BacklinksResult{}
	}

	data := template.EditorData{
		Title:       "Typst - " + filename,
		Filename:    filename,
		DisplayName: template.DisplayName(filename),
		Content:     content,
		Tags:        tags,
		AllTags:     allTags,
		Backlinks:   backlinks,
	}

	template.Typst(data).Render(r.Context(), w)
}

// HandleTypstRender compila o conteúdo Typst para SVG usando o CLI local do sistema.
func (ctx *HandlerContext) HandleTypstRender(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	content := r.FormValue("content")

	w.Header().Set("Content-Type", "application/json")

	// 1. Verifica se o executável 'typst' existe no PATH
	_, err := exec.LookPath("typst")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  "O compilador 'typst' não está instalado no servidor. Por favor, execute 'winget install Typst.Typst' (Windows) ou 'brew install typst' (Mac/Linux), ou verifique se ele está no PATH do sistema.",
		})
		return
	}

	// 2. Cria diretório temporário para a compilação
	tmpDir, err := os.MkdirTemp("", "ton618-typst-*")
	if err != nil {
		slog.Error("erro ao criar diretório temporário para o typst", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  fmt.Sprintf("Erro interno ao criar espaço de compilação: %v", err),
		})
		return
	}
	defer os.RemoveAll(tmpDir)

	// Limpa o frontmatter para compilar apenas o código Typst puro
	compileBody := content
	if _, body, err := service.ParseFrontmatter(content); err == nil {
		compileBody = body
	}
	// Garante A4 como padrão no compilador sem sujar o código da nota com regras #set
	compileBody = "#set page(paper: \"a4\")\n" + compileBody

	// 3. Grava o código Typst em um arquivo temporário
	tmpFile := filepath.Join(tmpDir, "main.typ")
	if err := os.WriteFile(tmpFile, []byte(compileBody), 0644); err != nil {
		slog.Error("erro ao escrever arquivo main.typ", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  fmt.Sprintf("Erro interno ao salvar fonte do Typst: %v", err),
		})
		return
	}

	// 4. Executa a compilação do Typst para SVG
	// Usa page-{p}.svg para suportar múltiplos SVGs se houver mais de uma página
	cmd := exec.Command("typst", "compile", "main.typ", "page-{p}.svg")
	cmd.Dir = tmpDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Retorna erro amigável de compilação (geralmente gerado pelo compilador do typst)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  stderr.String(),
		})
		return
	}

	// 5. Coleta todas as páginas geradas
	var pages []string
	for i := 1; ; i++ {
		pagePath := filepath.Join(tmpDir, fmt.Sprintf("page-%d.svg", i))
		data, err := os.ReadFile(pagePath)
		if err != nil {
			// Não existem mais páginas
			break
		}
		pages = append(pages, string(data))
	}

	// Caso nenhum arquivo tenha sido gerado (improvável se o typst retornou sucesso)
	if len(pages) == 0 {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  "Nenhuma página SVG foi gerada pelo compilador.",
		})
		return
	}

	// 6. Sucesso!
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"pages":  pages,
	})
}

// HandleTypstPDF compila o conteúdo Typst para um arquivo PDF e o retorna como download.
func (ctx *HandlerContext) HandleTypstPDF(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		http.Error(w, "parâmetro 'file' é obrigatório", http.StatusBadRequest)
		return
	}

	sanitized := noteFilename(filename)
	content, err := ctx.Store.GetNote(sanitized)
	if err != nil || content == "" {
		http.Error(w, "nota não encontrada", http.StatusNotFound)
		return
	}

	// 1. Verifica se o executável 'typst' existe no PATH
	_, err = exec.LookPath("typst")
	if err != nil {
		http.Error(w, "O compilador 'typst' não está instalado no servidor.", http.StatusInternalServerError)
		return
	}

	// 2. Limpa o frontmatter para compilar apenas o código Typst puro
	compileBody := content
	if _, body, err := service.ParseFrontmatter(content); err == nil {
		compileBody = body
	}
	// Garante A4 como padrão no compilador sem sujar o código da nota com regras #set
	compileBody = "#set page(paper: \"a4\")\n" + compileBody

	// 3. Cria diretório temporário para a compilação
	tmpDir, err := os.MkdirTemp("", "ton618-typst-pdf-*")
	if err != nil {
		slog.Error("erro ao criar diretório temporário para o typst pdf", "error", err)
		http.Error(w, fmt.Sprintf("Erro interno ao criar espaço de compilação: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)

	// 4. Grava o código Typst em um arquivo temporário
	tmpFile := filepath.Join(tmpDir, "main.typ")
	if err := os.WriteFile(tmpFile, []byte(compileBody), 0644); err != nil {
		slog.Error("erro ao escrever arquivo main.typ", "error", err)
		http.Error(w, fmt.Sprintf("Erro interno ao salvar fonte do Typst: %v", err), http.StatusInternalServerError)
		return
	}

	// 5. Executa a compilação do Typst para PDF
	pdfPath := filepath.Join(tmpDir, "output.pdf")
	cmd := exec.Command("typst", "compile", "main.typ", "output.pdf")
	cmd.Dir = tmpDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		slog.Error("erro ao compilar PDF", "error", err, "stderr", stderr.String())
		http.Error(w, fmt.Sprintf("Erro de compilação do Typst:\n%s", stderr.String()), http.StatusBadRequest)
		return
	}

	// 6. Lê o PDF gerado
	pdfBytes, err := os.ReadFile(pdfPath)
	if err != nil {
		http.Error(w, "Erro ao ler o PDF gerado", http.StatusInternalServerError)
		return
	}

	// 7. Define headers e retorna o arquivo
	displayName := template.DisplayName(sanitized)
	pdfName := strings.TrimSuffix(displayName, ".md") + ".pdf"

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", strings.ReplaceAll(pdfName, "\"", "\\\"")))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
	w.Write(pdfBytes)
}
