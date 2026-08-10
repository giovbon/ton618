package domain

import (
	"testing"
)

func TestDetectNoteType(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		arquivo  string
		expected NoteType
	}{
		{name: "Explicit tag mermaid", tags: []string{"mermaid"}, arquivo: "notes/Calendário Acadêmico 2026-2.md", expected: NoteTypeMermaid},
		{name: "Explicit tag drawing", tags: []string{"drawing"}, arquivo: "notes/x.md", expected: NoteTypeDrawing},
		{name: "Explicit tag desenho", tags: []string{"desenho"}, arquivo: "notes/x.md", expected: NoteTypeDrawing},
		{name: "Explicit tag spreadsheet", tags: []string{"spreadsheet"}, arquivo: "notes/x.md", expected: NoteTypeSpreadsheet},
		{name: "Explicit tag typst", tags: []string{"typst"}, arquivo: "notes/x.md", expected: NoteTypeTypst},
		{name: "Explicit tag markmap", tags: []string{"markmap"}, arquivo: "notes/x.md", expected: NoteTypeMindmap},
		{name: "Explicit tag mindmap", tags: []string{"mindmap"}, arquivo: "notes/x.md", expected: NoteTypeMindmap},
		{name: "Explicit tag mapa", tags: []string{"mapa"}, arquivo: "notes/x.md", expected: NoteTypeMap},
		{name: "Explicit tag map", tags: []string{"map"}, arquivo: "notes/x.md", expected: NoteTypeMap},
		{name: "Explicit tag youtube", tags: []string{"youtube"}, arquivo: "notes/x.md", expected: NoteTypeYoutube},
		{name: "Explicit tag artigo", tags: []string{"artigo"}, arquivo: "notes/x.md", expected: NoteTypeArticle},
		{name: "Explicit tag captura", tags: []string{"captura"}, arquivo: "notes/x.md", expected: NoteTypeCapture},
		{name: "PDF path", tags: nil, arquivo: "pdfs/manual.pdf", expected: NoteTypePDF},
		{name: "Attachment path", tags: nil, arquivo: "attachments/x.zip", expected: NoteTypeAttachment},
		{name: "Archive path", tags: nil, arquivo: "archives/x.md", expected: NoteTypeArchive},
		{name: "EPUB path", tags: nil, arquivo: "epubs/livro.epub", expected: NoteTypeEPUB},
		{name: "EPUB extension", tags: nil, arquivo: "notes/livro.epub", expected: NoteTypeEPUB},
		{name: "Image img_ prefix", tags: nil, arquivo: "notes/img_172300000_foto.png", expected: NoteTypeImage},
		{name: "Image jpeg extension", tags: nil, arquivo: "notes/foto.jpeg", expected: NoteTypeImage},
		{name: "Filename mindmap", tags: nil, arquivo: "notes/mindmap-geral.md", expected: NoteTypeMindmap},
		{name: "Filename drawing", tags: nil, arquivo: "notes/meu-desenho.md", expected: NoteTypeDrawing},
		{name: "Filename mermaid", tags: nil, arquivo: "notes/diagrama-mermaid.md", expected: NoteTypeMermaid},
		{name: "Filename mapa- prefix", tags: nil, arquivo: "notes/mapa-roteiro.md", expected: NoteTypeMap},
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
			name:     "Frontmatter type: mermaid",
			tags:     nil,
			content:  "---\ntype: mermaid\n---\ngantt title Calendário Acadêmico",
			arquivo:  "notes/Calendário Acadêmico 2026-2.md",
			expected: NoteTypeMermaid,
		},
		{
			name:     "Markdown code block ```mermaid",
			tags:     nil,
			content:  "```mermaid\ngraph TD\nA --> B\n```",
			arquivo:  "notes/Diagrama.md",
			expected: NoteTypeMermaid,
		},
		{
			name:     "Direct gantt content without frontmatter or tag",
			tags:     nil,
			content:  "gantt title Calendário Acadêmico 2026-2 dateFormat YYYY-MM-DD\nsection AGO\nInício veteranos :a2, 2026-08-03, 1d",
			arquivo:  "notes/Calendário Acadêmico 2026-2.md",
			expected: NoteTypeMermaid,
		},
		{
			name:     "Direct graph content without frontmatter or tag",
			tags:     nil,
			content:  "graph TD\n    A[Início] --> B(Fim)",
			arquivo:  "notes/Fluxo.md",
			expected: NoteTypeMermaid,
		},
		{
			name:     "Frontmatter type: drawing",
			tags:     nil,
			content:  "---\ntype: drawing\n---\n{}",
			arquivo:  "notes/desenho.md",
			expected: NoteTypeDrawing,
		},
		{
			name:     "Frontmatter type: typst",
			tags:     nil,
			content:  "---\ntype: typst\n---\n= Titulo",
			arquivo:  "notes/doc.typ",
			expected: NoteTypeTypst,
		},
		{
			name:     "Tag priority over content",
			tags:     []string{"mermaid"},
			content:  "some random text",
			arquivo:  "notes/Calendário Acadêmico 2026-2.md",
			expected: NoteTypeMermaid,
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
		{name: "Map", noteType: NoteTypeMap, arquivo: "notes/m.md", wantURL: "/map?file=notes/m.md", wantBlank: false},
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
