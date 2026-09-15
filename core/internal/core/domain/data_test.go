package domain

import (
	"strings"
	"testing"
	"time"
)

// TestDisplayName_RemoveCapturaPrefix garante que o prefixo interno "captura-"
// não aparece no nome exibido (editor, banco de dados, backlinks, busca),
// mantendo uniformidade com a sidebar — que já o removia.
func TestDisplayName_RemoveCapturaPrefix(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"captura com caminho", "notes/captura-terremoto-7-4.md", "terremoto-7-4.md"},
		{"captura sem caminho", "captura-artigo-web.md", "artigo-web.md"},
		{"nota normal mantém", "notes/nota-qualquer.md", "nota-qualquer.md"},
		{"pdf mantém", "pdfs/doc.pdf", "doc.pdf"},
		{"anexo mantém", "attachments/arquivo.zip", "arquivo.zip"},
		{"captura em subcaminho", "notes/captura-a.md", "a.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DisplayName(tc.in); got != tc.want {
				t.Errorf("DisplayName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestDetectNoteType(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		arquivo  string
		expected NoteType
	}{
		{name: "Explicit tag drawing", tags: []string{"drawing"}, arquivo: "notes/x.md", expected: NoteTypeDrawing},
		{name: "Explicit tag desenho", tags: []string{"desenho"}, arquivo: "notes/x.md", expected: NoteTypeDrawing},
		{name: "Explicit tag markmap", tags: []string{"markmap"}, arquivo: "notes/x.md", expected: NoteTypeMindmap},
		{name: "Explicit tag mindmap", tags: []string{"mindmap"}, arquivo: "notes/x.md", expected: NoteTypeMindmap},
		{name: "Explicit tag youtube", tags: []string{"youtube"}, arquivo: "notes/x.md", expected: NoteTypeYoutube},
		{name: "Explicit tag artigo", tags: []string{"artigo"}, arquivo: "notes/x.md", expected: NoteTypeArticle},
		{name: "Explicit tag captura", tags: []string{"captura"}, arquivo: "notes/x.md", expected: NoteTypeCapture},
		{name: "Explicit tag semanal", tags: []string{"semanal"}, arquivo: "notes/x.md", expected: NoteTypeSemanal},
		{name: "Explicit tag semana", tags: []string{"semana"}, arquivo: "notes/x.md", expected: NoteTypeSemanal},
		{name: "Explicit tag weekly", tags: []string{"weekly"}, arquivo: "notes/x.md", expected: NoteTypeSemanal},
		{name: "Filename semanal", tags: nil, arquivo: "notes/2026-S38.md", expected: NoteTypeSemanal},
		{name: "Filename semanal sem prefixo notes/", tags: nil, arquivo: "2026-S01.md", expected: NoteTypeSemanal},
		{name: "Filename semanal semana 53", tags: nil, arquivo: "notes/2026-S53.md", expected: NoteTypeSemanal},
		{name: "Nome parecido mas semana inválida", tags: nil, arquivo: "notes/2026-S54.md", expected: NoteTypeMarkdown},
		{name: "Ano de 3 dígitos não é semanal", tags: nil, arquivo: "notes/202-S38.md", expected: NoteTypeMarkdown},
		{name: "PDF path", tags: nil, arquivo: "pdfs/manual.pdf", expected: NoteTypePDF},
		{name: "Attachment path", tags: nil, arquivo: "attachments/x.zip", expected: NoteTypeAttachment},
		{name: "Archive path", tags: nil, arquivo: "archives/x.md", expected: NoteTypeArchive},
		{name: "EPUB path", tags: nil, arquivo: "epubs/livro.epub", expected: NoteTypeEPUB},
		{name: "EPUB extension", tags: nil, arquivo: "notes/livro.epub", expected: NoteTypeEPUB},
		{name: "Image img_ prefix", tags: nil, arquivo: "notes/img_172300000_foto.png", expected: NoteTypeImage},
		{name: "Image jpeg extension", tags: nil, arquivo: "notes/foto.jpeg", expected: NoteTypeImage},
		{name: "Filename mindmap", tags: nil, arquivo: "notes/mindmap-geral.md", expected: NoteTypeMindmap},
		{name: "Filename markmap", tags: nil, arquivo: "notes/markmap-geral.md", expected: NoteTypeMindmap},
		{name: "Filename drawing", tags: nil, arquivo: "notes/meu-desenho.md", expected: NoteTypeDrawing},
		{name: "Normal markdown", tags: nil, arquivo: "notes/Calendário Acadêmico 2026-2.md", expected: NoteTypeMarkdown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectNoteType(tt.tags, tt.arquivo); got != tt.expected {
				t.Errorf("DetectNoteType() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDetectNoteTypeFromContent(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		content  string
		arquivo  string
		expected NoteType
	}{
		{
			name:     "Frontmatter type: markmap",
			tags:     nil,
			content:  "---\ntype: markmap\n---\n# Meu Mapa\n- Tópico",
			arquivo:  "notes/mapa.md",
			expected: NoteTypeMindmap,
		},
		{
			name:     "Frontmatter type: mindmap",
			tags:     nil,
			content:  "---\ntype: mindmap\n---\n# Mapa\n- Item",
			arquivo:  "notes/mapa.md",
			expected: NoteTypeMindmap,
		},
		{
			name:     "Markdown code block ```markmap",
			tags:     nil,
			content:  "```markmap\n# Mapa\n- A\n- B\n```",
			arquivo:  "notes/Diagrama.md",
			expected: NoteTypeMindmap,
		},
		{
			name:     "Frontmatter type: drawing",
			tags:     nil,
			content:  "---\ntype: drawing\n---\n{}",
			arquivo:  "notes/desenho.md",
			expected: NoteTypeDrawing,
		},
		{
			name:     "Tag priority over content",
			tags:     []string{"markmap"},
			content:  "some random text",
			arquivo:  "notes/mapa.md",
			expected: NoteTypeMindmap,
		},
		{
			name:     "Normal markdown note",
			tags:     nil,
			content:  "# Minha nota normal\nEste é um texto comum.",
			arquivo:  "notes/Calendário Acadêmico 2026-2.md",
			expected: NoteTypeMarkdown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectNoteTypeFromContent(tt.tags, tt.content, tt.arquivo); got != tt.expected {
				t.Errorf("DetectNoteTypeFromContent() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNoteOpenTarget(t *testing.T) {
	tests := []struct {
		name      string
		noteType  NoteType
		arquivo   string
		wantURL   string
		wantBlank bool
	}{
		{name: "Markdown", noteType: NoteTypeMarkdown, arquivo: "notes/a.md", wantURL: "/editor?file=notes/a.md", wantBlank: false},
		{name: "Drawing", noteType: NoteTypeDrawing, arquivo: "notes/d.md", wantURL: "/drawing?file=notes/d.md", wantBlank: false},
		{name: "Mindmap", noteType: NoteTypeMindmap, arquivo: "notes/m.md", wantURL: "/mindmap?file=notes/m.md", wantBlank: false},
		{name: "Semanal", noteType: NoteTypeSemanal, arquivo: "notes/2026-S38.md", wantURL: "/editor?file=notes/2026-S38.md", wantBlank: false},
		{name: "PDF", noteType: NoteTypePDF, arquivo: "pdfs/x.pdf", wantURL: "/file?name=pdfs/x.pdf", wantBlank: true},
		{name: "Attachment", noteType: NoteTypeAttachment, arquivo: "attachments/x.zip", wantURL: "/file/download?name=attachments/x.zip", wantBlank: true},
		{name: "Archive", noteType: NoteTypeArchive, arquivo: "archives/x.md", wantURL: "/file/download?name=archives/x.md", wantBlank: true},
		{name: "EPUB", noteType: NoteTypeEPUB, arquivo: "epubs/livro.epub", wantURL: "/epub/reader?file=epubs/livro.epub", wantBlank: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, blank := NoteOpenTarget(tt.noteType, tt.arquivo)
			if url != tt.wantURL || blank != tt.wantBlank {
				t.Errorf("NoteOpenTarget() = (%q, %v), want (%q, %v)", url, blank, tt.wantURL, tt.wantBlank)
			}
		})
	}
}

// ── Nota Semanal ──

// TestWeeklyNoteFilename valida o nome determinístico da nota semanal
// (ano-semana ISO-8601), incluindo as viradas de ano em que o ano-semana
// difere do ano civil — que é justamente onde uma implementação ingênua erra.
func TestWeeklyNoteFilename(t *testing.T) {
	tests := []struct {
		name string
		date time.Time
		want string
	}{
		{"Meio do ano", time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC), "notes/2026-S38.md"},
		{"Primeiro dia do ano (quinta da semana 1)", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), "notes/2026-S01.md"},
		{"Segunda 29/12/2025 já é semana 1 de 2026", time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC), "notes/2026-S01.md"},
		{"Sexta 01/01/2021 pertence à semana 53 de 2020", time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), "notes/2020-S53.md"},
		{"Semana sempre com dois dígitos", time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), "notes/2026-S02.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WeeklyNoteFilename(tt.date)
			if got != tt.want {
				t.Errorf("WeeklyNoteFilename(%s) = %q, want %q", tt.date.Format("2006-01-02"), got, tt.want)
			}
			// O nome gerado precisa ser sempre reconhecido como nota semanal.
			if !IsWeeklyNoteFilename(got) {
				t.Errorf("IsWeeklyNoteFilename(%q) = false, esperado true", got)
			}
			// E o tipo detectado a partir do nome precisa ser semanal.
			if got := DetectNoteType(nil, WeeklyNoteFilename(tt.date)); got != NoteTypeSemanal {
				t.Errorf("DetectNoteType(nil, %q) = %v, want %v", WeeklyNoteFilename(tt.date), got, NoteTypeSemanal)
			}
		})
	}
}

// TestWeeklyNoteEmptyContent garante que a nota semanal nasce EM BRANCO: sem
// frontmatter, sem título e sem tag — o tipo vem do NOME do arquivo, não de tags.
func TestWeeklyNoteEmptyContent(t *testing.T) {
	if strings.TrimSpace(WeeklyNoteEmptyContent) != "" {
		t.Errorf("conteúdo da nota semanal deveria ser em branco, got %q", WeeklyNoteEmptyContent)
	}
	if strings.Contains(WeeklyNoteEmptyContent, "tags:") || strings.Contains(WeeklyNoteEmptyContent, "---") {
		t.Errorf("conteúdo não deveria ter frontmatter/tag: %q", WeeklyNoteEmptyContent)
	}
	if strings.Contains(WeeklyNoteEmptyContent, "#") {
		t.Errorf("conteúdo não deveria ter título markdown: %q", WeeklyNoteEmptyContent)
	}
}

func TestIsWeeklyNoteFilename(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"com prefixo notes/ e .md", "notes/2026-S38.md", true},
		{"sem prefixo", "2026-S38.md", true},
		{"sem extensão", "2026-S38", true},
		{"semana 1", "notes/2026-S01.md", true},
		{"semana 53", "notes/2026-S53.md", true},
		{"semana 0 inválida", "notes/2026-S00.md", false},
		{"semana 54 inválida", "notes/2026-S54.md", false},
		{"designador W (ISO) não é o usado", "notes/2026-W38.md", false},
		{"semana com 1 dígito", "notes/2026-S3.md", false},
		{"ano com 3 dígitos", "notes/202-S38.md", false},
		{"nota comum", "notes/nota-qualquer.md", false},
		{"nome gerado (diceware)", "notes/veloz-tigre-42.md", false},
		{"vazio", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsWeeklyNoteFilename(tt.in); got != tt.want {
				t.Errorf("IsWeeklyNoteFilename(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestNoteTypeCanonicalTag_Semanal garante que a tag canônica do tipo semanal
// está mapeada (usada por EnsureTypeTags para tornar o tipo persistido).
func TestNoteTypeCanonicalTag_Semanal(t *testing.T) {
	if got := NoteTypeCanonicalTag(NoteTypeSemanal); got != "semanal" {
		t.Errorf("NoteTypeCanonicalTag(NoteTypeSemanal) = %q, want %q", got, "semanal")
	}
	// O editor da nota semanal é o editor markdown padrão.
	if got := NoteTypeSemanal.EditorRoute(); got != "/editor" {
		t.Errorf("NoteTypeSemanal.EditorRoute() = %q, want %q", got, "/editor")
	}
}

// TestDetectNoteType_WeeklyIsAnchored garante que a heurística de nome semanal
// é ancorada: um nome que apenas CONTÉM o padrão continua sendo nota comum.
func TestDetectNoteType_WeeklyIsAnchored(t *testing.T) {
	tests := []struct {
		arquivo  string
		expected NoteType
	}{
		{"notes/notas-da-semana-2026-S38.md", NoteTypeMarkdown},
		{"notes/2026-S38-rascunho.md", NoteTypeMarkdown},
		{"notes/2026-S38.md", NoteTypeSemanal},
	}
	for _, tt := range tests {
		if got := DetectNoteType(nil, tt.arquivo); got != tt.expected {
			t.Errorf("DetectNoteType(nil, %q) = %v, want %v", tt.arquivo, got, tt.expected)
		}
	}
}
