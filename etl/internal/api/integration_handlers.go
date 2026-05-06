package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"etl/internal/utils"

	"etl/internal/ingest"
	"etl/internal/search"

	readability "codeberg.org/readeck/go-readability"
	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/horiagug/youtube-transcript-api-go/pkg/yt_transcript"
	"github.com/horiagug/youtube-transcript-api-go/pkg/yt_transcript_formatters"
)

// isYouTubeURL verifica se é URL do YouTube
func isYouTubeURL(urlStr string) bool {
	return strings.Contains(urlStr, "youtube.com") || strings.Contains(urlStr, "youtu.be")
}

// extractVideoID pega o ID do vídeo
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

// getYouTubeTranscript baixa a transcrição
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

// getYouTubeTitle extrai o título do vídeo da página
func getYouTubeTitle(videoURL string) string {
	article, err := readability.FromURL(videoURL, 10*time.Second)
	if err != nil {
		return ""
	}
	title := strings.TrimSuffix(article.Title, " - YouTube")
	title = strings.TrimSpace(title)
	return title
}

func (ctx *HandlerContext) HandleLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não suportado", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.URL == "" {
		http.Error(w, "URL é obrigatória", http.StatusBadRequest)
		return
	}

	log.Printf("[Pocket] Capturando link: %s\n", req.URL)

	var finalMarkdown string
	var slugTitle string

	if isYouTubeURL(req.URL) {
		videoID := extractVideoID(req.URL)
		if videoID == "" {
			http.Error(w, "URL YouTube inválida", http.StatusBadRequest)
			return
		}

		transcript, err := getYouTubeTranscript(videoID)
		if err != nil {
			log.Printf("[Pocket] Erro ao extrair transcrição do YouTube: %v\n", err)
			http.Error(w, fmt.Sprintf("Erro ao extrair transcrição: %v", err), http.StatusBadGateway)
			return
		}

		title := getYouTubeTitle(req.URL)
		if title == "" {
			log.Printf("[Pocket] Aviso: Não foi possível extrair o título do YouTube, usando ID do vídeo.\n")
			title = fmt.Sprintf("youtube-video-%s", videoID)
		}

		loc, _ := time.LoadLocation("America/Sao_Paulo")
		finalMarkdown = fmt.Sprintf("# %s\n\n> Fonte: [YouTube](%s)\n\n---\n\n## Transcrição\n\n%s\n\n---\n\n*Capturado em %s*", title, req.URL, transcript, time.Now().In(loc).Format("2006-01-02 15:04:05"))
		slugTitle = utils.SlugifyFilename(title)

	} else {
		log.Printf("[Pocket] Baixando conteúdo do site...\n")
		article, err := readability.FromURL(req.URL, 20*time.Second)
		if err != nil {
			log.Printf("[Pocket] Erro no download: %v\n", err)
			http.Error(w, "Falha ao extrair conteúdo do link.", http.StatusBadGateway)
			return
		}

		log.Printf("[Pocket] Download concluído: '%s' (%d bytes)\n", article.Title, len(article.Content))

		// — Fix 1: Validar conteúdo antes de salvar —
		// Rejeitar páginas de erro (404, 403, "não encontrada", etc.)
		errorIndicators := []string{
			"not found", "404", "403", "página não encontrada",
			"desculpe-nos", "erro ao carregar", "access denied",
			"page not found", "forbidden",
		}
		titleLower := strings.ToLower(article.Title)
		for _, indicator := range errorIndicators {
			if strings.Contains(titleLower, indicator) {
				log.Printf("[Pocket] Conteúdo rejeitado — página de erro detectada: '%s'\n", article.Title)
				http.Error(w, fmt.Sprintf("Página de erro detectada ('%s'). Verifique a URL e tente novamente.", article.Title), http.StatusUnprocessableEntity)
				return
			}
		}
		// Rejeitar conteúdo muito pequeno (provavelmente uma página de erro ou redirecionamento)
		if len(strings.TrimSpace(article.Content)) < 300 {
			log.Printf("[Pocket] Conteúdo rejeitado — muito pequeno (%d bytes): '%s'\n", len(article.Content), article.Title)
			http.Error(w, "Conteúdo insuficiente para salvar. A página pode estar vazia ou redirecionar para login.", http.StatusUnprocessableEntity)
			return
		}

		// Proteção contra conteúdos massivos
		if len(article.Content) > 2*1024*1024 {
			log.Printf("[Pocket] Aviso: Conteúdo muito grande, pode demorar...\n")
		}

		log.Printf("[Pocket] Convertendo para Markdown...\n")
		mdContent, err := htmltomarkdown.ConvertString(article.Content)
		if err != nil {
			http.Error(w, "Falha ao converter conteúdo", http.StatusInternalServerError)
			return
		}

		reHeaders := regexp.MustCompile(`(?m)^#+\s+(.*)$`)
		mdContent = reHeaders.ReplaceAllString(mdContent, "**$1**")

		finalMarkdown = fmt.Sprintf("# %s\n\n> Fonte: [%s](%s)\n\n---\n\n%s", article.Title, req.URL, req.URL, mdContent)
		slugTitle = utils.SlugifyFilename(article.Title)
	}

	if slugTitle == "" || slugTitle == ".md" {
		slugTitle = fmt.Sprintf("link-%d.md", time.Now().Unix())
	} else if !strings.HasSuffix(slugTitle, ".md") {
		slugTitle += ".md"
	}

	// Bug 5 Fix: Verificar se o arquivo já existe e adicionar sufixo para evitar overwrite silencioso
	linksDir := filepath.Join(ctx.Cfg.DocsDir, "links")
	base := strings.TrimSuffix(slugTitle, ".md")
	finalSlug := slugTitle
	for counter := 2; ; counter++ {
		candidate := filepath.Join(linksDir, finalSlug)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			break // nome livre encontrado
		}
		finalSlug = fmt.Sprintf("%s-%d.md", base, counter)
	}
	slugTitle = finalSlug

	path := filepath.Join(ctx.Cfg.DocsDir, "links", slugTitle)
	log.Printf("[Pocket] Salvando arquivo: %s\n", slugTitle)
	if err := os.WriteFile(path, []byte(finalMarkdown), 0644); err != nil {
		log.Printf("[Pocket] Erro ao salvar arquivo: %v\n", err)
		http.Error(w, "Erro ao salvar", http.StatusInternalServerError)
		return
	}

	relFilename := "links/" + slugTitle
	log.Printf("[Pocket] Captura finalizada com sucesso: %s\n", relFilename)
	search.ClearCache()

	// — UX Fix: Disparar sync imediato para que o link apareça na busca instantaneamente —
	go ingest.RunSync(ctx.Cfg, false, "auto", ctx.State)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "success",
		"filename": relFilename,
	})
}
