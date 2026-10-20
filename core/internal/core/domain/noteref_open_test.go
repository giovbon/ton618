package domain

import "testing"

// TestNoteRef_OpenURL trava a regra de destino das REFERÊNCIAS de navegação
// (ex: as filhas de uma nota, DECISIONS §6.30).
//
// A lista de filhas mistura notas e ARQUIVOS: um `pai` de PDF/EPUB/ZIP vive na
// tabela file_metadata (§6.24). Se o link for montado à mão ("/editor?file=..."),
// o PDF abre no editor e aparece uma nota vazia. O NoteRef precisa derivar o
// tipo do caminho e delegar para o NoteOpenTarget — a regra única do app.
func TestNoteRef_OpenURL(t *testing.T) {
	tests := []struct {
		arquivo   string
		wantURL   string
		wantBlank bool
	}{
		{arquivo: "notes/a.md", wantURL: "/editor?file=notes/a.md", wantBlank: false},
		{arquivo: "notes/2026-S38.md", wantURL: "/editor?file=notes/2026-S38.md", wantBlank: false},
		{arquivo: "notes/meu-desenho.md", wantURL: "/drawing?file=notes/meu-desenho.md", wantBlank: false},
		{arquivo: "pdfs/manual.pdf", wantURL: "/file?name=pdfs/manual.pdf", wantBlank: true},
		{arquivo: "epubs/livro.epub", wantURL: "/epub/reader?file=epubs/livro.epub", wantBlank: false},
		{arquivo: "attachments/x.zip", wantURL: "/file/download?name=attachments/x.zip", wantBlank: true},
	}

	for _, tt := range tests {
		ref := NoteRef{Filename: tt.arquivo}
		if got := ref.OpenURL(); got != tt.wantURL {
			t.Errorf("NoteRef{%q}.OpenURL() = %q, want %q", tt.arquivo, got, tt.wantURL)
		}
		if got := ref.OpenInNewTab(); got != tt.wantBlank {
			t.Errorf("NoteRef{%q}.OpenInNewTab() = %v, want %v", tt.arquivo, got, tt.wantBlank)
		}
	}
}

// TestNoteRef_OpenURLSegueNoteOpenTarget amarra o NoteRef à regra canônica:
// qualquer tipo/caminho tem que dar exatamente o mesmo destino do NoteOpenTarget.
// É a guarda contra alguém "otimizar" o NoteRef com um switch paralelo.
func TestNoteRef_OpenURLSegueNoteOpenTarget(t *testing.T) {
	caminhos := []string{
		"notes/a.md",
		"notes/mindmap-ideias.md",
		"pdfs/manual.pdf",
		"pdfs/sub/outro.pdf",
		"epubs/livro.epub",
		"attachments/x.zip",
		"archives/velha.md",
	}

	for _, caminho := range caminhos {
		wantURL, wantBlank := NoteOpenTarget(DetectNoteType(nil, caminho), caminho)
		ref := NoteRef{Filename: caminho}
		if got := ref.OpenURL(); got != wantURL {
			t.Errorf("%q: OpenURL() = %q, NoteOpenTarget = %q", caminho, got, wantURL)
		}
		if got := ref.OpenInNewTab(); got != wantBlank {
			t.Errorf("%q: OpenInNewTab() = %v, NoteOpenTarget = %v", caminho, got, wantBlank)
		}
	}
}
