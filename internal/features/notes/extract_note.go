package notes

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"ton618/core/internal/core/domain"
	"ton618/core/internal/extract"
)

// ── Extração de texto de PDF/EPUB para uma nota ────────────────────────
//
// O usuário pede a extração explicitamente (ação no Acervo ou no leitor de
// EPUB): nada é criado automaticamente no upload. O texto extraído entra numa
// nota markdown comum — com link para o documento original no corpo e o campo
// `fonte:` no frontmatter (chave usada na deduplicação: reextrair o MESMO
// arquivo sobrescreve a nota em vez de criar outra).

const (
	// extractTag identifica as notas geradas por extração (filtrável na busca
	// e no Acervo). Não é tag de TIPO: a nota continua sendo uma nota comum.
	extractTag = "texto-extraido"

	// extractSourceKey é a propriedade do frontmatter que guarda o caminho do
	// documento de origem (ex: pdfs/livro.pdf).
	extractSourceKey = "fonte"

	// maxExtractNoteAttempts evita loop infinito se houver muitas notas
	// homônimas que não são nossas.
	maxExtractNoteAttempts = 50
)

// ErrDocumentNotFound é devolvido quando o arquivo de origem não existe no docs/.
var ErrDocumentNotFound = errors.New("documento não encontrado")

// ErrInvalidDocument é devolvido para caminhos/extensões que não podem ser
// extraídos (erro do pedido, não do servidor).
var ErrInvalidDocument = errors.New("documento inválido")

// ExtractNoteResult descreve a nota criada/atualizada pela extração.
type ExtractNoteResult struct {
	Note      string `json:"note"`
	URL       string `json:"url"`
	Chars     int    `json:"chars"`
	Chapters  int    `json:"chapters"`
	Truncated bool   `json:"truncated"`
}

// ExtractDocumentText extrai o texto de um PDF/EPUB do docs/ e grava (ou
// atualiza) a nota correspondente.
func (ctx *HandlerContext) ExtractDocumentText(c context.Context, relPath string) (*ExtractNoteResult, error) {
	source, err := validateExtractableDocument(relPath)
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join(ctx.Cfg.DocsDir, filepath.FromSlash(source))
	if !isInsideDir(ctx.Cfg.DocsDir, fullPath) {
		return nil, fmt.Errorf("%w: caminho inválido: %s", ErrInvalidDocument, relPath)
	}
	if _, err := os.Stat(fullPath); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrDocumentNotFound
		}
		return nil, err
	}

	extracted, err := extract.File(c, fullPath)
	if err != nil {
		return nil, err
	}

	title := strings.TrimSpace(extracted.Title)
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
	}

	content := buildExtractNoteMarkdown(source, title, extracted, time.Now())
	noteName, err := ctx.extractNoteName(source)
	if err != nil {
		return nil, err
	}
	if err := ctx.Notes.SaveWithContext(c, noteName, content, nil); err != nil {
		return nil, fmt.Errorf("salvar nota extraída: %w", err)
	}

	openURL, _ := domain.NoteOpenTarget(domain.NoteTypeMarkdown, noteName)
	return &ExtractNoteResult{
		Note:      noteName,
		URL:       openURL,
		Chars:     extracted.Chars(),
		Chapters:  len(extracted.Chapters),
		Truncated: extracted.Truncated,
	}, nil
}

// validateExtractableDocument normaliza e valida o caminho relativo ao docs/.
// Só aceita pdfs/*.pdf e epubs/*.epub — a extração não é um leitor genérico.
func validateExtractableDocument(raw string) (string, error) {
	p := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if p == "" {
		return "", fmt.Errorf("%w: informe o arquivo do documento", ErrInvalidDocument)
	}

	clean := path.Clean(p)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
		return "", fmt.Errorf("%w: caminho inválido: %s", ErrInvalidDocument, raw)
	}

	lower := strings.ToLower(clean)
	switch {
	case strings.HasPrefix(clean, domain.PDFPrefix) && strings.HasSuffix(lower, ".pdf"):
		return clean, nil
	case strings.HasPrefix(clean, domain.EPUBPrefix) && strings.HasSuffix(lower, ".epub"):
		return clean, nil
	default:
		return "", fmt.Errorf("%w: extração disponível apenas para arquivos de %s e %s", ErrInvalidDocument, domain.PDFPrefix, domain.EPUBPrefix)
	}
}

// isInsideDir confirma que fullPath está dentro de dir (defesa contra `..`
// mesmo depois do join).
func isInsideDir(dir, fullPath string) bool {
	rel, err := filepath.Rel(dir, fullPath)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// extractNoteName devolve o nome da nota de extração, preferindo a nota já
// gerada para ESTE documento (para reextrair/atualizar) e caindo em sufixos
// numéricos quando o nome já pertence a outra nota.
func (ctx *HandlerContext) extractNoteName(source string) (string, error) {
	base := strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
	slug := neutralizeTypeHeuristic(slugifyFilename(base))
	if slug == "" || slug == "-" {
		slug = "documento"
	}

	for attempt := 1; attempt <= maxExtractNoteAttempts; attempt++ {
		name := fmt.Sprintf("%s%s-texto.md", domain.NotePrefix, slug)
		if attempt > 1 {
			name = fmt.Sprintf("%s%s-texto-%d.md", domain.NotePrefix, slug, attempt)
		}

		existing, err := ctx.Store.GetNote(name)
		if err != nil {
			return "", fmt.Errorf("consultar nota %s: %w", name, err)
		}
		if existing == "" || extractNoteSource(existing) == source {
			return name, nil
		}
	}

	return "", fmt.Errorf("não foi possível gerar um nome único para a nota de %s", source)
}

// neutralizeTypeHeuristic remove do nome da nota as palavras que
// domain.DetectNoteType usa como heurística de tipo pelo nome do arquivo.
// Sem isso, um PDF chamado "Drawing Guide.pdf" geraria a nota
// "drawing-guide-texto.md", que o resto da aplicação trataria como desenho
// (editor/ícone errados) só por causa do nome.
func neutralizeTypeHeuristic(slug string) string {
	for _, word := range []string{"drawing", "desenho"} {
		slug = strings.ReplaceAll(slug, word, "doc")
	}
	return slug
}

// extractNoteSource lê o campo `fonte:` do frontmatter ("" se ausente).
func extractNoteSource(content string) string {
	fm, _, err := ParseFrontmatter(content)
	if err != nil || fm == nil {
		return ""
	}
	raw, ok := fm[extractSourceKey]
	if !ok || raw == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(raw))
}

// buildExtractNoteMarkdown monta a nota: frontmatter com a origem, link para o
// documento (mesmas rotas do NoteOpenTarget) e o texto dividido por capítulos.
func buildExtractNoteMarkdown(source, title string, extracted *extract.Result, now time.Time) string {
	var b strings.Builder

	b.WriteString("---\n")
	fmt.Fprintf(&b, "tags: [%s]\n", extractTag)
	fmt.Fprintf(&b, "%s: %q\n", extractSourceKey, source)
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", strings.TrimSpace(title))
	fmt.Fprintf(&b, "> Documento original: [%s](%s)\n\n", extractLinkLabel(source), extractLinkURL(source))
	if extracted.Truncated {
		fmt.Fprintf(&b, "> ⚠️ Texto truncado em %d MiB na extração.\n\n", extract.MaxTextBytes/(1<<20))
	}

	for _, chapter := range extracted.Chapters {
		if strings.TrimSpace(chapter.Text) == "" {
			continue
		}
		if chapter.Title != "" {
			fmt.Fprintf(&b, "## %s\n\n", chapter.Title)
		}
		b.WriteString(strings.TrimSpace(chapter.Text))
		b.WriteString("\n\n")
	}

	fmt.Fprintf(&b, "---\n\n*Texto extraído automaticamente em %s.*\n", now.Format("02/01/2006 15:04"))
	return b.String()
}

func extractLinkLabel(source string) string {
	if strings.HasPrefix(source, domain.EPUBPrefix) {
		return "Abrir EPUB"
	}
	return "Abrir PDF"
}

// extractLinkURL devolve a rota canônica de abertura do documento, espelhando
// domain.NoteOpenTarget (PDF inline, EPUB no leitor embutido).
func extractLinkURL(source string) string {
	escaped := strings.ReplaceAll(url.QueryEscape(source), "%2F", "/")
	if strings.HasPrefix(source, domain.EPUBPrefix) {
		return "/epub/reader?file=" + escaped
	}
	return "/file?name=" + escaped
}
