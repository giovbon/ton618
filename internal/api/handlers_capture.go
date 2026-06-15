package api

import (
	"encoding/base64"
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

// withBrowserHeaders define headers de um navegador real para evitar bloqueios por WAF/Cloudflare.
func withBrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
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

	// Tenta decodificar o URL caso ele venha codificado em base64 (para evitar bloqueios de WAF/Cloudflare)
	rawURL := req.URL
	if decodedBytes, err := base64.StdEncoding.DecodeString(req.URL); err == nil {
		decodedStr := string(decodedBytes)
		// Se foi codificado com encodeURIComponent no JS, ele estará percent-encoded. Tentamos fazer unescape.
		if unescaped, err := url.QueryUnescape(decodedStr); err == nil {
			if strings.HasPrefix(unescaped, "http://") || strings.HasPrefix(unescaped, "https://") ||
				strings.Contains(unescaped, "youtube.com") || strings.Contains(unescaped, "youtu.be") {
				rawURL = unescaped
			}
		} else if strings.HasPrefix(decodedStr, "http://") || strings.HasPrefix(decodedStr, "https://") ||
			strings.Contains(decodedStr, "youtube.com") || strings.Contains(decodedStr, "youtu.be") {
			rawURL = decodedStr
		}
	}

	slog.Info("Capturando", "url", rawURL)

	var finalMarkdown string
	var title string

	if isYouTubeURL(rawURL) {
		videoID := extractVideoID(rawURL)
		if videoID == "" {
			http.Error(w, "URL do YouTube invalida", http.StatusBadRequest)
			return
		}

		title = getYouTubeTitle(rawURL)
		if title == "" {
			title = fmt.Sprintf("YouTube - %s", videoID)
		}

		transcript, err := getYouTubeTranscript(videoID)
		if err != nil {
			slog.Warn("transcricao nao disponivel", "video", videoID, "error", err)
			transcript = "*Transcricao nao disponivel para este video.*"
		}

		finalMarkdown = fmt.Sprintf(`---
tags: [keywords]
---

# %s

> Fonte: [YouTube](%s)

## Transcricao

%s

---

*Capturado em %s*`, title, rawURL, transcript, formatCaptureTimestamp(time.Now()))
	} else {
		article, err := readability.FromURL(rawURL, 12*time.Second, withBrowserHeaders)
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
tags: [keywords]
---

# %s

> Fonte: [%s](%s)

%s

---

*Capturado em %s*`, title, req.URL, req.URL, mdContent, formatCaptureTimestamp(time.Now()))
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
