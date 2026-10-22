package processor

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
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

	checkboxTodoRegex = regexp.MustCompile(`(?i)^\s*[-*]\s*\[([ xX])\]\s*(.+)$`)
	todoHeaderRegex   = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

	todoRegexMu      sync.RWMutex
	todoRegexPattern string
	cachedTodoRegex  *regexp.Regexp
)

type DefaultMarkerInfo struct {
	Marker string
	Color  string
	Active bool
}

var DefaultTodoMarkers = []DefaultMarkerInfo{
	{Marker: "TODO", Color: "#3b82f6", Active: true},
	{Marker: "DOING", Color: "#f59e0b", Active: true},
	{Marker: "DONE", Color: "#10b981", Active: true},
}

// noTodoMarkerPattern é um regex que NUNCA casa (nenhum caractere é, ao mesmo
// tempo, espaço e não-espaço). Usado quando não há marcador válido para detectar.
const noTodoMarkerPattern = `[^\s\S]`

// todoMarkerNameRegex valida o nome de um marcador criado pelo usuário:
// letras (acentos incluídos), números, espaço, "_", "-" e "." — sem dois-pontos,
// sem metacaractere de regex e sem caractere que atrapalhe a query string das
// URLs do HTMX. Máximo de 32 caracteres.
var todoMarkerNameRegex = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N} _.\-]{0,31}$`)

// IsValidTodoMarkerName diz se o nome pode virar marcador.
//
// Rejeita o reservado "TASK": é o tipo dos checkboxes `- [ ]` / `- [x]` de
// markdown, que não são marcadores e ficam fora da listagem (HandleListTodos) e
// da contagem do cabeçalho (CountTodosByMarkers) — um marcador com esse nome
// contaria itens que não aparecem em lugar nenhum.
func IsValidTodoMarkerName(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || strings.EqualFold(trimmed, "TASK") {
		return false
	}
	return todoMarkerNameRegex.MatchString(trimmed)
}

// todoMarkerColorRegex aceita só `#rrggbb`.
var todoMarkerColorRegex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// IsValidTodoMarkerColor diz se a cor é segura/útil para o estilo inline dos
// templates (o valor é interpolado em `style="background:<cor>20; color:<cor>"`).
func IsValidTodoMarkerColor(color string) bool {
	return todoMarkerColorRegex.MatchString(strings.TrimSpace(color))
}

// getTodoRegex compila (com cache) o regex de marcadores do tipo `TODO: texto`.
//
// Os marcadores são NOMEADOS pelo usuário (Configurações → Marcadores) e são
// interpolados no regex, então cada um passa por regexp.QuoteMeta. Sem isso um
// marcador como `(` derrubava o regexp.MustCompile com **panic** no próximo save
// de nota — e como o panic acontece antes do SaveNote, a nota não era salva e o
// app ficava inutilizável para escrever até o marcador ser removido. Além do
// panic, um marcador com metacaractere virava curinga silencioso (ex: `.` casava
// qualquer linha no formato `x: texto`). QuoteMeta é a proteção de fato, porque
// bancos antigos podem ter marcadores gravados antes da validação de nome.
//
// markers == nil significa "não informado" → usa os defaults. Slice vazio (ou só
// com nomes inválidos) significa "nenhum marcador ativo" → não detecta nada: antes
// o vazio caía nos defaults e continuava criando tarefas de marcadores que o
// usuário havia desativado.
func getTodoRegex(markers []string) *regexp.Regexp {
	if markers == nil {
		markers = make([]string, len(DefaultTodoMarkers))
		for i, m := range DefaultTodoMarkers {
			markers[i] = m.Marker
		}
	}

	parts := make([]string, 0, len(markers))
	seen := make(map[string]bool, len(markers))
	for _, m := range markers {
		name := strings.TrimSpace(m)
		key := strings.ToUpper(name)
		if name == "" || seen[key] {
			continue
		}
		seen[key] = true
		parts = append(parts, regexp.QuoteMeta(name))
	}

	pattern := strings.Join(parts, "|")
	if pattern == "" {
		pattern = noTodoMarkerPattern
	}

	todoRegexMu.RLock()
	if todoRegexPattern == pattern && cachedTodoRegex != nil {
		defer todoRegexMu.RUnlock()
		return cachedTodoRegex
	}
	todoRegexMu.RUnlock()

	todoRegexMu.Lock()
	defer todoRegexMu.Unlock()
	if todoRegexPattern == pattern && cachedTodoRegex != nil {
		return cachedTodoRegex
	}

	cachedTodoRegex = regexp.MustCompile(`(?i)^\s*(` + pattern + `):\s*(.+)$`)
	todoRegexPattern = pattern
	return cachedTodoRegex
}

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

// applyFrontmatterType aplica comportamentos específicos de tipo baseado no campo "type:" do frontmatter.
// Retorna as fileTags atualizadas.
func applyFrontmatterType(typeStr string, text *string, metaParts *[]string, fileTags []string) []string {
	// Tag canônica a garantir; tipos binários também limpam o texto
	var canonicalTag string
	clearText := false

	switch typeStr {
	case "drawing":
		canonicalTag = "drawing"
		clearText = true
	case "markmap", "mindmap":
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

		// Aplica comportamentos e tag canônica conforme o type: do frontmatter
		if typeRaw, ok := fm["type"]; ok {
			if typeStr, ok := typeRaw.(string); ok {
				frontmatterType = strings.ToLower(strings.TrimSpace(typeStr))
				fileTags = applyFrontmatterType(typeStr, &text, &metaParts, fileTags)
			}
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

// TodoItem representa um TODO, DOING, DONE ou checkbox encontrado em uma nota.
type TodoItem struct {
	ID      string
	File    string
	Section string
	Type    string // "TODO", "DOING", "DONE", "TASK"
	Status  string // "pending", "completed"
	Text    string
	Line    int
	Created time.Time
}

// ExtractTodos extrai marcadores (TODO, DOING, DONE, etc) e checkboxes de conteúdo markdown.
// markers é a lista de palavras-chave a detectar (ex: ["TODO", "DOING", "DONE"]).
// Se markers for nil/vazio, usa os defaults: TODO, DOING, DONE.
func ExtractTodos(content string, filename string, modTime time.Time, markers []string) []TodoItem {
	var todos []TodoItem

	// Get cached regex for markers
	todoRegex := getTodoRegex(markers)

	lines := strings.Split(content, "\n")
	currentSection := "Geral"
	lineNum := 0
	startIdx := 0

	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				startIdx = i + 1
				lineNum = i + 1
				break
			}
		}
	}

	activeTodoHeaderRegex := todoHeaderRegex

	for i := startIdx; i < len(lines); i++ {
		line := lines[i]
		lineNum = i + 1

		// Update current section on header
		if headerMatch := activeTodoHeaderRegex.FindStringSubmatch(line); headerMatch != nil {
			currentSection = strings.TrimSpace(headerMatch[2])
			continue
		}

		// Check for TODO/DOING/DONE
		if todoMatch := todoRegex.FindStringSubmatch(line); todoMatch != nil {
			todoType := strings.ToUpper(todoMatch[1])
			todoText := strings.TrimSpace(todoMatch[2])
			todos = append(todos, TodoItem{
				ID:      HashFunc(filename + currentSection + strconv.Itoa(lineNum)),
				File:    filename,
				Section: currentSection,
				Type:    todoType,
				Status:  "pending",
				Text:    todoText,
				Line:    lineNum,
				Created: modTime,
			})
			continue
		}

		// Check for checkboxes
		if checkboxMatch := checkboxTodoRegex.FindStringSubmatch(line); checkboxMatch != nil {
			checkbox := checkboxMatch[1]
			taskText := strings.TrimSpace(checkboxMatch[2])
			status := "pending"
			if strings.ToLower(checkbox) == "x" {
				status = "completed"
			}
			todos = append(todos, TodoItem{
				ID:      HashFunc(filename + currentSection + taskText + strconv.Itoa(lineNum)),
				File:    filename,
				Section: currentSection,
				Type:    "TASK",
				Status:  status,
				Text:    taskText,
				Line:    lineNum,
				Created: modTime,
			})
			continue
		}
	}

	return todos
}
