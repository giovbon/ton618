package db

import "testing"

// Contagem de notas que ainda referenciam um arquivo de célula Tabulator:
// é a guarda usada antes de apagar um arquivo de datasets/.
func TestCountOtherNotesReferencing(t *testing.T) {
	s := newTestStore(t)

	if err := s.SaveNote("notes/a.md", `{"rows":[{"c1":"datasets/relatorio.pdf"}]}`, ""); err != nil {
		t.Fatalf("SaveNote a: %v", err)
	}
	if err := s.SaveNote("notes/b.md", "cópia de a", ""); err != nil {
		t.Fatalf("SaveNote b: %v", err)
	}
	// b.md duplicado também aponta para o mesmo arquivo.
	if err := s.SaveNote("notes/b.md", `{"rows":[{"c1":"datasets/relatorio.pdf"}]}`, ""); err != nil {
		t.Fatalf("SaveNote b (dup): %v", err)
	}
	if err := s.SaveNote("notes/c.md", "nota sem datasets/", ""); err != nil {
		t.Fatalf("SaveNote c: %v", err)
	}

	tests := []struct {
		name   string
		path   string
		except string
		want   int
	}{
		{"outras notas referenciam", "datasets/relatorio.pdf", "notes/c.md", 2},
		{"apenas a propria nota referencia", "datasets/relatorio.pdf", "notes/a.md", 1},
		{"ninguem referencia", "datasets/inexistente.pdf", "notes/a.md", 0},
		{"caminho vazio", "", "notes/a.md", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.CountOtherNotesReferencing(tt.path, tt.except)
			if err != nil {
				t.Fatalf("erro: %v", err)
			}
			if got != tt.want {
				t.Errorf("CountOtherNotesReferencing(%q, %q) = %d, want %d", tt.path, tt.except, got, tt.want)
			}
		})
	}
}

// Curingas do LIKE no nome do arquivo não podem casar com qualquer coisa: um
// arquivo "rel_x.pdf" não pode ser considerado referenciado por "relatório.pdf".
func TestCountOtherNotesReferencing_EscapaCuringasLike(t *testing.T) {
	s := newTestStore(t)

	if err := s.SaveNote("notes/a.md", `{"rows":[{"c1":"datasets/relatorio.pdf"}]}`, ""); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}

	for _, path := range []string{"datasets/rel_x.pdf", "datasets/rel%x.pdf", `datasets/rel\x.pdf`} {
		got, err := s.CountOtherNotesReferencing(path, "notes/outra.md")
		if err != nil {
			t.Fatalf("erro: %v", err)
		}
		if got != 0 {
			t.Errorf("CountOtherNotesReferencing(%q) = %d, want 0 (curinga escapado)", path, got)
		}
	}

	// O próprio caminho continua sendo encontrado (o escape não quebra o caso normal).
	got, err := s.CountOtherNotesReferencing("datasets/relatorio.pdf", "notes/outra.md")
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if got != 1 {
		t.Errorf("esperado 1, got %d", got)
	}
}
