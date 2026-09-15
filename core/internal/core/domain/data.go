package domain

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"ton618/core/internal/ui/icons"
)

// ── NoteType ──

// NoteType representa o tipo de editor de uma nota.
type NoteType string

const (
	NoteTypeMarkdown   NoteType = "nota"
	NoteTypeDrawing    NoteType = "desenho"
	NoteTypeMindmap    NoteType = "markmap"
	NoteTypeYoutube    NoteType = "youtube"
	NoteTypeArticle    NoteType = "artigo"
	NoteTypeCapture    NoteType = "captura"
	NoteTypeSemanal    NoteType = "semanal"
	NoteTypePDF        NoteType = "pdf"
	NoteTypeAttachment NoteType = "anexo"
	NoteTypeArchive    NoteType = "arquivo"
	NoteTypeEPUB       NoteType = "epub"
	NoteTypeImage      NoteType = "imagem"
)

// ── Nota Semanal ──
//
// A nota semanal é uma nota markdown comum, porém com nome DETERMINÍSTICO
// derivado da semana ISO-8601: "notes/<ano>-S<semana>.md" (ex: notes/2026-S38.md).
// Como o nome é estável dentro da semana, abrir o recurso várias vezes sempre
// reusa a MESMA nota — o usuário cria no máximo uma nota por semana.
//
// O tipo é detectado SEMPRE pelo nome do arquivo (heurística determinística),
// de modo que a nota já nasce com o ícone/tipo corretos antes mesmo do primeiro
// save. A tag canônica "semanal" também é aceita (ver NoteTypeCanonicalTag).

// weeklyNoteRegex reconhece o nome canônico de uma nota semanal (sem prefixo de
// diretório e sem extensão), ex: "2026-S38".
var weeklyNoteRegex = regexp.MustCompile(`^(\d{4})-S(\d{2})$`)

// WeeklyNoteFilename retorna o nome canônico da nota semanal que contém a data
// informada: "notes/<ano>-S<semana>.md". Usa o ano-semana ISO-8601 (ISOWeek),
// que é o único que garante 1..53 semanas consistentes com a segunda-feira.
func WeeklyNoteFilename(t time.Time) string {
	year, week := t.ISOWeek()
	return fmt.Sprintf("notes/%d-S%02d.md", year, week)
}

// WeeklyNoteEmptyContent é o conteúdo inicial da nota semanal recém-criada: uma
// nota EM BRANCO — sem frontmatter, sem título e sem tag.
//
// É apenas uma quebra de linha porque NoteService.Save recusa conteúdo vazio
// ("processAndSave: conteúdo vazio"). Essa proteção impede sobrescrever notas com
// conteúdo em branco e NÃO deve ser afrouxada só por causa da nota semanal.
//
// Por que sem título e sem tag:
//   - a nota nasce em branco (o nome exibido vem do próprio nome do arquivo);
//   - o tipo "semanal" é derivado do NOME do arquivo (IsWeeklyNoteFilename), então
//     persistir a tag canônica é redundante — ver NoteTypeCanonicalTag.
const WeeklyNoteEmptyContent = "\n"

// IsWeeklyNoteFilename informa se o nome/path de arquivo segue o padrão de nota
// semanal ("notes/2026-S38.md" ou "2026-S38.md"), validando a faixa da semana.
func IsWeeklyNoteFilename(name string) bool {
	base := strings.TrimPrefix(name, "notes/")
	base = strings.TrimSuffix(base, ".md")
	m := weeklyNoteRegex.FindStringSubmatch(base)
	if m == nil {
		return false
	}
	week, err := strconv.Atoi(m[2])
	if err != nil {
		return false
	}
	return week >= 1 && week <= 53
}

// InternalTypeTags são as tags usadas para denotar o tipo do editor
// que NÃO devem ser exibidas ao usuário na interface.
var InternalTypeTags = map[string]bool{
	"drawing": true,
	"markmap": true,
	"mindmap": true,
}

// EditorRoute retorna a rota de URL do editor correto para este tipo de nota.
func (t NoteType) EditorRoute() string {
	switch t {
	case NoteTypeDrawing:
		return "/drawing"
	case NoteTypeMindmap:
		return "/mindmap"
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
		case "markmap", "mindmap":
			return NoteTypeMindmap
		case "youtube":
			return NoteTypeYoutube
		case "artigo", "article":
			return NoteTypeArticle
		case "captura", "capture":
			return NoteTypeCapture
		case "semanal", "semana", "weekly":
			return NoteTypeSemanal
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
	if IsWeeklyNoteFilename(arquivo) {
		return NoteTypeSemanal
	}
	lowerFile := strings.ToLower(arquivo)
	if strings.Contains(lowerFile, "mindmap") || strings.Contains(lowerFile, "markmap") {
		return NoteTypeMindmap
	}
	if strings.Contains(lowerFile, "drawing") || strings.Contains(lowerFile, "desenho") {
		return NoteTypeDrawing
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
		if strings.Contains(lowerContent, "type: markmap") || strings.Contains(lowerContent, "type: mindmap") ||
			strings.Contains(lowerContent, "```markmap") || strings.Contains(lowerContent, "# markmap") ||
			strings.Contains(lowerContent, "# mindmap") {
			return NoteTypeMindmap
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
	case NoteTypeMindmap:
		return "markmap"
	case NoteTypeYoutube:
		return "youtube"
	case NoteTypeArticle:
		return "artigo"
	case NoteTypeCapture:
		return "captura"
	case NoteTypeSemanal:
		return "semanal"
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

// DisplayName extrai o nome do arquivo da rota ou caminho.
// Remove o prefixo interno "captura-" (usado apenas na geração do nome da
// captura) para que a exibição seja uniforme em todos os lugares — editor,
// sidebar, banco de dados, backlinks e busca. O sufixo ".md" é mantido.
func DisplayName(name string) string {
	parts := strings.Split(name, "/")
	base := name
	if len(parts) > 0 {
		base = parts[len(parts)-1]
	}
	return strings.TrimPrefix(base, "captura-")
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
