package processor

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"ton618/core/internal/core/domain"
)

// Document representa um fragmento indexável de um arquivo.
type Document struct {
	ID        string
	Tipo      string
	Arquivo   string
	Secao     string
	Pagina    int
	Ordem     int
	Texto     string
	Timestamp string
	Created   string
	Hash      string
	Tags      []string
	// NoteType é o valor canônico do campo `tipo:` da nota de origem
	// (vocabulário PT snake_case: mapa_mental, anexo, nota, ...). Alimenta a
	// coluna note_type de documents/docs_fts, usada pelo filtro `tipo:` da
	// busca global e do Acervo. Vazio = o indexador deriva de tags + caminho.
	NoteType string
}

const (
	SectionDefault = "Geral"
	SectionImages  = "Anexos / Imagens"
)

var (
	headerRegex           = regexp.MustCompile(`(?m)^(#{1,6})\s+(.*)`)
	hashtagRegex          = regexp.MustCompile(`(?m)(?:\s|^)#([a-zA-Z0-9_À-ÿ\-]+)`)
	WikilinkRegex         = regexp.MustCompile(`\[\[([^\]|#]+)(?:[|#][^\]]*)?\]\]`)
	mediaLinkRegex        = regexp.MustCompile(`(?i)(?:/file(?:/download)?|/api/file)\?name=([^)\s"&]+)`)
	markdownFileLinkRegex = regexp.MustCompile(`(?i)\]\(((?:attachments|archives|pdfs|epubs|notes)/[^)\s]+)\)`)
)

// HashFunc gera um ID único para um fragmento.
func HashFunc(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:16])
}

// CalculateHash gera hash para detecção de mudanças.
func CalculateHash(secao, texto string, tags []string) string {
	input := secao + texto + strings.Join(tags, ",")
	h := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", h[:8])
}

// frontmatterTypeValue devolve o tipo declarado no frontmatter, aceitando a
// chave canônica `tipo` (gravada pelo app) e a legada `type`, ambas já com
// chaves minúsculas. O valor sai normalizado por domain.NormalizeNoteTipo
// (Drawing → desenho, mindmap → mapa_mental, ...).
func frontmatterTypeValue(fm map[string]interface{}) (string, bool) {
	for _, key := range []string{"tipo", "type"} {
		raw, ok := fm[key]
		if !ok {
			continue
		}
		s, ok := raw.(string)
		if !ok || strings.TrimSpace(s) == "" {
			continue
		}
		return domain.NormalizeNoteTipo(s), true
	}
	return "", false
}

// applyFrontmatterType aplica comportamentos específicos de tipo. Recebe o
// valor JÁ normalizado por domain.NormalizeNoteTipo (desenho, mapa_mental,
// tabulator, ...), não o texto cru do frontmatter.
// Retorna as fileTags atualizadas.
func applyFrontmatterType(tipo string, text *string, metaParts *[]string, fileTags []string) []string {
	// Tag canônica a garantir; tipos binários também limpam o texto
	var canonicalTag string
	clearText := false

	switch tipo {
	case "desenho":
		canonicalTag = "drawing"
		clearText = true
	case "mapa_mental":
		canonicalTag = "markmap"
	case "tabulator":
		// Nota Tabulator: o corpo (JSON da planilha) É indexado — ver
		// ProcessMarkdownContent, que delega para ProcessTabulatorContent.
		canonicalTag = "tabulator"
	default:
		return fileTags
	}

	if clearText {
		*text = ""
		*metaParts = nil
	}

	// Garante que a tag canônica esteja presente
	for _, t := range fileTags {
		if t == canonicalTag {
			return fileTags
		}
	}
	fileTags = append(fileTags, canonicalTag)
	return fileTags
}

// ProcessMarkdown analisa um arquivo markdown e retorna fragmentos de documento.
// Lê o conteúdo do disco e delega para ProcessMarkdownContent.
func ProcessMarkdown(path, filename string, modTime time.Time, creationTime time.Time) ([]Document, []string, []string) {
	rawContent, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil
	}
	return ProcessMarkdownContent(rawContent, filename, modTime, modTime)
}

// ProcessMarkdownContent analisa conteúdo markdown em memória e retorna fragmentos.
// É idêntica a ProcessMarkdown mas aceita o conteúdo como []byte em vez de ler do disco.
func ProcessMarkdownContent(content []byte, filename string, modTime time.Time, creationTime time.Time) ([]Document, []string, []string) {
	timestampStr := modTime.UTC().Format(time.RFC3339)
	creationStr := creationTime.UTC().Format(time.RFC3339)

	text := strings.TrimLeft(string(content), " \t\r\n\xef\xbb\xbf")
	text, metaParts, frontmatterType, fileTags := parseFrontmatterForIndex(text)

	// Tipo da nota para a coluna note_type do índice (filtro `tipo:` da busca).
	// Espelho de DetectNoteType com fallback de conteúdo: tags + caminho + nome
	// já decidem a maioria; o conteúdo cobre os especiais que só têm o `type:`
	// no frontmatter (drawing/markmap/tabulator sem tag persistida ainda).
	noteTipo := domain.FrontmatterTipo(domain.DetectNoteTypeFromContent(fileTags, string(content), filename))

	// Nota Tabulator: indexa o corpo JSON célula a célula (um fragmento por
	// linha + um com os nomes das colunas) em vez de tratá-lo como markdown.
	if frontmatterType == "tabulator" {
		tabDocs := ProcessTabulatorContent(text, filename, timestampStr, creationStr, fileTags)
		for i := range tabDocs {
			tabDocs[i].NoteType = noteTipo
		}
		return tabDocs, nil, fileTags
	}

	if strings.TrimSpace(text) == "" {
		return nil, nil, fileTags
	}

	fileTags = appendHashtags(fileTags, text)
	links := extractMarkdownLinks(text)
	docs := buildMarkdownDocuments(text, filename, timestampStr, creationStr, fileTags, noteTipo)

	// Prepend frontmatter metadata to first document for FTS indexing
	if len(metaParts) > 0 && len(docs) > 0 {
		docs[0].Texto = strings.Join(metaParts, " | ") + "\n\n" + docs[0].Texto
	}

	return docs, links, fileTags
}

// parseFrontmatterForIndex separa o frontmatter do corpo e devolve os metadados
// que entram no FTS, o `type:` (normalizado) e as tags declaradas. Em YAML
// inválido o bloco é descartado, como antes.
func parseFrontmatterForIndex(text string) (body string, metaParts []string, frontmatterType string, fileTags []string) {
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return text, nil, "", nil
	}
	endIdx := strings.Index(text[4:], "\n---")
	if endIdx == -1 {
		return text, nil, "", nil
	}
	endIdx += 4

	var fm map[string]interface{}
	if err := yaml.Unmarshal([]byte(text[4:endIdx]), &fm); err == nil {
		// Normalize keys to lowercase to allow case-insensitive frontmatter keys
		cleanFm := make(map[string]interface{}, len(fm))
		for k, v := range fm {
			cleanFm[strings.ToLower(k)] = v
		}
		fm = cleanFm

		fileTags = extractFrontmatterTags(fm["tags"])
		// Serializa demais campos do frontmatter para indexacao FTS
		for k, v := range fm {
			if k == "tags" || k == "keywords" {
				continue
			}
			metaParts = append(metaParts, fmt.Sprintf("%v: %v", k, v))
		}

		// Aplica comportamentos e tag canônica conforme o tipo do frontmatter.
		// `tipo` é a chave canônica gravada pelo app; `type` é aceita por
		// compatibilidade. A normalização é a mesma da coluna note_type/filtros.
		if tipo, ok := frontmatterTypeValue(fm); ok {
			frontmatterType = tipo
			fileTags = applyFrontmatterType(tipo, &text, &metaParts, fileTags)
		}
	}

	afterFrontmatter := endIdx + 4
	if afterFrontmatter < len(text) && text[afterFrontmatter] == '\n' {
		afterFrontmatter++
	}
	if afterFrontmatter <= len(text) {
		text = text[afterFrontmatter:]
	}
	return text, metaParts, frontmatterType, fileTags
}

// extractFrontmatterTags aceita `tags` como lista YAML ou string separada por vírgulas.
func extractFrontmatterTags(raw interface{}) []string {
	var tags []string
	switch v := raw.(type) {
	case []interface{}:
		for _, t := range v {
			if ts, ok := t.(string); ok {
				tags = appendCleanTag(tags, ts)
			}
		}
	case string:
		for _, t := range strings.Split(v, ",") {
			tags = appendCleanTag(tags, t)
		}
	}
	return tags
}

func appendCleanTag(tags []string, raw string) []string {
	tag := strings.ToLower(strings.TrimSpace(raw))
	tag = strings.TrimPrefix(tag, "#")
	if tag == "" {
		return tags
	}
	return append(tags, tag)
}

// appendHashtags adiciona ao índice as hashtags do corpo, ignorando repetidas.
func appendHashtags(tags []string, text string) []string {
	for _, m := range hashtagRegex.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			tag := strings.ToLower(m[1])
			if tag == "" || containsTag(tags, tag) {
				continue
			}
			tags = append(tags, tag)
		}
	}
	return tags
}

func containsTag(tags []string, tag string) bool {
	for _, existing := range tags {
		if existing == tag {
			return true
		}
	}
	return false
}

// extractMarkdownLinks coleta wikilinks, links de mídia (?name=…) e caminhos
// relativos (attachments/…), normalizando o que é nota para notes/xxx.md.
func extractMarkdownLinks(text string) []string {
	var links []string

	// Extract wikilinks e normaliza: notas markdown ganham prefixo notes/
	for _, m := range WikilinkRegex.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			target := strings.TrimSpace(m[1])
			if target != "" {
				target = strings.ToLower(target)
				if !strings.Contains(target, ".") {
					target += ".md"
				}
				// Normaliza: se for .md sem path, adiciona prefixo notes/
				if strings.HasSuffix(target, ".md") && !strings.Contains(target, "/") {
					target = "notes/" + target
				}
				links = append(links, target)
				// Se for um anexo sem diretório (ex: meuarquivo.zip), indexa também com os prefixos possíveis
				if strings.Contains(target, ".") && !strings.Contains(target, "/") && !strings.HasSuffix(target, ".md") {
					for _, prefix := range domain.NonNoteFilePrefixes {
						links = append(links, prefix+target)
					}
				}
			}
		}
	}

	// Extract media e download links (?name=...)
	for _, m := range mediaLinkRegex.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			target := strings.TrimSpace(m[1])
			if target != "" {
				if unescaped, err := url.QueryUnescape(target); err == nil {
					target = unescaped
				}
				links = append(links, strings.ToLower(target))
			}
		}
	}

	// Extract markdown relative links ](attachments/... etc)
	for _, m := range markdownFileLinkRegex.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			target := strings.TrimSpace(m[1])
			if target != "" {
				links = append(links, strings.ToLower(target))
			}
		}
	}

	return links
}

// buildMarkdownDocuments divide o corpo por headers, montando a trilha de
// seções (headerStack) e um Document por trecho não vazio.
func buildMarkdownDocuments(text, filename, timestampStr, creationStr string, fileTags []string, noteTipo string) []Document {
	matches := headerRegex.FindAllStringSubmatchIndex(text, -1)
	ordem := 0
	headerStack := make([]string, 7)

	formatSectionTrail := func(level int, title string) string {
		title = strings.TrimSpace(title)
		if level >= 1 && level <= 6 {
			headerStack[level] = title
			for i := level + 1; i <= 6; i++ {
				headerStack[i] = ""
			}
		}
		var trail []string
		for i := 1; i <= 6; i++ {
			if headerStack[i] != "" {
				trail = append(trail, headerStack[i])
			}
		}
		if len(trail) == 0 {
			return SectionDefault
		}
		return strings.Join(trail, " › ")
	}

	if len(matches) == 0 {
		return []Document{{
			ID:        HashFunc(filename + SectionDefault),
			Tipo:      "markdown",
			Arquivo:   filename,
			Secao:     SectionDefault,
			Texto:     text,
			Timestamp: timestampStr,
			Created:   creationStr,
			Hash:      CalculateHash(SectionDefault, text, fileTags),
			Tags:      fileTags,
			NoteType:  noteTipo,
		}}
	}

	var docs []Document
	lastPos := 0
	currentHeader := SectionDefault

	for _, m := range matches {
		contentBefore := text[lastPos:m[0]]
		if strings.TrimSpace(contentBefore) != "" {
			docs = append(docs, Document{
				ID:        HashFunc(filename + currentHeader + strconv.Itoa(ordem)),
				Tipo:      "markdown",
				Arquivo:   filename,
				Secao:     currentHeader,
				Texto:     contentBefore,
				Timestamp: timestampStr,
				Created:   creationStr,
				Hash:      CalculateHash(currentHeader, contentBefore, fileTags),
				Tags:      fileTags,
				Ordem:     ordem,
				NoteType:  noteTipo,
			})
			ordem++
		}
		levelStr := text[m[2]:m[3]]
		level := len(levelStr)
		if level > 6 {
			level = 6
		}
		title := text[m[4]:m[5]]
		currentHeader = formatSectionTrail(level, title)
		lastPos = m[0]
	}

	remaining := text[lastPos:]
	docs = append(docs, Document{
		ID:        HashFunc(filename + currentHeader + strconv.Itoa(ordem)),
		Tipo:      "markdown",
		Arquivo:   filename,
		Secao:     currentHeader,
		Texto:     remaining,
		Timestamp: timestampStr,
		Created:   creationStr,
		Hash:      CalculateHash(currentHeader, remaining, fileTags),
		Tags:      fileTags,
		Ordem:     ordem,
		NoteType:  noteTipo,
	})
	return docs
}

func ExtractTitle(content, filename string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			// É um heading markdown; remove os # e espaços iniciais
			clean := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			if clean != "" {
				return clean
			}
		}
	}
	// Nenhum heading encontrado: usa o nome do arquivo
	parts := strings.Split(filename, "/")
	return strings.TrimSuffix(parts[len(parts)-1], ".md")
}
