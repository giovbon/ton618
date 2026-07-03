package domain

import (
	"strings"
	)

type EditorData struct {
	Title        string
	Filename     string
	DisplayName  string
	Content      string
	Tags         []string
	AllTags      []string
	Backlinks    *BacklinksResult
}

// DisplayName extrai o nome do arquivo da rota ou caminho
func DisplayName(name string) string {
	parts := strings.Split(name, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return name
}

func NoteIcon(arquivo string, tags []string) string {
	isPdf := strings.HasPrefix(arquivo, "pdfs/")
	isAttach := strings.HasPrefix(arquivo, "attachments/")
	isArchive := strings.HasPrefix(arquivo, "archives/")
	hasTag := func(tag string) bool {
		for _, t := range tags {
			if t == tag {
				return true
			}
		}
		return false
	}
	lowerFile := strings.ToLower(arquivo)
	if isPdf {
		return "📕"
	} else if hasTag("typst") {
		return "📘"
	} else if hasTag("drawing") {
		return "🎨"
	} else if hasTag("spreadsheet") {
		return "📊"
	} else if hasTag("mermaid") {
		return "🧜"
	} else if hasTag("mindmap") || hasTag("markmap") || strings.Contains(lowerFile, "mindmap") || strings.Contains(lowerFile, "markmap") {
		return "🔱"
	} else if hasTag("map") || hasTag("mapa") || strings.Contains(lowerFile, "mapa-") || strings.Contains(lowerFile, "mapa.") || strings.HasSuffix(lowerFile, "/map") {
		return "🗺️"
	} else if hasTag("youtube") {
		return "🎬"
	} else if hasTag("artigo") {
		return "📰"
	} else if hasTag("captura") {
		return "🌐"
	} else if isAttach {
		return "📦"
	} else if isArchive {
		return "💾"
	}
	return "📄"
}
