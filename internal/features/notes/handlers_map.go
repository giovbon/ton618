package notes

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"ton618/internal/core/domain"
	"ton618/internal/processor"
)

func (ctx *HandlerContext) HandleMap(w http.ResponseWriter, r *http.Request) {
	fileParam := r.URL.Query().Get("file")
	if fileParam == "" {
		slug := processor.GenerateCUID2()
		fileParam = fmt.Sprintf("notes/mapa-%s.md", slug)
		http.Redirect(w, r, "/map?file="+url.QueryEscape(fileParam), http.StatusFound)
		return
	}

	content, err := ctx.Store.GetNote(fileParam)
	if err != nil {
		content = "[]"
	}

	tags, _ := ctx.Store.GetFileTags(fileParam)

	if content != "" {
		fm, _, _ := ParseFrontmatter(content)
		if fm != nil {
			noteType, _ := fm["type"].(string)
			noteType = strings.ToLower(noteType)
			if noteType != "" && noteType != "map" {
				var redirect string
				switch noteType {
				case "spreadsheet", "planilha":
					redirect = "/spreadsheet?file=" + url.QueryEscape(fileParam)
				case "drawing", "desenho":
					redirect = "/drawing?file=" + url.QueryEscape(fileParam)
				case "typst":
					redirect = "/typst?file=" + url.QueryEscape(fileParam)
				case "mermaid":
					redirect = "/mermaid?file=" + url.QueryEscape(fileParam)
				case "mindmap", "markmap":
					redirect = "/mindmap?file=" + url.QueryEscape(fileParam)
				default:
					redirect = "/editor?file=" + url.QueryEscape(fileParam)
				}
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

	MapNote(data).Render(r.Context(), w)
}
