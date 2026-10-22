package domain

import "testing"

func TestFrontmatterTipo(t *testing.T) {
	tests := []struct {
		in   NoteType
		want string
	}{
		{NoteTypeMarkdown, "nota"},
		{NoteTypeDrawing, "desenho"},
		{NoteTypeMindmap, "mapa_mental"}, // única divergência do valor interno
		{NoteTypeTabulator, "tabulator"},
		{NoteTypePDF, "pdf"},
		{NoteTypeEPUB, "epub"},
		{NoteTypeAttachment, "anexo"},
		{NoteTypeArchive, "arquivo"},
		{NoteTypeImage, "imagem"},
		{NoteTypeYoutube, "youtube"},
		{NoteTypeArticle, "artigo"},
		{NoteTypeCapture, "captura"},
		{NoteTypeSemanal, "semanal"},
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
		{"mapa_mental", "mapa_mental"},
		{"anexo", "anexo"},
		{"tabulator", "tabulator"},
		// normalização básica
		{"  Nota  ", "nota"},
		{"Mapa Mental", "mapa_mental"},
		{"mapa-mental", "mapa_mental"},
		// aliases históricos
		{"markmap", "mapa_mental"},
		{"mindmap", "mapa_mental"},
		{"drawing", "desenho"},
		{"markdown", "nota"},
		{"md", "nota"},
		{"capture", "captura"},
		{"article", "artigo"},
		{"weekly", "semanal"},
		{"attachment", "anexo"},
		{"archive", "arquivo"},
		{"image", "imagem"},
		{"img", "imagem"},
		{"pdfs", "pdf"},
		{"epubs", "epub"},
		// valor desconhecido: só normaliza (não casa nada, mas não quebra)
		{"qualquer_coisa", "qualquer_coisa"},
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
		NoteTypeMarkdown, NoteTypeDrawing, NoteTypeMindmap, NoteTypeTabulator,
		NoteTypePDF, NoteTypeEPUB, NoteTypeAttachment, NoteTypeArchive,
		NoteTypeImage, NoteTypeYoutube, NoteTypeArticle, NoteTypeCapture,
		NoteTypeSemanal,
	}
	for _, nt := range tipos {
		v := FrontmatterTipo(nt)
		if got := NormalizeNoteTipo(v); got != v {
			t.Errorf("NormalizeNoteTipo(FrontmatterTipo(%q)) = %q, want %q (idempotente)", nt, got, v)
		}
	}
}
