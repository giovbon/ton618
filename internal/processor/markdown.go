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
	ID         string
	Tipo       string
	Arquivo    string
	Secao      string
	Pagina     int
	Ordem      int
	Texto      string
	Timestamp  string
	Created    string
	Hash       string
	VectorHash string
	Tags       []string
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
	return fmt.Sprintf("%x", sha256.Sum256([]byte(input))[:8])
}

// CalculateVectorHash gera hash do conteúdo textual (independente de tags/seção).
func CalculateVectorHash(secao, texto string) string {
	input := secao + texto
	return fmt.Sprintf("%x", sha256.Sum256([]byte(input))[:8])
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

	// Extract wikilinks
	linkMatches := wikilinkRegex.FindAllStringSubmatch(text, -1)
	for _, m := range linkMatches {
		if len(m) > 1 {
			target := strings.TrimSpace(m[1])
			if target != "" {
				target = strings.ToLower(target)
				if !strings.Contains(target, ".") {
					target += ".md"
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
			ID:         HashFunc(filename + SectionDefault),
			Tipo:       "markdown",
			Arquivo:    filename,
			Secao:      SectionDefault,
			Texto:      text,
			Timestamp:  timestampStr,
			Created:    creationStr,
			Hash:       CalculateHash(SectionDefault, text, fileTags),
			VectorHash: CalculateVectorHash(SectionDefault, text),
			Tags:       fileTags,
		})
	} else {
		lastPos := 0
		currentHeader := SectionDefault

		for _, m := range matches {
			contentBefore := text[lastPos:m[0]]
			if strings.TrimSpace(contentBefore) != "" {
				docs = append(docs, Document{
					ID:         HashFunc(filename + currentHeader + strconv.Itoa(ordem)),
					Tipo:       "markdown",
					Arquivo:    filename,
					Secao:      currentHeader,
					Texto:      contentBefore,
					Timestamp:  timestampStr,
					Created:    creationStr,
					Hash:       CalculateHash(currentHeader, contentBefore, fileTags),
					VectorHash: CalculateVectorHash(currentHeader, contentBefore),
					Tags:       fileTags,
					Ordem:      ordem,
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
			ID:         HashFunc(filename + currentHeader + strconv.Itoa(ordem)),
			Tipo:       "markdown",
			Arquivo:    filename,
			Secao:      currentHeader,
			Texto:      remaining,
			Timestamp:  timestampStr,
			Created:    creationStr,
			Hash:       CalculateHash(currentHeader, remaining, fileTags),
			VectorHash: CalculateVectorHash(currentHeader, remaining),
			Tags:       fileTags,
			Ordem:      ordem,
		})
	}

	return docs, links, fileTags
}

// ExtractTitle extrai o título da primeira linha não-vazia do conteúdo.
func ExtractTitle(content, filename string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		clean := strings.TrimSpace(strings.TrimLeft(line, "# "))
		if clean != "" {
			return clean
		}
	}
	parts := strings.Split(filename, "/")
	return strings.TrimSuffix(parts[len(parts)-1], ".md")
}
