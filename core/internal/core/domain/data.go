package domain

import (
	"net/url"
	"path/filepath"
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
	NoteTypeImage       NoteType = "imagem"
)

// InternalTypeTags são as tags usadas para denotar o tipo do editor
// que NÃO devem ser exibidas ao usuário na interface.
var InternalTypeTags = map[string]bool{
	"typst":       true,
	"drawing":     true,
	"spreadsheet": true,
	"mermaid":     true,
	"mindmap":     true,
	"markmap":     true,
	"map":         true,
	"mapa":        true,
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

// DetectNoteType determina o tipo de editor de uma nota a partir de dados
// ESTÁVEIS e determinísticos: tags persistidas + caminho do arquivo + nome do
// arquivo. NÃO recebe conteúdo — isso garante que a mesma nota tenha SEMPRE o
// mesmo tipo (e portanto o mesmo ícone), em qualquer lugar da aplicação
// (sidebar, banco de dados, busca, embeddings).
//
// Para tipos derivados apenas do conteúdo (ex: frontmatter "type: X" sem a tag
// persistida), use DetectNoteTypeFromContent ou garanta a persistência da tag
// canônica via NoteTypeCanonicalTag (ver NoteService.EnsureTypeTags).
func DetectNoteType(tags []string, arquivo string) NoteType {
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

	ext := strings.ToLower(filepath.Ext(arquivo))
	if strings.HasPrefix(arquivo, "notes/img_") || ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".webp" || ext == ".svg" {
		return NoteTypeImage
	}

	// 3. Nome de arquivo como heurística adicional
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

// DetectNoteTypeFromContent é a variante que também considera o CONTEÚDO
// (frontmatter "type: X" ou blocos de código). Deve ser usada apenas onde o
// conteúdo já está carregado e não pode faltar (ex: decidir qual editor abrir)
// e no backfill que persiste a tag canônica. Para decidir o ícone, prefira
// sempre DetectNoteType (sem conteúdo).
func DetectNoteTypeFromContent(tags []string, content, arquivo string) NoteType {
	// Tags e caminho têm prioridade e são determinísticos.
	nt := DetectNoteType(tags, arquivo)
	if nt != NoteTypeMarkdown {
		return nt
	}

	// Fallback: apenas quando o conteúdo está disponível.
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

		if isMermaidContent(lowerContent) {
			return NoteTypeMermaid
		}
	}

	return nt
}

// NoteTypeCanonicalTag retorna a tag canônica persistida na tabela tags para um
// tipo especial de nota ("" para tipos sem tag de tipo). Usada para tornar a
// detecção de tipo estável e independente de conteúdo.
func NoteTypeCanonicalTag(t NoteType) string {
	switch t {
	case NoteTypeDrawing:
		return "drawing"
	case NoteTypeSpreadsheet:
		return "spreadsheet"
	case NoteTypeTypst:
		return "typst"
	case NoteTypeMermaid:
		return "mermaid"
	case NoteTypeMindmap:
		return "markmap"
	case NoteTypeMap:
		return "map"
	case NoteTypeYoutube:
		return "youtube"
	case NoteTypeArticle:
		return "artigo"
	case NoteTypeCapture:
		return "captura"
	}
	return ""
}

// NoteOpenTarget retorna a URL para abrir uma nota e se deve abrir em nova aba.
func NoteOpenTarget(t NoteType, arquivo string) (url string, blank bool) {
	escaped := escapeFileQuery(arquivo)
	switch t {
	case NoteTypePDF:
		return "/file?name=" + escaped, true
	case NoteTypeAttachment, NoteTypeArchive:
		return "/file/download?name=" + escaped, true
	case NoteTypeEPUB:
		return "/epub/reader?file=" + escaped, false
	default:
		return t.EditorRoute() + "?file=" + escaped, false
	}
}

// escapeFileQuery escapa um caminho de arquivo para query string mantendo as
// barras (evita bloqueio de proxies reversos e mantém o caminho legível).
func escapeFileQuery(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "%2F", "/")
}

func isMermaidContent(lowerContent string) bool {
	text := strings.TrimSpace(lowerContent)
	if strings.HasPrefix(text, "---") {
		if idx := strings.Index(text[3:], "---"); idx != -1 {
			text = strings.TrimSpace(text[idx+6:])
		}
	}
	keywords := []string{
		"gantt", "graph ", "graph\n", "graph\r", "flowchart",
		"sequencediagram", "classdiagram", "statediagram",
		"erdiagram", "pie", "gitgraph", "journey", "timeline",
		"zenuml", "architecture-beta", "kanban", "block-beta",
		"packet-beta", "c4diagram", "sankey-beta", "quadrantchart",
		"requirementdiagram",
	}
	for _, kw := range keywords {
		if strings.HasPrefix(text, kw) {
			return true
		}
	}
	return false
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

type EditorData struct {
	Title       string
	Filename    string
	DisplayName string
	Content     string
	Tags        []string
	AllTags     []string
	Backlinks   *BacklinksResult
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
	noteType := DetectNoteType(tags, arquivo)
	return icons.GetIcon(string(noteType))
}

// NoteIconColor retorna a classe Tailwind de cor sortida exclusiva para cada ícone vinda do mapa de configuração centralizado.
func NoteIconColor(iconName string) string {
	return icons.GetColor(iconName)
}

// AutoTagRule define uma regra de auto-tagging baseada na idade da nota.
type AutoTagRule struct {
	Days int    `json:"days"`
	Tag  string `json:"tag"`
}
