package notes

import (
	"strings"
	"testing"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
)

// ── isYouTubeURL ────────────────────────────────────────────────

func TestIsYouTubeURL_YouTubeCom(t *testing.T) {
	if !isYouTubeURL("https://www.youtube.com/watch?v=dQw4w9WgXcQ") {
		t.Error("youtube.com URL deve retornar true")
	}
}

func TestIsYouTubeURL_YouTubeComShort(t *testing.T) {
	if !isYouTubeURL("https://youtube.com/watch?v=dQw4w9WgXcQ") {
		t.Error("youtube.com (sem www) URL deve retornar true")
	}
}

func TestIsYouTubeURL_YouTubeBe(t *testing.T) {
	if !isYouTubeURL("https://youtu.be/dQw4w9WgXcQ") {
		t.Error("youtu.be URL deve retornar true")
	}
}

func TestIsYouTubeURL_NotYouTube(t *testing.T) {
	if isYouTubeURL("https://example.com/video") {
		t.Error("URL nao-YouTube deve retornar false")
	}
}

func TestIsYouTubeURL_Empty(t *testing.T) {
	if isYouTubeURL("") {
		t.Error("URL vazia deve retornar false")
	}
}

// ── extractVideoID ──────────────────────────────────────────────

func TestExtractVideoID_Standard(t *testing.T) {
	id := extractVideoID("https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	if id != "dQw4w9WgXcQ" {
		t.Errorf("esperado dQw4w9WgXcQ, got %q", id)
	}
}

func TestExtractVideoID_WithExtraParams(t *testing.T) {
	id := extractVideoID("https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=120s")
	if id != "dQw4w9WgXcQ" {
		t.Errorf("esperado dQw4w9WgXcQ, got %q", id)
	}
}

func TestExtractVideoID_ShortURL(t *testing.T) {
	id := extractVideoID("https://youtu.be/dQw4w9WgXcQ")
	if id != "dQw4w9WgXcQ" {
		t.Errorf("esperado dQw4w9WgXcQ, got %q", id)
	}
}

func TestExtractVideoID_ShortURLWithParams(t *testing.T) {
	id := extractVideoID("https://youtu.be/dQw4w9WgXcQ?si=abc123")
	if id != "dQw4w9WgXcQ" {
		t.Errorf("esperado dQw4w9WgXcQ, got %q", id)
	}
}

func TestExtractVideoID_NoID(t *testing.T) {
	id := extractVideoID("https://www.youtube.com/")
	if id != "" {
		t.Errorf("esperado vazio, got %q", id)
	}
}

func TestExtractVideoID_InvalidURL(t *testing.T) {
	id := extractVideoID("not-a-url")
	if id != "" {
		t.Errorf("esperado vazio, got %q", id)
	}
}

// ── slugifyFilename ─────────────────────────────────────────────

func TestSlugifyFilename_Basic(t *testing.T) {
	slug := slugifyFilename("Hello World")
	if slug != "hello-world" {
		t.Errorf("esperado 'hello-world', got %q", slug)
	}
}

func TestSlugifyFilename_Accents(t *testing.T) {
	slug := slugifyFilename("Aplicação Web")
	if slug != "aplicação-web" {
		t.Errorf("esperado 'aplicação-web', got %q", slug)
	}
}

func TestSlugifyFilename_SpecialChars(t *testing.T) {
	slug := slugifyFilename("Hello! World? (Test)")
	if slug != "hello-world-test" {
		t.Errorf("esperado 'hello-world-test', got %q", slug)
	}
}

func TestSlugifyFilename_Empty(t *testing.T) {
	slug := slugifyFilename("")
	if slug != "captura" {
		t.Errorf("esperado 'captura', got %q", slug)
	}
}

func TestSlugifyFilename_OnlySpecialChars(t *testing.T) {
	slug := slugifyFilename("!!! ???")
	if slug != "captura" {
		t.Errorf("esperado 'captura', got %q", slug)
	}
}

func TestSlugifyFilename_Truncate(t *testing.T) {
	long := strings.Repeat("a", 100)
	slug := slugifyFilename(long)
	if len(slug) > 60 {
		t.Errorf("slug nao deve exceder 60 caracteres, tem %d", len(slug))
	}
	if !strings.HasPrefix(slug, strings.Repeat("a", 60)) {
		t.Errorf("esperado 60 'a's, got %q (len=%d)", slug, len(slug))
	}
}

func TestSlugifyFilename_TrimLeadingTrailingHyphens(t *testing.T) {
	slug := slugifyFilename("---hello---world---")
	if slug != "hello-world" || strings.HasPrefix(slug, "-") || strings.HasSuffix(slug, "-") {
		t.Errorf("esperado 'hello-world', got %q", slug)
	}
}

func TestSlugifyFilename_RemoveConsecutiveHyphens(t *testing.T) {
	slug := slugifyFilename("hello    world  test")
	if slug != "hello-world-test" {
		t.Errorf("esperado 'hello-world-test', got %q", slug)
	}
}

// ── cleanupMarkdown ─────────────────────────────────────────────

func TestCleanupMarkdown_ResidualHTML(t *testing.T) {
	md := "<p>Hello world</p>"
	result := cleanupMarkdown(md)
	if !strings.Contains(result, "Hello world") {
		t.Errorf("deve conter 'Hello world', got %q", result)
	}
	if strings.Contains(result, "<p>") {
		t.Errorf("nao deve conter tags HTML, got %q", result)
	}
}

func TestCleanupMarkdown_HTMLEntities(t *testing.T) {
	md := "Hello &gt; world &amp; foo"
	result := cleanupMarkdown(md)
	if !strings.Contains(result, "> world & foo") {
		t.Errorf("esperado '> world & foo', got %q", result)
	}
}

func TestCleanupMarkdown_RemoveEmbedURLs(t *testing.T) {
	md := "Some text\nyoutube.com/embed/abc123\nmore text"
	result := cleanupMarkdown(md)
	if strings.Contains(result, "youtube.com/embed") {
		t.Errorf("nao deve conter embed URLs, got %q", result)
	}
}

func TestCleanupMarkdown_CollapseNewlines(t *testing.T) {
	md := "Line one\n\n\n\n\nLine two"
	result := cleanupMarkdown(md)
	lines := strings.Split(result, "\n")
	// Should have at most one blank line between
	blankCount := 0
	for _, l := range lines {
		if l == "" {
			blankCount++
		}
	}
	if blankCount > 2 {
		t.Errorf("excesso de linhas em branco (%d), resultado: %q", blankCount, result)
	}
}

func TestCleanupMarkdown_DuplicateImageDescription(t *testing.T) {
	md := "![alt](image.png)\nalt"
	result := cleanupMarkdown(md)
	// A linha duplicada "alt" apos a imagem deve ser removida
	lines := strings.Split(result, "\n")
	if len(lines) > 1 {
		t.Errorf("linha duplicada de descricao de imagem nao foi removida, resultado: %q", result)
	}
}

func TestCleanupMarkdown_MultipleImages(t *testing.T) {
	md := "![img1](https://site.com/1.jpg)\n![img2](https://site.com/2.jpg)\nLegenda curta"
	result := cleanupMarkdown(md)
	if !strings.Contains(result, "1.jpg") {
		t.Errorf("deve conter imagem 1, got %q", result)
	}
	if !strings.Contains(result, "2.jpg") {
		t.Errorf("deve conter imagem 2, got %q", result)
	}
	if !strings.Contains(result, "Legenda curta") {
		t.Errorf("deve conter legenda curta, got %q", result)
	}
}

func TestPreprocessHTMLForCapture_RelativeURLAndLazyLoading(t *testing.T) {
	rawURL := "https://example.com/blog/article-1"
	html := `<div>
		<img src="/img/relative.png" alt="Relativa" />
		<img src="data:image/svg+xml;base64,AAA" data-src="https://cdn.example.com/real.jpg" alt="Lazy" />
		<img src="" srcset="https://cdn.example.com/pic-800.jpg 800w, https://cdn.example.com/pic-400.jpg 400w" alt="Srcset" />
	</div>`

	processed := preprocessHTMLForCapture(html, rawURL)

	if !strings.Contains(processed, "https://example.com/img/relative.png") {
		t.Errorf("esperado URL relativa resolvida para https://example.com/img/relative.png, got: %s", processed)
	}
	if !strings.Contains(processed, "https://cdn.example.com/real.jpg") {
		t.Errorf("esperado data-src promovido a src, got: %s", processed)
	}
	if !strings.Contains(processed, "https://cdn.example.com/pic-800.jpg") {
		t.Errorf("esperado srcset promovido a src, got: %s", processed)
	}
}

func TestHTMLToMarkdown_Images(t *testing.T) {
	htmlStr := `<div>
		<p>Texto do artigo</p>
		<img src="https://example.com/foto.jpg" alt="Minha Foto" />
		<figure>
			<img src="https://example.com/figura.jpg" alt="Figura" />
			<figcaption>Legenda da figura</figcaption>
		</figure>
	</div>`

	processed := preprocessHTMLForCapture(htmlStr, "https://example.com/article")
	mdContent, err := htmltomarkdown.ConvertString(processed)
	if err != nil {
		t.Fatalf("erro ao converter: %v", err)
	}
	cleaned := cleanupMarkdown(mdContent)

	t.Logf("Markdown gerado:\n%s", cleaned)
	if !strings.Contains(cleaned, "https://example.com/foto.jpg") {
		t.Errorf("markdown deve conter foto.jpg, got:\n%s", cleaned)
	}
	if !strings.Contains(cleaned, "https://example.com/figura.jpg") {
		t.Errorf("markdown deve conter figura.jpg, got:\n%s", cleaned)
	}
}

// ── formatCaptureTimestamp ──────────────────────────────────────

func TestFormatCaptureTimestamp_Formato(t *testing.T) {
	// Use a fixed time for reproducibility
	fixed := time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)
	result := formatCaptureTimestamp(fixed)
	expected := "15/06/2025 14:30:00"
	if result != expected {
		t.Errorf("esperado %q, got %q", expected, result)
	}
}

// TestUniqueFilename_SemPrefixoCaptura garante que os nomes de captura novos
// não usam o prefixo "captura-" (removido em 17/08/2026 para manter a
// exibição uniforme). O fallback de slug vazio também não usa o prefixo.
func TestUniqueFilename_SemPrefixoCaptura(t *testing.T) {
	ctx := newTestContext(t)
	svc := NewCaptureService(ctx.Store)

	// Nome normal: sem o prefixo captura-
	f := svc.uniqueFilename("terremoto-de-magnitude-7-4")
	if f != "notes/terremoto-de-magnitude-7-4.md" {
		t.Errorf("esperado sem prefixo captura-, got %q", f)
	}

	// Colisão: incrementa sufixo sem prefixo
	now := time.Now().Format(time.RFC3339)
	if err := ctx.Store.SaveNote("notes/terremoto-de-magnitude-7-4.md", "# x", now); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	f2 := svc.uniqueFilename("terremoto-de-magnitude-7-4")
	if f2 != "notes/terremoto-de-magnitude-7-4-2.md" {
		t.Errorf("esperado sufixo -2, got %q", f2)
	}

	// Fallback slug vazio: usa nota-<timestamp>, sem captura-
	f3 := svc.uniqueFilename("")
	if strings.HasPrefix(f3, "notes/captura-") {
		t.Errorf("fallback não deveria ter captura-, got %q", f3)
	}
	if !strings.HasPrefix(f3, "notes/nota-") {
		t.Errorf("fallback deveria ser notes/nota-..., got %q", f3)
	}
}
