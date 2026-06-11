package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	readability "codeberg.org/readeck/go-readability"
	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/horiagug/youtube-transcript-api-go/pkg/yt_transcript"
	"github.com/horiagug/youtube-transcript-api-go/pkg/yt_transcript_formatters"
)

// isYouTubeURL verifica se é URL do YouTube.
func isYouTubeURL(urlStr string) bool {
	return strings.Contains(urlStr, "youtube.com") || strings.Contains(urlStr, "youtu.be")
}

// extractVideoID extrai o ID do vídeo de uma URL do YouTube.
func extractVideoID(urlStr string) string {
	u, _ := url.Parse(urlStr)
	if u != nil {
		if id := u.Query().Get("v"); id != "" {
			return id
		}
	}
	if strings.Contains(urlStr, "youtu.be") {
		parts := strings.Split(urlStr, "youtu.be/")
		if len(parts) > 1 {
			return strings.Split(parts[1], "?")[0]
		}
	}
	return ""
}

// getYouTubeTranscript baixa a transcricao de um video.
func getYouTubeTranscript(videoID string) (string, error) {
	textFormatter := yt_transcript_formatters.NewTextFormatter(
		yt_transcript_formatters.WithTimestamps(false),
	)
	client := yt_transcript.NewClient(
		yt_transcript.WithFormatter(textFormatter),
	)
	transcript, err := client.GetFormattedTranscripts(videoID, []string{"pt-BR", "pt", "en"}, true)
	if err != nil {
		return "", err
	}
	return transcript, nil
}

// getYouTubeTitle extrai o titulo do video via HTTP direto e regex no <title>.
func getYouTubeTitle(videoURL string) string {
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(videoURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	re := regexp.MustCompile(`<title>([^<]+)</title>`)
	matches := re.FindStringSubmatch(string(body))
	if len(matches) < 2 {
		return ""
	}
	title := strings.TrimSpace(matches[1])
	title = strings.TrimSuffix(title, " - YouTube")
	title = strings.TrimSpace(title)
	return title
}

// slugifyFilename gera um slug seguro para nome de arquivo.
func slugifyFilename(title string) string {
	lower := strings.ToLower(title)
	re := regexp.MustCompile(`[^a-z0-9À-ÿ]+`)
	slug := re.ReplaceAllString(lower, "-")
	slug = strings.Trim(slug, "-")
	slug = regexp.MustCompile(`-{2,}`).ReplaceAllString(slug, "-")
	if len(slug) > 60 {
		slug = slug[:60]
	}
	slug = strings.TrimRight(slug, "-")
	if slug == "" {
		slug = "captura"
	}
	return slug
}

// cleanupMarkdown remove artefatos e texto duplicado gerado pelo html-to-markdown.
// Também remove tags HTML residuais que o conversor pode deixar no output.
func cleanupMarkdown(md string) string {
	// Remove tags HTML residuais que o conversor pode não tratar (e.g. <p>, </p>)
	reHTML := regexp.MustCompile(`</?[a-zA-Z][^>]*>`)
	md = reHTML.ReplaceAllString(md, "")

	// Corrige entidades HTML comuns
	md = strings.ReplaceAll(md, "&gt;", ">")
	md = strings.ReplaceAll(md, "&lt;", "<")
	md = strings.ReplaceAll(md, "&amp;", "&")
	md = strings.ReplaceAll(md, "&quot;", "\"")
	md = strings.ReplaceAll(md, "&#39;", "'")

	lines := strings.Split(md, "\n")
	var cleaned []string
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if i > 0 && trimmed != "" {
			prev := strings.TrimSpace(lines[i-1])
			if strings.Contains(prev, "![") && strings.Contains(prev, trimmed) {
				continue
			}
			if strings.HasPrefix(prev, "!") && len(trimmed) < 120 {
				continue
			}
		}
		if strings.Contains(trimmed, "youtube.com/embed") || strings.Contains(trimmed, "player.vimeo.com") {
			continue
		}
		cleaned = append(cleaned, line)
	}
	md = strings.Join(cleaned, "\n")
	re := regexp.MustCompile(`\n{3,}`)
	md = re.ReplaceAllString(md, "\n\n")
	return strings.TrimSpace(md)
}

// HandleCapture processa uma URL (artigo web ou video YouTube) e salva como nota.
func (ctx *HandlerContext) HandleCapture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON invalido", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "URL obrigatoria", http.StatusBadRequest)
		return
	}

	slog.Info("Capturando", "url", req.URL)

	var finalMarkdown string
	var title string

	if isYouTubeURL(req.URL) {
		videoID := extractVideoID(req.URL)
		if videoID == "" {
			http.Error(w, "URL do YouTube invalida", http.StatusBadRequest)
			return
		}

		title = getYouTubeTitle(req.URL)
		if title == "" {
			title = fmt.Sprintf("YouTube - %s", videoID)
		}

		transcript, err := getYouTubeTranscript(videoID)
		if err != nil {
			slog.Warn("transcricao nao disponivel", "video", videoID, "error", err)
			transcript = "*Transcricao nao disponivel para este video.*"
		}

		finalMarkdown = fmt.Sprintf(`---
title: "%s"
tags: [captura, keywords]
---

# %s

> Fonte: [YouTube](%s)

## Transcricao

%s

---

*Capturado em %s*`, title, title, req.URL, transcript, formatCaptureTimestamp(time.Now()))
	} else {
		article, err := readability.FromURL(req.URL, 20*time.Second)
		if err != nil {
			slog.Error("erro ao capturar artigo", "error", err)
			http.Error(w, fmt.Sprintf("Falha ao capturar: %v", err), http.StatusBadGateway)
			return
		}

		errorIndicators := []string{
			"not found", "404", "403", "pagina nao encontrada",
			"access denied", "forbidden",
		}
		titleLower := strings.ToLower(article.Title)
		for _, indicator := range errorIndicators {
			if strings.Contains(titleLower, indicator) {
				http.Error(w, fmt.Sprintf("Pagina de erro detectada: %s", article.Title), http.StatusUnprocessableEntity)
				return
			}
		}
		if len(strings.TrimSpace(article.Content)) < 300 {
			http.Error(w, "Conteudo insuficiente", http.StatusUnprocessableEntity)
			return
		}

		title = article.Title

		mdContent, err := htmltomarkdown.ConvertString(article.Content)
		if err != nil {
			http.Error(w, "Falha ao converter HTML", http.StatusInternalServerError)
			return
		}

		mdContent = cleanupMarkdown(mdContent)

		finalMarkdown = fmt.Sprintf(`---
title: "%s"
tags: [captura, keywords]
---

# %s

> Fonte: [%s](%s)

%s

---

*Capturado em %s*`, title, title, req.URL, req.URL, mdContent, formatCaptureTimestamp(time.Now()))
	}

	slug := slugifyFilename(title)
	if slug == "" || slug == "-" {
		slug = fmt.Sprintf("captura-%d", time.Now().Unix())
	}

	filename := fmt.Sprintf("notes/captura-%s.md", slug)

	base := slug
	finalFilename := filename
	for counter := 2; ; counter++ {
		if !ctx.Store.NoteExists(finalFilename) {
			break
		}
		finalFilename = fmt.Sprintf("notes/captura-%s-%d.md", base, counter)
	}
	filename = finalFilename

	mtime := time.Now()
	if err := ctx.Store.SaveNote(filename, finalMarkdown, mtime.Format(time.RFC3339)); err != nil {
		slog.Error("erro ao salvar captura no DB", "error", err)
		http.Error(w, "Erro ao salvar", http.StatusInternalServerError)
		return
	}
	if err := ctx.reindexNote(filename, finalMarkdown, mtime); err != nil {
		slog.Error("reindexar captura", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "success",
		"filename": filename,
	})
}

func formatCaptureTimestamp(t time.Time) string {
	return t.Format("2006-01-02 15:04:05 MST")
}
