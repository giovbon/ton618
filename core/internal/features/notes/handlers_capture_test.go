package notes

import (
	"strings"
	"testing"
	"time"
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
