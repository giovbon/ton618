package processor

import (
	"crypto/sha256"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
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
}

const (
	SectionDefault = "Geral"
	SectionImages  = "Anexos / Imagens"
)

var (
	headerRegex    = regexp.MustCompile(`(?m)^(#{1,6})\s+(.*)`)
	hashtagRegex   = regexp.MustCompile(`(?m)(?:\s|^)#([a-zA-Z0-9_À-ÿ\-]+)`)
	wikilinkRegex  = regexp.MustCompile(`\[\[([^\]|#]+)(?:[|#][^\]]*)?\]\]`)
	mediaLinkRegex = regexp.MustCompile(`\(/api/file\?name=([^)&]+)`)
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

// ProcessMarkdown analisa um arquivo markdown e retorna fragmentos de documento.
func ProcessMarkdown(path, filename string, modTime time.Time, creationTime time.Time) ([]Document, []string, []string) {
	timestampStr := modTime.UTC().Format(time.RFC3339)
	creationStr := creationTime.UTC().Format(time.RFC3339)

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil
	}

	text := strings.TrimLeft(string(content), " \t\r\n\xef\xbb\xbf")
	var docs []Document
	var fileTags []string
	var links []string

	// Parse frontmatter
	var metaParts []string
	if strings.HasPrefix(text, "---\n") || strings.HasPrefix(text, "---\r\n") {
		endIdx := strings.Index(text[4:], "\n---")
		if endIdx != -1 {
			endIdx += 4
			yamlContent := text[4:endIdx]
			var fm map[string]interface{}
			if err := yaml.Unmarshal([]byte(yamlContent), &fm); err == nil {
				if tRaw, ok := fm["tags"]; ok {
					if tList, ok := tRaw.([]interface{}); ok {
						for _, t := range tList {
							if ts, ok := t.(string); ok {
								cleanTag := strings.ToLower(strings.TrimSpace(ts))
								if cleanTag != "" {
									fileTags = append(fileTags, cleanTag)
								}
							}
						}
					}
				}
				// Detecta no_keywords: true e tag no-keywords
				fileTags = detectKeywords(fm, fileTags)
				// Serializa demais campos do frontmatter para indexacao FTS
				for k, v := range fm {
					if k == "tags" || k == "no_keywords" {
						continue
					}
					metaParts = append(metaParts, fmt.Sprintf("%v: %v", k, v))
				}

				// Se for planilha, esvaziamos o texto para não indexar o JSON na busca global
				if tRaw, ok := fm["type"]; ok && tRaw == "spreadsheet" {
					text = ""
					metaParts = nil // limpa também o frontmatter
					// Garante a tag "spreadsheet"
					hasSpreadsheetTag := false
					for _, tag := range fileTags {
						if tag == "spreadsheet" {
							hasSpreadsheetTag = true
							break
						}
					}
					if !hasSpreadsheetTag {
						fileTags = append(fileTags, "spreadsheet")
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
		}
	}

	if strings.TrimSpace(text) == "" {
		return nil, links, fileTags
	}

	// Extract hashtags from body
	tagMatches := hashtagRegex.FindAllStringSubmatch(text, -1)
	for _, m := range tagMatches {
		if len(m) > 1 {
			tag := strings.ToLower(m[1])
			exists := false
			for _, existing := range fileTags {
				if existing == tag {
					exists = true
					break
				}
			}
			if !exists && tag != "" {
				fileTags = append(fileTags, tag)
			}
		}
	}

	// Extract wikilinks e normaliza: notas markdown ganham prefixo notes/
	linkMatches := wikilinkRegex.FindAllStringSubmatch(text, -1)
	for _, m := range linkMatches {
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
			}
		}
	}

	// Extract media links
	mediaMatches := mediaLinkRegex.FindAllStringSubmatch(text, -1)
	for _, m := range mediaMatches {
		if len(m) > 1 {
			target := strings.TrimSpace(m[1])
			if target != "" {
				links = append(links, target)
			}
		}
	}

	// Split by headers
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
		docs = append(docs, Document{
			ID:        HashFunc(filename + SectionDefault),
			Tipo:      "markdown",
			Arquivo:   filename,
			Secao:     SectionDefault,
			Texto:     text,
			Timestamp: timestampStr,
			Created:   creationStr,
			Hash:      CalculateHash(SectionDefault, text, fileTags),
			Tags:      fileTags,
		})
	} else {
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
				})
				ordem++
			}
			levelStr := text[m[2]:m[3]]
			level := len(levelStr)
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
		})
	}

	// Prepend frontmatter metadata to first document for FTS indexing
	if len(metaParts) > 0 && len(docs) > 0 {
		docs[0].Texto = strings.Join(metaParts, " | ") + "\n\n" + docs[0].Texto
	}

	return docs, links, fileTags
}

// ProcessMarkdownContent analisa conteúdo markdown em memória e retorna fragmentos.
// É idêntica a ProcessMarkdown mas aceita o conteúdo como []byte em vez de ler do disco.
func ProcessMarkdownContent(content []byte, filename string, modTime time.Time, creationTime time.Time) ([]Document, []string, []string) {
	timestampStr := modTime.UTC().Format(time.RFC3339)
	creationStr := creationTime.UTC().Format(time.RFC3339)

	text := strings.TrimLeft(string(content), " \t\r\n\xef\xbb\xbf")
	var docs []Document
	var fileTags []string
	var links []string

	// Parse frontmatter
	var metaParts []string
	if strings.HasPrefix(text, "---\n") || strings.HasPrefix(text, "---\r\n") {
		endIdx := strings.Index(text[4:], "\n---")
		if endIdx != -1 {
			endIdx += 4
			yamlContent := text[4:endIdx]
			var fm map[string]interface{}
			if err := yaml.Unmarshal([]byte(yamlContent), &fm); err == nil {
				if tRaw, ok := fm["tags"]; ok {
					if tList, ok := tRaw.([]interface{}); ok {
						for _, t := range tList {
							if ts, ok := t.(string); ok {
								cleanTag := strings.ToLower(strings.TrimSpace(ts))
								if cleanTag != "" {
									fileTags = append(fileTags, cleanTag)
								}
							}
						}
					}
				}
				// Detecta no_keywords: true e tag no-keywords
				fileTags = detectKeywords(fm, fileTags)
				// Serializa demais campos do frontmatter para indexacao FTS
				for k, v := range fm {
					if k == "tags" || k == "no_keywords" {
						continue
					}
					metaParts = append(metaParts, fmt.Sprintf("%v: %v", k, v))
				}

				// Se for planilha, esvaziamos o texto para não indexar o JSON na busca global
				if tRaw, ok := fm["type"]; ok && tRaw == "spreadsheet" {
					text = ""
					metaParts = nil // limpa também o frontmatter
					// Garante a tag "spreadsheet"
					hasSpreadsheetTag := false
					for _, tag := range fileTags {
						if tag == "spreadsheet" {
							hasSpreadsheetTag = true
							break
						}
					}
					if !hasSpreadsheetTag {
						fileTags = append(fileTags, "spreadsheet")
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
		}
	}

	if strings.TrimSpace(text) == "" {
		return nil, links, fileTags
	}

	// Extract hashtags from body
	tagMatches := hashtagRegex.FindAllStringSubmatch(text, -1)
	for _, m := range tagMatches {
		if len(m) > 1 {
			tag := strings.ToLower(m[1])
			exists := false
			for _, existing := range fileTags {
				if existing == tag {
					exists = true
					break
				}
			}
			if !exists && tag != "" {
				fileTags = append(fileTags, tag)
			}
		}
	}

	// Extract wikilinks e normaliza: notas markdown ganham prefixo notes/
	linkMatches := wikilinkRegex.FindAllStringSubmatch(text, -1)
	for _, m := range linkMatches {
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
			}
		}
	}

	// Extract media links
	mediaMatches := mediaLinkRegex.FindAllStringSubmatch(text, -1)
	for _, m := range mediaMatches {
		if len(m) > 1 {
			target := strings.TrimSpace(m[1])
			if target != "" {
				links = append(links, target)
			}
		}
	}

	// Split by headers
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
		docs = append(docs, Document{
			ID:        HashFunc(filename + SectionDefault),
			Tipo:      "markdown",
			Arquivo:   filename,
			Secao:     SectionDefault,
			Texto:     text,
			Timestamp: timestampStr,
			Created:   creationStr,
			Hash:      CalculateHash(SectionDefault, text, fileTags),
			Tags:      fileTags,
		})
	} else {
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
				})
				ordem++
			}
			levelStr := text[m[2]:m[3]]
			level := len(levelStr)
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
		})
	}

	// Prepend frontmatter metadata to first document for FTS indexing
	if len(metaParts) > 0 && len(docs) > 0 {
		docs[0].Texto = strings.Join(metaParts, " | ") + "\n\n" + docs[0].Texto
	}

	return docs, links, fileTags
}

// ExtractTitle extrai o título do primeiro heading markdown (linha iniciada com #).
// Se não houver heading, retorna o nome do arquivo sem extensão.
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

// ── No-Keywords Flag ──
// A propriedade no_keywords: true no frontmatter YAML ou a tag "no-keywords"
// desabilita a extração de palavras-chave (RAKE) para esta nota.

const keywordsSentinel = "__keywords__"

// HasNoKeywords verifica se o slice de tags contém o sentinel
// que indica que a extração de keywords deve ser ignorada.
func HasKeywords(fileTags []string) bool {
	for _, t := range fileTags {
		if t == keywordsSentinel {
			return true
		}
	}
	return false
}

// FilterNoKeywords remove o sentinel __no_keywords__ de fileTags
// para que ele não seja persistido como tag real no banco.
func FilterKeywords(fileTags []string) []string {
	filtered := make([]string, 0, len(fileTags))
	for _, t := range fileTags {
		if t != keywordsSentinel {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// detectNoKeywords verifica se a propriedade no_keywords: true está presente
// no frontmatter YAML, ou se a tag "no-keywords" está na lista de tags.
// Se detectado, adiciona o sentinel __no_keywords__ a fileTags.
func detectKeywords(fm map[string]interface{}, fileTags []string) []string {
	// Verifica propriedade keywords no frontmatter
	if v, ok := fm["keywords"]; ok {
		if b, ok := v.(bool); ok && b {
			hasSentinel := false
			for _, t := range fileTags {
				if t == keywordsSentinel {
					hasSentinel = true
					break
				}
			}
			if !hasSentinel {
				fileTags = append(fileTags, keywordsSentinel)
			}
			return fileTags
		}
	}
	// Verifica se a tag "keywords" está presente (como hashtag #keywords)
	for _, t := range fileTags {
		if t == "keywords" {
			hasSentinel := false
			for _, ft := range fileTags {
				if ft == keywordsSentinel {
					hasSentinel = true
					break
				}
			}
			if !hasSentinel {
				fileTags = append(fileTags, keywordsSentinel)
			}
			return fileTags
		}
	}
	return fileTags
}

// TodoItem representa um TODO, FIXME, BUG ou checkbox encontrado em uma nota.
type TodoItem struct {
	ID      string
	File    string
	Section string
	Type    string // "TODO", "FIXME", "BUG", "TASK"
	Status  string // "pending", "completed"
	Text    string
	Line    int
	Created time.Time
}

// ExtractTodos extrai marcadores (TODO, FIXME, BUG, etc) e checkboxes de conteúdo markdown.
// markers é a lista de palavras-chave a detectar (ex: ["TODO", "FIXME", "BUG"]).
// Se markers for nil/vazio, usa os defaults: TODO, FIXME, BUG.
func ExtractTodos(content string, filename string, modTime time.Time, markers []string) []TodoItem {
	var todos []TodoItem

	if len(markers) == 0 {
		markers = []string{"TODO", "FIXME", "BUG"}
	}

	// Build dynamic regex from markers list
	markerPattern := strings.Join(markers, "|")
	todoRegex := regexp.MustCompile(`(?i)^\s*(` + markerPattern + `):\s*(.+)$`)
	checkboxRegex := regexp.MustCompile(`(?i)^\s*[-*]\s*\[([ xX])\]\s*(.+)$`)
	headerRegex := regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

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

	for i := startIdx; i < len(lines); i++ {
		line := lines[i]
		lineNum = i + 1

		// Update current section on header
		if headerMatch := headerRegex.FindStringSubmatch(line); headerMatch != nil {
			currentSection = strings.TrimSpace(headerMatch[2])
			continue
		}

		// Check for TODO/FIXME/BUG
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
		if checkboxMatch := checkboxRegex.FindStringSubmatch(line); checkboxMatch != nil {
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
