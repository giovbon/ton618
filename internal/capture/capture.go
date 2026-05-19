package capture

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	readability "codeberg.org/readeck/go-readability"
	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/horiagug/youtube-transcript-api-go/pkg/yt_transcript"
	"github.com/horiagug/youtube-transcript-api-go/pkg/yt_transcript_formatters"

	"ton618/internal/config"
	"ton618/internal/db"
	"ton618/internal/semantic"
	"ton618/internal/watcher"
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

// getYouTubeTitle extrai o titulo do video da pagina.
func getYouTubeTitle(videoURL string) string {
	article, err := readability.FromURL(videoURL, 10*time.Second)
	if err != nil {
		return ""
	}
	title := strings.TrimSuffix(article.Title, " - YouTube")
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

// HandlerContext contem as dependencias para o handler de captura.
type HandlerContext struct {
	Cfg       *config.AppConfig
	Store     *db.Store
	Embed     semantic.EmbeddingProvider
	EmbedAll  bool
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
		// ── YouTube: transcricao ──
		videoID := extractVideoID(req.URL)
		if videoID == "" {
			http.Error(w, "URL do YouTube invalida", http.StatusBadRequest)
			return
		}

		// Tenta obter o titulo primeiro
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
tags: [embed, youtube, captura]
source: %s
---

# %s

> Fonte: [YouTube](%s)

## Transcricao

%s

---

*Capturado em %s*`, title, req.URL, title, req.URL, transcript, time.Now().Format("2006-01-02 15:04:05"))
	} else {
		// ── Artigo web ──
		article, err := readability.FromURL(req.URL, 20*time.Second)
		if err != nil {
			slog.Error("erro ao capturar artigo", "error", err)
			http.Error(w, fmt.Sprintf("Falha ao capturar: %v", err), http.StatusBadGateway)
			return
		}

		// Validar conteudo
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

		finalMarkdown = fmt.Sprintf(`---
title: "%s"
tags: [embed, artigo, captura]
source: %s
---

# %s

> Fonte: [%s](%s)

%s

---

*Capturado em %s*`, title, req.URL, title, req.URL, req.URL, mdContent, time.Now().Format("2006-01-02 15:04:05"))
	}

	// Slug para nome do arquivo
	slug := slugifyFilename(title)
	if slug == "" || slug == "-" {
		slug = fmt.Sprintf("captura-%d", time.Now().Unix())
	}

	filename := fmt.Sprintf("notes/captura-%s.md", slug)

	// Verificar se ja existe e adicionar sufixo
	base := slug
	finalFilename := filename
	for counter := 2; ; counter++ {
		fullPath := filepath.Join(ctx.Cfg.DocsDir, finalFilename)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			break
		}
		finalFilename = fmt.Sprintf("notes/captura-%s-%d.md", base, counter)
	}
	filename = finalFilename

	// Escrever arquivo
	fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	if err := os.WriteFile(fullPath, []byte(finalMarkdown), 0644); err != nil {
		slog.Error("erro ao salvar captura", "error", err)
		http.Error(w, "Erro ao salvar", http.StatusInternalServerError)
		return
	}

	slog.Info("Captura salva", "file", filename)

	// Indexar e gerar embedding (forcando embedAll=true para garantir)
	ev := watcher.FileEvent{
		Path:     fullPath,
		Filename: filename,
		ModTime:  time.Now(),
		Type:     "modify",
	}
	if err := watcher.ProcessFile(ctx.Store, ev, ctx.Embed, true); err != nil {
		slog.Error("processar captura", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "success",
		"filename": filename,
	})
}
