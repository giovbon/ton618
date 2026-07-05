package notes

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"ton618/internal/core/domain"
)

// HandleTypst renderiza a página do editor Typst.
func (ctx *HandlerContext) HandleTypst(w http.ResponseWriter, r *http.Request) {
	fileParam := r.URL.Query().Get("file")
	if fileParam == "" {
		http.Error(w, "file parameter required", http.StatusBadRequest)
		return
	}

	content, err := ctx.Store.GetNote(fileParam)
	if err != nil {
		content = `---
type: typst
---

= Titulo

Escreva seu conteudo Typst aqui.`
	}

	tags, _ := ctx.Store.GetFileTags(fileParam)

	var userTags []string
	for _, t := range tags {
		t = strings.ToLower(t)
		if t != "typst" && t != "drawing" && t != "spreadsheet" && t != "mermaid" && t != "mindmap" && t != "markmap" && t != "keywords" {
			userTags = append(userTags, t)
		}
	}

	typstType := false
	for _, t := range tags {
		if strings.ToLower(t) == "typst" {
			typstType = true
			break
		}
	}

	if !typstType && content != "" {
		frontmatter, _, _ := ParseFrontmatter(content)
		if frontmatter != nil {
			noteType, _ := frontmatter["type"].(string)
			noteType = strings.ToLower(noteType)
			var redirect string
			switch noteType {
			case "spreadsheet", "planilha":
				redirect = "/spreadsheet?file=" + url.QueryEscape(fileParam)
			case "drawing", "desenho":
				redirect = "/drawing?file=" + url.QueryEscape(fileParam)
			case "mermaid":
				redirect = "/mermaid?file=" + url.QueryEscape(fileParam)
			case "mindmap", "markmap":
				redirect = "/mindmap?file=" + url.QueryEscape(fileParam)
			default:
				redirect = "/editor?file=" + url.QueryEscape(fileParam)
			}
			if redirect != "" {
				http.Redirect(w, r, redirect, http.StatusFound)
				return
			}
		}
	}

	allTags, _ := ctx.Store.GetAllTags()
	displayName := domain.DisplayName(fileParam)

	data := domain.EditorData{
		Title:       displayName + " — TON-618",
		Filename:    fileParam,
		DisplayName: displayName,
		Content:     content,
		Tags:        tags,
		AllTags:     allTags,
	}

	Typst(data).Render(r.Context(), w)
}

// HandleTypstRender compila o conteúdo Typst para SVG e retorna HTML.
func (ctx *HandlerContext) HandleTypstRender(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	content := r.FormValue("content")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	result := ctx.Typst.RenderToSVG(content)
	if result.Error != "" {
		w.Write([]byte(`<div class="w-full bg-red-950/85 border border-red-800/60 text-red-300 px-4 py-3 rounded-lg font-mono text-xs whitespace-pre-wrap shrink-0 overflow-x-auto">` + result.Error + `</div>`))
		return
	}

	var finalHTML strings.Builder
	for _, page := range result.Pages {
		finalHTML.WriteString(`<div class="typst-page">`)
		finalHTML.WriteString(page)
		finalHTML.WriteString(`</div>`)
	}
	w.Write([]byte(finalHTML.String()))
}

// HandleTypstPDF compila o conteúdo Typst para PDF e retorna como download.
func (ctx *HandlerContext) HandleTypstPDF(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		http.Error(w, "parâmetro 'file' é obrigatório", http.StatusBadRequest)
		return
	}

	content, err := ctx.Store.GetNote(filename)
	if err != nil || !ctx.Store.NoteExists(filename) {
		http.Error(w, "Nota não encontrada", http.StatusNotFound)
		return
	}

	pdfData, err := ctx.Typst.RenderToPDF(content)
	if err != nil {
		http.Error(w, fmt.Sprintf("Erro ao compilar PDF: %v", err), http.StatusInternalServerError)
		return
	}

	cleanName := strings.TrimSuffix(strings.TrimPrefix(filename, "notes/"), ".md")
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.pdf"`, cleanName))
	w.Write(pdfData)
}
