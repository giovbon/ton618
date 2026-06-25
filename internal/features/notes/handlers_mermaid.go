package notes

import (
	"ton618/internal/core/domain"

	"net/http"
	"net/url"
	"strings"
	"ton618/internal/processor"
		)

// HandleMermaid renderiza a página do editor split-pane do Mermaid.
func (ctx *HandlerContext) HandleMermaid(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		filename = "notes/" + processor.GenerateCUID2() + ".md"
	}

	sanitized := NoteFilename(filename)
	if sanitized != filename {
		canonical := "/mermaid?file=" + url.QueryEscape(sanitized)
		http.Redirect(w, r, canonical, http.StatusFound)
		return
	}

	var content string
	var tags []string

	if data, err := ctx.Store.GetNote(filename); err == nil && data != "" {
		content = data
		ctx.Store.IncrementPopularity(filename)
	} else {
		// Conteúdo default para uma nova nota Mermaid com frontmatter
		content = "---\ntype: mermaid\n---\ngraph TD\n    A[Início] --> B(Processamento)\n    B --> C{Decisão}\n    C -->|Sim| D[Resultado 1]\n    C -->|Não| E[Resultado 2]"
	}

	fileTags, err := ctx.Store.GetFileTags(filename)
	if err == nil {
		tags = fileTags
	}

	// Redireciona planilhas, desenhos, typst ou editor para seus respectivos editores se esta nota mudou de tipo
	if content != "" {
		isMermaid := false
		isTypst := false
		isDrawing := false
		isSpreadsheet := false
		for _, t := range fileTags {
			lowerT := strings.ToLower(t)
			if lowerT == "mermaid" {
				isMermaid = true
			} else if lowerT == "typst" {
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
		if isTypst || strings.Contains(content, "type: typst") {
			http.Redirect(w, r, "/typst?file="+url.QueryEscape(filename), http.StatusFound)
			return
		}
		if !isMermaid && !strings.Contains(content, "type: mermaid") {
			http.Redirect(w, r, "/editor?file="+url.QueryEscape(filename), http.StatusFound)
			return
		}
	}

	// Filtra as tags internas para não mostrar na UI do editor
	var userTags []string
	for _, t := range fileTags {
		lt := strings.ToLower(t)
		if lt != "spreadsheet" && lt != "drawing" && lt != "typst" && lt != "mermaid" {
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
		backlinks = &domain.BacklinksResult{}
	}

	data := domain.EditorData{
		Title:       "Mermaid - " + filename,
		Filename:    filename,
		DisplayName: domain.DisplayName(filename),
		Content:     content,
		Tags:        tags,
		AllTags:     allTags,
		Backlinks:   backlinks,
	}

	Mermaid(data).Render(r.Context(), w)
}
