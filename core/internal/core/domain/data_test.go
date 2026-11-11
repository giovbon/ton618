package domain

import (
	"strings"
	"testing"
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
		{"captura com caminho", "notes/captura-terremoto-7-4.md", "terremoto-7-4"},
		{"captura sem caminho", "captura-artigo-web.md", "artigo-web"},
		{"nota normal mantém", "notes/nota-qualquer.md", "nota-qualquer"},
		{"pdf mantém", "pdfs/doc.pdf", "doc.pdf"},
		{"anexo mantém", "attachments/arquivo.zip", "arquivo.zip"},
		{"captura em subcaminho", "notes/captura-a.md", "a"},
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
		{name: "Explicit tag calculo", tags: []string{"calculo"}, arquivo: "notes/x.md", expected: NoteTypeCalculo},
		{name: "Explicit tag cálculo acentuado", tags: []string{"cálculo"}, arquivo: "notes/x.md", expected: NoteTypeCalculo},
		{name: "Explicit tag calc", tags: []string{"calc"}, arquivo: "notes/x.md", expected: NoteTypeCalculo},
		{name: "PDF path", tags: nil, arquivo: "pdfs/manual.pdf", expected: NoteTypePDF},
		{name: "Attachment path", tags: nil, arquivo: "attachments/x.zip", expected: NoteTypeAttachment},
		{name: "Archive path", tags: nil, arquivo: "archives/x.md", expected: NoteTypeArchive},
		{name: "EPUB path", tags: nil, arquivo: "epubs/livro.epub", expected: NoteTypeEPUB},
		{name: "EPUB extension", tags: nil, arquivo: "notes/livro.epub", expected: NoteTypeEPUB},
		{name: "Image img_ prefix", tags: nil, arquivo: "notes/img_172300000_foto.png", expected: NoteTypeImage},
		{name: "Image images/ prefix", tags: nil, arquivo: "images/img_172300000_foto.png", expected: NoteTypeImage},
		{name: "Image images/ com nome arbitrário", tags: nil, arquivo: "images/qualquer-coisa.png", expected: NoteTypeImage},
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
		{name: "Cálculo", noteType: NoteTypeCalculo, arquivo: "notes/calculo-x.md", wantURL: "/editor?file=notes/calculo-x.md", wantBlank: false},
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

// ── Nota de Cálculo ──

// TestNoteTypeCanonicalTag_Calculo garante que a tag canônica do tipo cálculo
// está mapeada (usada por EnsureTypeTags para tornar o tipo persistido).
func TestNoteTypeCanonicalTag_Calculo(t *testing.T) {
	if got := NoteTypeCanonicalTag(NoteTypeCalculo); got != CalculoTag {
		t.Errorf("NoteTypeCanonicalTag(NoteTypeCalculo) = %q, want %q", got, CalculoTag)
	}
	// A nota de cálculo abre no editor markdown padrão.
	if got := NoteTypeCalculo.EditorRoute(); got != "/editor" {
		t.Errorf("NoteTypeCalculo.EditorRoute() = %q, want %q", got, "/editor")
	}
}

// TestCalculoNoteEmptyContent garante que a nota de cálculo nasce com a tag
// canônica (para a detecção do tipo) e com um bloco ```calc vazio.
func TestCalculoNoteEmptyContent(t *testing.T) {
	if !strings.Contains(CalculoNoteEmptyContent, "tags: ["+CalculoTag+"]") {
		t.Errorf("conteúdo deveria ter a tag %q: %q", CalculoTag, CalculoNoteEmptyContent)
	}
	if !strings.Contains(CalculoNoteEmptyContent, "```calc") {
		t.Errorf("conteúdo deveria ter um bloco ```calc: %q", CalculoNoteEmptyContent)
	}
	if got := DetectNoteTypeFromContent([]string{CalculoTag}, CalculoNoteEmptyContent, "notes/calculo-x.md"); got != NoteTypeCalculo {
		t.Errorf("DetectNoteTypeFromContent() = %v, want %v", got, NoteTypeCalculo)
	}
	// Notas antigas podem ter só o frontmatter, sem a tag persistida.
	legacy := "---\ntipo: calculo\n---\n\n```calc\n1 + 1\n```\n"
	if got := DetectNoteTypeFromContent(nil, legacy, "notes/calculo-x.md"); got != NoteTypeCalculo {
		t.Errorf("DetectNoteTypeFromContent(legacy) = %v, want %v", got, NoteTypeCalculo)
	}
}

// TestPrefixosDeDiretorio_SaoDerivadosDaFonteUnica trava a consolidação do
// layout do docs/ (DECISIONS §12): as listas derivadas precisam cobrir
// exatamente AllowedFilePrefixes, sem literais duplicados espalhados.
func TestPrefixosDeDiretorio_SaoDerivadosDaFonteUnica(t *testing.T) {
	allowed := make(map[string]bool, len(AllowedFilePrefixes))
	for _, p := range AllowedFilePrefixes {
		allowed[p] = true
	}

	if len(NonNoteFilePrefixes)+1+len(InternalFilePrefixes) != len(AllowedFilePrefixes) {
		t.Errorf("AllowedFilePrefixes deveria ser NonNoteFilePrefixes + notes/ + internos (%d), got %d",
			len(NonNoteFilePrefixes)+1+len(InternalFilePrefixes), len(AllowedFilePrefixes))
	}
	for i, p := range NonNoteFilePrefixes {
		if !allowed[p] {
			t.Errorf("NonNoteFilePrefixes[%d]=%q não está em AllowedFilePrefixes", i, p)
		}
		if p == NotePrefix {
			t.Error("NonNoteFilePrefixes não deve conter notes/")
		}
		if FileDirs[i] != strings.TrimSuffix(p, "/") {
			t.Errorf("FileDirs[%d]=%q não corresponde a %q", i, FileDirs[i], p)
		}
	}
	if len(FileDirs) != len(NonNoteFilePrefixes) {
		t.Errorf("FileDirs deveria espelhar NonNoteFilePrefixes (%d), got %d", len(NonNoteFilePrefixes), len(FileDirs))
	}
	if len(SearchDirs) != len(FileDirs)+1 || SearchDirs[len(SearchDirs)-1] != NotesDir {
		t.Errorf("SearchDirs deveria ser FileDirs + notes/ no fim, got %v", SearchDirs)
	}

	// Matchers usados pelos handlers.
	if !HasAllowedFilePrefix("notes/x.md") || !HasAllowedFilePrefix("images/a.png") {
		t.Error("prefixos permitidos deveriam ser aceitos")
	}
	// datasets/ é permitido (servido por /file/download), mas é diretório INTERNO:
	// fica fora de NonNoteFilePrefixes e de FileDirs (não é varrido/indexado).
	if !HasAllowedFilePrefix("datasets/relatorio.pdf") {
		t.Error("datasets/ deveria ser um prefixo permitido")
	}
	for _, p := range NonNoteFilePrefixes {
		if p == DatasetPrefix {
			t.Error("NonNoteFilePrefixes não deve conter o diretório interno datasets/")
		}
	}
	for _, d := range FileDirs {
		if d == DatasetDir {
			t.Error("FileDirs não deve conter datasets/ (não é varrido nem indexado)")
		}
	}
	if HasAllowedFilePrefix("etc/passwd") || HasAllowedFilePrefix("x.md") {
		t.Error("caminho fora dos prefixos permitidos não deveria ser aceito")
	}
	if !HasInlineDocPrefix("attachments/a.zip") || !HasInlineDocPrefix("images/a.png") {
		t.Error("InlineDocPrefixes deve incluir attachments/ e images/")
	}
	if HasInlineDocPrefix("epubs/livro.epub") || HasInlineDocPrefix("archives/x.zip") {
		t.Error("InlineDocPrefixes não deve incluir epubs/ nem archives/")
	}
	if HasNonNoteFilePrefix("notes/x.md") || !HasNonNoteFilePrefix("pdfs/x.pdf") {
		t.Error("HasNonNoteFilePrefix deve excluir notes/")
	}
}
