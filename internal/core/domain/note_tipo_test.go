package domain

import "testing"

func TestFrontmatterTipo(t *testing.T) {
	tests := []struct {
		in   NoteType
		want string
	}{
		{NoteTypeMarkdown, "nota"},
		{NoteTypeDrawing, "desenho"},
		{NoteTypeTabulator, "tabulator"},
		{NoteTypePDF, "pdf"},
		{NoteTypeEPUB, "epub"},
		{NoteTypeAttachment, "anexo"},
		{NoteTypeArchive, "arquivo"},
		{NoteTypeImage, "imagem"},
		{NoteTypeYoutube, "youtube"},
		{NoteTypeArticle, "artigo"},
		{NoteTypeCapture, "captura"},
		{NoteTypeCalculo, "calculo"},
	}
	for _, tt := range tests {
		if got := FrontmatterTipo(tt.in); got != tt.want {
			t.Errorf("FrontmatterTipo(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeNoteTipo(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		// canônicos passam direto
		{"nota", "nota"},
		{"anexo", "anexo"},
		{"tabulator", "tabulator"},
		// normalização básica
		{"  Nota  ", "nota"},
		{"Nota Com Espaço", "nota_com_espaço"},
		// aliases históricos
		{"drawing", "desenho"},
		{"markdown", "nota"},
		{"md", "nota"},
		{"capture", "captura"},
		{"article", "artigo"},
		{"calc", "calculo"},
		{"attachment", "anexo"},
		{"archive", "arquivo"},
		{"image", "imagem"},
		{"img", "imagem"},
		{"pdfs", "pdf"},
		{"epubs", "epub"},
		// valor desconhecido: só normaliza (não casa nada, mas não quebra)
		{"qualquer_coisa", "qualquer_coisa"},
		// markmap/mindmap já não são aliases válidos (tipo removido)
		{"markmap", "markmap"},
		{"mindmap", "mindmap"},
	}
	for _, tt := range tests {
		if got := NormalizeNoteTipo(tt.in); got != tt.want {
			t.Errorf("NormalizeNoteTipo(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// Roundtrip: todo valor canônico passa por Normalize sem mudar.
func TestNormalizeNoteTipoIdempotente(t *testing.T) {
	tipos := []NoteType{
		NoteTypeMarkdown, NoteTypeDrawing, NoteTypeTabulator,
		NoteTypePDF, NoteTypeEPUB, NoteTypeAttachment, NoteTypeArchive,
		NoteTypeImage, NoteTypeYoutube, NoteTypeArticle, NoteTypeCapture,
		NoteTypeCalculo,
	}
	for _, nt := range tipos {
		v := FrontmatterTipo(nt)
		if got := NormalizeNoteTipo(v); got != v {
			t.Errorf("NormalizeNoteTipo(FrontmatterTipo(%q)) = %q, want %q (idempotente)", nt, got, v)
		}
	}
}
