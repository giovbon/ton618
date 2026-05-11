package ingest

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"etl/internal/models"
	"etl/internal/semantic"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
	"gopkg.in/yaml.v3"
)

var (
	headerRegex    = regexp.MustCompile(`(?m)^(#{1,6})\s+(.*)`)
	hashtagRegex   = regexp.MustCompile(`(?m)(?:\s|^)#([a-zA-Z0-9_À-ÿ\-]+)`)
	wikilinkRegex  = regexp.MustCompile(`\[\[([^\]|#]+)(?:[|#][^\]]*)?\]\]`)
	semanticRegex  = regexp.MustCompile(`@\\?\[([^\]]+?)\\?\]`) // Aceita @[texto] e @\[texto\] com backslash opcional antes dos colchetes
	mediaLinkRegex = regexp.MustCompile(`\(/api/file\?name=([^)&]+)`)
)

var stripHTMLRe = regexp.MustCompile(`<[^>]*>`)

// Auxiliar para remover tags HTML simples
func stripHTML(s string) string {
	return stripHTMLRe.ReplaceAllString(s, "")
}

// ExtractSemanticLinks extrai links semanticos @[topico] do texto.
// Primeiro remove HTML (o serializador markdown envolve @[texto] em <span>),
// depois aplica regex que aceita @[texto] e @\[texto\] com backslash opcional.
func ExtractSemanticLinks(text string) []string {
	cleanText := stripHTML(text)
	cleanText = strings.ReplaceAll(cleanText, "\n", " ")
	matches := semanticRegex.FindAllStringSubmatch(cleanText, -1)
	var links []string
	for _, m := range matches {
		if len(m) > 1 {
			target := strings.TrimSpace(m[1])
			if target != "" {
				target = strings.ReplaceAll(target, "\\", "")
				links = append(links, strings.TrimSpace(target))
			}
		}
	}
	return links
}

func ReadFileWithRetry(path string, retries int) ([]byte, error) {
	var content []byte
	var err error
	for i := 0; i < retries; i++ {
		content, err = os.ReadFile(path)
		if err == nil {
			return content, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, err
}

func ProcessMarkdown(path, filename string, modTime time.Time, appState *AppState) ([]models.Document, []string, []string, map[string]interface{}, []string) {
	log.Printf("[Sync] ProcessMarkdown: processando %s\n", filename)
	timestampStr := modTime.UTC().Format(time.RFC3339)

	// Recuperar data de criação original (se existir)
	creationTime, exists := appState.GetFileCreation(filename)
	if !exists {
		creationTime = modTime
	}
	creationStr := creationTime.UTC().Format(time.RFC3339)

	content, err := ReadFileWithRetry(path, 3)
	if err != nil {
		log.Printf("Erro ao ler %s: %v\n", path, err)
		return nil, nil, nil, nil, nil
	}

	var docs []models.Document
	text := strings.TrimLeft(string(content), " \t\r\n\xef\xbb\xbf")
	var fileTags []string
	metadata := make(map[string]interface{})

	if strings.Contains(text, "PANIC_TEST_TRIGGER") {
		panic("Simulated processing panic")
	}

	if strings.HasPrefix(text, "---\n") || strings.HasPrefix(text, "---\r\n") {
		endIdx := strings.Index(text[4:], "\n---")
		if endIdx != -1 {
			endIdx += 4
			yamlContent := text[4:endIdx]
			var fm map[string]interface{}
			if err := yaml.Unmarshal([]byte(yamlContent), &fm); err == nil {
				metadata = fm
				if tRaw, ok := fm["tags"]; ok {
					if tList, ok := tRaw.([]interface{}); ok {
						for _, t := range tList {
							if ts, ok := t.(string); ok {
								cleanTag := strings.ToLower(strings.TrimSpace(ts))
								if cleanTag != "" {
									fileTags = append(fileTags, cleanTag)
									appState.AddKnownTag(cleanTag)
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

	tagMatches := hashtagRegex.FindAllStringSubmatch(text, -1)
	for _, m := range tagMatches {
		if len(m) > 1 {
			tag := strings.ToLower(m[1])
			isNew := true
			for _, existing := range fileTags {
				if existing == tag {
					isNew = false
					break
				}
			}
			if isNew && tag != "" {
				fileTags = append(fileTags, tag)
				appState.AddKnownTag(tag)
			}
		}
	}

	var links []string
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

	mediaMatches := mediaLinkRegex.FindAllStringSubmatch(text, -1)
	for _, m := range mediaMatches {
		if len(m) > 1 {
			target := strings.TrimSpace(m[1])
			if target != "" {
				links = append(links, target)
			}
		}
	}

	semanticLinks := ExtractSemanticLinks(text)
	if len(semanticLinks) > 0 {
		log.Printf("[Sync] LINKS SEMANTICOS EXTRAIDOS em %s: %v\n", filename, semanticLinks)
	}

	matches := headerRegex.FindAllStringSubmatchIndex(text, -1)
	ordem := 0

	// Rastreamento de hierarquia de cabeçalhos
	headerStack := make([]string, 7) // Pilha para níveis 1-6

	formatSectionTrail := func(level int, title string) string {
		title = strings.TrimSpace(title)
		if level >= 1 && level <= 6 {
			headerStack[level] = title
			// Limpa sub-níveis
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
			return models.SectionDefault
		}
		return strings.Join(trail, " › ")
	}

	if len(matches) == 0 {
		docs = append(docs, models.Document{
			ID:         semantic.HashFunc(filename + models.SectionDefault),
			Tipo:       "markdown",
			Arquivo:    filename,
			Secao:      models.SectionDefault,
			Texto:      text,
			Timestamp:  timestampStr,
			Created:    creationStr,
			Hash:       semantic.CalculateHash(models.SectionDefault, text, fileTags),
			VectorHash: semantic.CalculateVectorHash(models.SectionDefault, text),
			Tags:       fileTags,
		})
	} else {
		lastPos := 0
		currentHeader := models.SectionDefault

		for _, m := range matches {
			contentBefore := text[lastPos:m[0]]
			if strings.TrimSpace(contentBefore) != "" {
				docs = append(docs, models.Document{
					ID:         semantic.HashFunc(filename + currentHeader + strconv.Itoa(ordem)),
					Tipo:       "markdown",
					Arquivo:    filename,
					Secao:      currentHeader,
					Texto:      contentBefore,
					Timestamp:  timestampStr,
					Created:    creationStr,
					Hash:       semantic.CalculateHash(currentHeader, contentBefore, fileTags),
					VectorHash: semantic.CalculateVectorHash(currentHeader, contentBefore),
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
		docs = append(docs, models.Document{
			ID:         semantic.HashFunc(filename + currentHeader + strconv.Itoa(ordem)),
			Tipo:       "markdown",
			Arquivo:    filename,
			Secao:      currentHeader,
			Texto:      remaining,
			Timestamp:  timestampStr,
			Created:    creationStr,
			Hash:       semantic.CalculateHash(currentHeader, remaining, fileTags),
			VectorHash: semantic.CalculateVectorHash(currentHeader, remaining),
			Tags:       fileTags,
			Ordem:      ordem,
		})
	}

	return docs, links, semanticLinks, metadata, fileTags
}

func ExtractPDFText(path string) ([]string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var pages []string
	totalPage := r.NumPage()
	for i := 1; i <= totalPage; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		s, _ := p.GetPlainText(nil)
		pages = append(pages, s)
	}
	return pages, nil
}

func ProcessPDF(path, filename string, modTime time.Time, appState *AppState) []models.Document {
	// Recuperar data de criação original (se existir)
	creationTime, exists := appState.GetFileCreation(filename)
	if !exists {
		creationTime = modTime
	}
	creationStr := creationTime.UTC().Format(time.RFC3339)
	timestampStr := modTime.UTC().Format(time.RFC3339)

	pages, err := ExtractPDFText(path)
	if err != nil {
		log.Printf("Erro ao ler PDF %s: %v\n", path, err)
		return nil
	}

	var docs []models.Document
	for i, text := range pages {
		if strings.TrimSpace(text) == "" {
			continue
		}

		tags := appState.GetFileTags(filename)
		docs = append(docs, models.Document{
			ID:         semantic.HashFunc(fmt.Sprintf("%s-p%d", filename, i+1)),
			Tipo:       "pdf",
			Arquivo:    filename,
			Secao:      fmt.Sprintf("Página %d", i+1),
			Pagina:     i + 1,
			Texto:      text,
			Timestamp:  timestampStr,
			Created:    creationStr,
			Hash:       semantic.CalculateHash(fmt.Sprintf("page-%d", i+1), text, tags),
			VectorHash: semantic.CalculateVectorHash(fmt.Sprintf("page-%d", i+1), text),
			Tags:       tags,
		})
	}
	return docs
}

func OCRImage(ctx_unused interface{}, path, key string) (string, error) {
	fileData, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	base64Image := base64.StdEncoding.EncodeToString(fileData)

	payload := map[string]interface{}{
		"requests": []map[string]interface{}{
			{
				"image": map[string]interface{}{"content": base64Image},
				"features": []map[string]interface{}{
					{"type": "TEXT_DETECTION"},
				},
			},
		},
	}
	payloadBytes, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "https://vision.googleapis.com/v1/images:annotate?key="+key, bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var visionResp struct {
		Responses []struct {
			FullTextAnnotation struct {
				Text string `json:"text"`
			} `json:"fullTextAnnotation"`
		} `json:"responses"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&visionResp); err != nil {
		return "", err
	}

	if len(visionResp.Responses) > 0 {
		return visionResp.Responses[0].FullTextAnnotation.Text, nil
	}
	return "", nil
}

func ProcessImage(path, filename string, modTime time.Time, appState *AppState) []models.Document {
	// Recuperar data de criação original (se existir)
	creationTime, exists := appState.GetFileCreation(filename)
	if !exists {
		creationTime = modTime
	}
	creationStr := creationTime.UTC().Format(time.RFC3339)
	timestampStr := modTime.UTC().Format(time.RFC3339)

	settings := appState.GetSettings()
	text := ""
	if settings.GoogleVisionKey != "" {
		res, err := OCRImage(nil, path, settings.GoogleVisionKey)
		if err == nil {
			text = res
		}
	}

	tags := appState.GetFileTags(filename)
	return []models.Document{{
		ID:         semantic.HashFunc("img-" + filename),
		Tipo:       "imagem",
		Arquivo:    filename,
		Secao:      models.SectionImages,
		Texto:      text,
		Timestamp:  timestampStr,
		Created:    creationStr,
		Hash:       semantic.CalculateHash("ocr", text, tags),
		VectorHash: semantic.CalculateVectorHash("ocr", text),
		Tags:       tags,
	}}
}
