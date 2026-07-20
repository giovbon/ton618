package notes

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	readability "codeberg.org/readeck/go-readability"
	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/horiagug/youtube-transcript-api-go/pkg/yt_transcript"
	"github.com/horiagug/youtube-transcript-api-go/pkg/yt_transcript_formatters"

	"ton618/core/internal/core/db"
)

var (
	youtubeTitleRe      = regexp.MustCompile(`<title>([^<]+)</title>`)
	slugifyRe           = regexp.MustCompile(`[^a-z0-9À-ÿ]+`)
	dashRe              = regexp.MustCompile(`-{2,}`)
	htmlTagRe           = regexp.MustCompile(`</?[a-zA-Z][^>]*>`)
	multipleNewlineRe   = regexp.MustCompile(`\n{3,}`)
)

// CaptureService lida com a captura de URLs (artigos web e YouTube).
type CaptureService struct {
	store *db.Store
}

func NewCaptureService(store *db.Store) *CaptureService {
	return &CaptureService{store: store}
}

// CaptureResult contém o resultado de uma captura de URL.
type CaptureResult struct {
	Title    string
	Filename string
	Markdown string
}

// CaptureURL processa uma URL e retorna o markdown pronto para salvar.
func (s *CaptureService) CaptureURL(rawURL string) (*CaptureResult, error) {
	if isYouTubeURL(rawURL) {
		return s.captureYouTube(rawURL)
	}
	return s.captureArticle(rawURL)
}

func (s *CaptureService) captureYouTube(rawURL string) (*CaptureResult, error) {
	videoID := extractVideoID(rawURL)
	if videoID == "" {
		return nil, fmt.Errorf("URL do YouTube invalida")
	}

	title := getYouTubeTitle(rawURL)
	if title == "" {
		title = fmt.Sprintf("YouTube - %s", videoID)
	}

	transcript, err := getYouTubeTranscript(videoID)
	if err != nil {
		transcript = "*Transcricao nao disponivel para este video.*"
	}

	markdown := fmt.Sprintf(`---
tags: []
---

# %s

> Fonte: [YouTube](%s)

## Transcricao

%s

---

*Capturado em %s*`, title, rawURL, transcript, formatCaptureTimestamp(time.Now()))

	filename := s.uniqueFilename(slugifyFilename(title))
	return &CaptureResult{Title: title, Filename: filename, Markdown: markdown}, nil
}

func (s *CaptureService) captureArticle(rawURL string) (*CaptureResult, error) {
	article, err := readability.FromURL(rawURL, 12*time.Second, withBrowserHeaders)
	if err != nil {
		return nil, fmt.Errorf("falha ao capturar: %w", err)
	}

	errorIndicators := []string{
		"not found", "404", "403", "pagina nao encontrada",
		"access denied", "forbidden",
	}
	titleLower := strings.ToLower(article.Title)
	for _, indicator := range errorIndicators {
		if strings.Contains(titleLower, indicator) {
			return nil, fmt.Errorf("pagina de erro detectada: %s", article.Title)
		}
	}
	if len(strings.TrimSpace(article.Content)) < 300 {
		return nil, fmt.Errorf("conteudo insuficiente")
	}

	mdContent, err := htmltomarkdown.ConvertString(article.Content)
	if err != nil {
		return nil, fmt.Errorf("falha ao converter HTML: %w", err)
	}
	mdContent = cleanupMarkdown(mdContent)

	markdown := fmt.Sprintf(`---
tags: []
---

# %s

> Fonte: [%s](%s)

%s

---

*Capturado em %s*`, article.Title, rawURL, rawURL, mdContent, formatCaptureTimestamp(time.Now()))

	filename := s.uniqueFilename(slugifyFilename(article.Title))
	return &CaptureResult{Title: article.Title, Filename: filename, Markdown: markdown}, nil
}

// uniqueFilename garante um nome único incrementando sufixo se necessário.
func (s *CaptureService) uniqueFilename(slug string) string {
	if slug == "" || slug == "-" {
		slug = fmt.Sprintf("captura-%d", time.Now().Unix())
	}
	filename := fmt.Sprintf("notes/captura-%s.md", slug)
	for counter := 2; s.store.NoteExists(filename); counter++ {
		filename = fmt.Sprintf("notes/captura-%s-%d.md", slug, counter)
	}
	return filename
}

// ── Helpers ────────────────────────────────────────────────────────────

func isYouTubeURL(urlStr string) bool {
	return strings.Contains(urlStr, "youtube.com") || strings.Contains(urlStr, "youtu.be")
}

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
	matches := youtubeTitleRe.FindStringSubmatch(string(body))
	if len(matches) < 2 {
		return ""
	}
	title := strings.TrimSpace(matches[1])
	title = strings.TrimSuffix(title, " - YouTube")
	title = strings.TrimSpace(title)
	return title
}

func slugifyFilename(title string) string {
	lower := strings.ToLower(title)
	slug := slugifyRe.ReplaceAllString(lower, "-")
	slug = strings.Trim(slug, "-")
	slug = dashRe.ReplaceAllString(slug, "-")
	if len(slug) > 60 {
		slug = slug[:60]
	}
	slug = strings.TrimRight(slug, "-")
	if slug == "" {
		slug = "captura"
	}
	return slug
}

func cleanupMarkdown(md string) string {
	md = htmlTagRe.ReplaceAllString(md, "")
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
	md = multipleNewlineRe.ReplaceAllString(md, "\n\n")
	return strings.TrimSpace(md)
}

func withBrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
}

func formatCaptureTimestamp(t time.Time) string {
	return t.Format("02/01/2006 15:04:05")
}

// DecodeCaptureURL tenta decodificar uma URL que pode vir em base64 (anti-WAF).
func DecodeCaptureURL(encodedURL string) string {
	if decodedBytes, err := base64.StdEncoding.DecodeString(encodedURL); err == nil {
		decodedStr := string(decodedBytes)
		if unescaped, err := url.QueryUnescape(decodedStr); err == nil {
			if looksLikeURL(unescaped) {
				return unescaped
			}
		}
		if looksLikeURL(decodedStr) {
			return decodedStr
		}
	}
	return encodedURL
}

func looksLikeURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") ||
		strings.Contains(s, "youtube.com") || strings.Contains(s, "youtu.be")
}
