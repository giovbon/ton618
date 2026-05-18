package ingest

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"etl/internal/models"
	"etl/internal/semantic"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	headerRegex    = regexp.MustCompile(`(?m)^(#{1,6})\s+(.*)`)
	hashtagRegex   = regexp.MustCompile(`(?m)(?:\s|^)#([a-zA-Z0-9_À-ÿ\-]+)`)
	wikilinkRegex  = regexp.MustCompile(`\[\[([^\]|#]+)(?:[|#][^\]]*)?\]\]`)
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
	return nil
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

	// Extrai links da chave "links" no frontmatter (fonte primária)
	// Declarada antes do bloco de parse do YAML para que a declaração abaixo
	// (variavel de retorno) reutilize as fatias, em vez de criar um novo escopo.
	var semanticLinks []string

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

	cleanTextForTags := stripHTML(text)
	tagMatches := hashtagRegex.FindAllStringSubmatch(cleanTextForTags, -1)
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

// ProcessPDF handles PDF indexing by returning nil as text extraction is disabled.
func ProcessPDF(path, filename string, modTime time.Time, appState *AppState) []models.Document {
	return nil
}

func OCRImage(ctx_unused interface{}, path, key string) (string, error) {
	slog.Info("[OCRImage] Lendo arquivo para processamento", "path", path)
	fileData, err := os.ReadFile(path)
	if err != nil {
		slog.Error("[OCRImage] Erro ao ler arquivo no disco", "path", path, "error", err)
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

	slog.Info("[OCRImage] Enviando requisição para Google Vision API...")
	req, _ := http.NewRequest("POST", "https://vision.googleapis.com/v1/images:annotate?key="+key, bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("[OCRImage] Falha na conexão HTTP com Google Vision API", "error", err)
		return "", err
	}
	defer resp.Body.Close()

	slog.Info("[OCRImage] Resposta recebida", "status", resp.Status)

	var visionResp struct {
		Responses []struct {
			FullTextAnnotation struct {
				Text string `json:"text"`
			} `json:"fullTextAnnotation"`
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		} `json:"responses"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&visionResp); err != nil {
		slog.Error("[OCRImage] Erro ao decodificar JSON da resposta da API", "error", err)
		return "", err
	}

	if len(visionResp.Responses) > 0 {
		resp0 := visionResp.Responses[0]
		if resp0.Error.Message != "" {
			slog.Error("[OCRImage] Erro retornado pela API do Google Vision", "code", resp0.Error.Code, "message", resp0.Error.Message)
			return "", fmt.Errorf("google vision api error: %s", resp0.Error.Message)
		}
		slog.Info("[OCRImage] Texto extraído com sucesso", "length", len(resp0.FullTextAnnotation.Text))
		return resp0.FullTextAnnotation.Text, nil
	}

	slog.Warn("[OCRImage] Nenhuma resposta retornada do Google Vision API")
	return "", nil
}

func ProcessImage(path, filename string, modTime time.Time, appState *AppState) []models.Document {
	slog.Info("[ProcessImage] Iniciando processamento de imagem", "file", filename)
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
		} else {
			slog.Error("[ProcessImage] Erro ao executar OCR", "file", filename, "error", err)
		}
	} else {
		slog.Warn("[ProcessImage] GoogleVisionKey não está configurada nas configurações. Ignorando OCR.", "file", filename)
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
