package domain

import (
	"strings"
	"ton618/core/internal/ui/icons"
)

// ── NoteType ──

// NoteType representa o tipo de editor de uma nota.
type NoteType string

const (
	NoteTypeMarkdown    NoteType = "nota"
	NoteTypeDrawing     NoteType = "desenho"
	NoteTypeSpreadsheet NoteType = "planilha"
	NoteTypeTypst       NoteType = "typst"
	NoteTypeMermaid     NoteType = "mermaid"
	NoteTypeMindmap     NoteType = "markmap"
	NoteTypeMap         NoteType = "mapa"
	NoteTypeYoutube     NoteType = "youtube"
	NoteTypeArticle     NoteType = "artigo"
	NoteTypeCapture     NoteType = "captura"
	NoteTypePDF         NoteType = "pdf"
	NoteTypeAttachment  NoteType = "anexo"
	NoteTypeArchive     NoteType = "arquivo"
	NoteTypeEPUB        NoteType = "epub"
)

// InternalTypeTags são as tags usadas para denotar o tipo do editor
// que NÃO devem ser exibidas ao usuário na interface.
var InternalTypeTags = map[string]bool{
	"typst":      true,
	"drawing":    true,
	"spreadsheet": true,
	"mermaid":    true,
	"mindmap":    true,
	"markmap":    true,
	"map":        true,
	"mapa":       true,
}

// EditorRoute retorna a rota de URL do editor correto para este tipo de nota.
func (t NoteType) EditorRoute() string {
	switch t {
	case NoteTypeDrawing:
		return "/drawing"
	case NoteTypeSpreadsheet:
		return "/spreadsheet"
	case NoteTypeTypst:
		return "/typst"
	case NoteTypeMermaid:
		return "/mermaid"
	case NoteTypeMindmap:
		return "/mindmap"
	case NoteTypeMap:
		return "/map"
	default:
		return "/editor"
	}
}

// DetectNoteType determina o tipo de editor de uma nota a partir de suas tags,
// conteúdo e caminho de arquivo. Esta é a fonte de verdade única para detecção de tipo.
func DetectNoteType(tags []string, content, arquivo string) NoteType {
	// 1. Tags têm prioridade máxima (são explicitamente definidas pelo usuário/editor)
	for _, t := range tags {
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "drawing", "desenho":
			return NoteTypeDrawing
		case "spreadsheet", "planilha":
			return NoteTypeSpreadsheet
		case "typst":
			return NoteTypeTypst
		case "mermaid":
			return NoteTypeMermaid
		case "mindmap", "markmap":
			return NoteTypeMindmap
		case "map", "mapa":
			return NoteTypeMap
		case "youtube":
			return NoteTypeYoutube
		case "artigo", "article":
			return NoteTypeArticle
		case "captura", "capture":
			return NoteTypeCapture
		}
	}

	// 2. Prefixo de caminho para tipos de arquivo especiais
	if strings.HasPrefix(arquivo, "pdfs/") {
		return NoteTypePDF
	}
	if strings.HasPrefix(arquivo, "attachments/") {
		return NoteTypeAttachment
	}
	if strings.HasPrefix(arquivo, "archives/") {
		return NoteTypeArchive
	}
	if strings.HasPrefix(arquivo, "epubs/") || strings.HasSuffix(strings.ToLower(arquivo), ".epub") {
		return NoteTypeEPUB
	}

	// 3. Conteúdo frontmatter (type: X ou marcas de código)
	if content != "" {
		lowerContent := strings.ToLower(content)
		if strings.Contains(lowerContent, "type: drawing") || strings.Contains(lowerContent, "type: desenho") {
			return NoteTypeDrawing
		}
		if strings.Contains(lowerContent, "type: spreadsheet") || strings.Contains(lowerContent, "type: planilha") {
			return NoteTypeSpreadsheet
		}
		if strings.Contains(lowerContent, "type: typst") {
			return NoteTypeTypst
		}
		if strings.Contains(lowerContent, "type: mermaid") || strings.Contains(lowerContent, "```mermaid") {
			return NoteTypeMermaid
		}
		if strings.Contains(lowerContent, "type: mindmap") || strings.Contains(lowerContent, "type: markmap") ||
			strings.Contains(lowerContent, "```markmap") || strings.Contains(lowerContent, "--- markmap") ||
			strings.Contains(lowerContent, "# markmap") || strings.Contains(lowerContent, "# mindmap") {
			return NoteTypeMindmap
		}
		if strings.Contains(lowerContent, "type: map") || strings.Contains(lowerContent, "type: mapa") {
			return NoteTypeMap
		}
	}

	// 4. Nome de arquivo como heurística adicional
	lowerFile := strings.ToLower(arquivo)
	if strings.Contains(lowerFile, "mindmap") || strings.Contains(lowerFile, "markmap") {
		return NoteTypeMindmap
	}
	if strings.Contains(lowerFile, "drawing") || strings.Contains(lowerFile, "desenho") {
		return NoteTypeDrawing
	}
	if strings.Contains(lowerFile, "spreadsheet") || strings.Contains(lowerFile, "planilha") {
		return NoteTypeSpreadsheet
	}
	if strings.Contains(lowerFile, "typst") {
		return NoteTypeTypst
	}
	if strings.Contains(lowerFile, "mermaid") {
		return NoteTypeMermaid
	}
	if strings.Contains(lowerFile, "mapa-") || strings.Contains(lowerFile, "mapa.") || strings.HasSuffix(lowerFile, "/map") || strings.Contains(lowerFile, "map-") {
		return NoteTypeMap
	}

	return NoteTypeMarkdown
}

// FilterUserTags remove as tags internas de tipo de editor de uma lista de tags,
// retornando apenas as tags que devem ser exibidas ao usuário.
func FilterUserTags(tags []string) []string {
	var result []string
	for _, t := range tags {
		if !InternalTypeTags[strings.ToLower(t)] {
			result = append(result, t)
		}
	}
	return result
}

// ── EditorData ──

type SimilarNoteItem struct {
	Filename    string
	DisplayName string
	Percentage  int
}

type EditorData struct {
	Title        string
	Filename     string
	DisplayName  string
	Content      string
	Tags         []string
	AllTags      []string
	Backlinks    *BacklinksResult
	SimilarNotes []SimilarNoteItem
}

// DisplayName extrai o nome do arquivo da rota ou caminho
func DisplayName(name string) string {
	parts := strings.Split(name, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return name
}

// AllowedFilePrefixes são os prefixos de diretório permitidos para acesso via API de arquivos.
var AllowedFilePrefixes = []string{"notes/", "pdfs/", "attachments/", "archives/", "epubs/"}

// NoteIcon retorna o nome do ícone Lucide correspondente ao tipo de nota vindo do mapa de configuração centralizado.
func NoteIcon(arquivo string, tags []string) string {
	noteType := DetectNoteType(tags, "", arquivo)
	return icons.GetIcon(string(noteType))
}

// NoteIconColor retorna a classe Tailwind de cor sortida exclusiva para cada ícone vinda do mapa de configuração centralizado.
func NoteIconColor(iconName string) string {
	return icons.GetColor(iconName)
}
