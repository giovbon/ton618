package notes

import (
	"os"
	"path/filepath"
	"testing"
)

// stubRefCounter simula o *db.Store: devolve quantas OUTRAS notas referenciam
// cada caminho.
type stubRefCounter map[string]int

func (s stubRefCounter) CountOtherNotesReferencing(path, except string) (int, error) {
	return s[path], nil
}

func body(paths ...string) string {
	rows := ""
	for i, p := range paths {
		if i > 0 {
			rows += ","
		}
		rows += `{"c1":"` + p + `"}`
	}
	return `{"columns":[{"field":"c1","title":"Anexo","type":"file"}],"rows":[` + rows + `]}`
}

func writeDatasets(t *testing.T, docsDir string, names ...string) {
	t.Helper()
	dir := filepath.Join(docsDir, "datasets")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("conteudo"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func exists(docsDir, rel string) bool {
	_, err := os.Stat(filepath.Join(docsDir, filepath.FromSlash(rel)))
	return err == nil
}

// Ao remover o arquivo de uma célula, ele é apagado do disco.
func TestReconcileTabulatorFiles_ApagaArquivoRemovidoDaCelula(t *testing.T) {
	docs := t.TempDir()
	writeDatasets(t, docs, "a.pdf", "b.pdf")

	removed := ReconcileTabulatorFiles(nil, docs, "notes/n.md",
		body("datasets/a.pdf", "datasets/b.pdf"),
		body("datasets/a.pdf"))

	if removed != 1 {
		t.Fatalf("esperado 1 arquivo removido, got %d", removed)
	}
	if !exists(docs, "datasets/a.pdf") {
		t.Error("a.pdf (ainda referenciado) não deveria ser removido")
	}
	if exists(docs, "datasets/b.pdf") {
		t.Error("b.pdf (removido da célula) deveria ter sido apagado")
	}
}

// Trocar o arquivo da célula apaga o antigo.
func TestReconcileTabulatorFiles_ApagaArquivoTrocado(t *testing.T) {
	docs := t.TempDir()
	writeDatasets(t, docs, "antigo.pdf", "novo.pdf")

	removed := ReconcileTabulatorFiles(nil, docs, "notes/n.md",
		body("datasets/antigo.pdf"),
		body("datasets/novo.pdf"))

	if removed != 1 {
		t.Fatalf("esperado 1 arquivo removido, got %d", removed)
	}
	if exists(docs, "datasets/antigo.pdf") {
		t.Error("o arquivo antigo deveria ter sido apagado")
	}
	if !exists(docs, "datasets/novo.pdf") {
		t.Error("o arquivo novo não deveria ser tocado")
	}
}

// Nota removida por completo (newContent vazio) → apaga os arquivos da nota.
func TestReconcileTabulatorFiles_NotaRemovida(t *testing.T) {
	docs := t.TempDir()
	writeDatasets(t, docs, "a.pdf", "b.pdf")

	removed := ReconcileTabulatorFiles(nil, docs, "notes/n.md",
		body("datasets/a.pdf", "datasets/b.pdf"), "")

	if removed != 2 {
		t.Fatalf("esperado 2 arquivos removidos, got %d", removed)
	}
}

// Arquivo ainda usado por uma nota duplicada é preservado.
func TestReconcileTabulatorFiles_RespeitaOutrasNotas(t *testing.T) {
	docs := t.TempDir()
	writeDatasets(t, docs, "compartilhado.pdf", "só-minha.pdf")

	refs := stubRefCounter{"datasets/compartilhado.pdf": 1}
	removed := ReconcileTabulatorFiles(refs, docs, "notes/n.md",
		body("datasets/compartilhado.pdf", "datasets/só-minha.pdf"), "")

	if removed != 1 {
		t.Fatalf("esperado 1 arquivo removido, got %d", removed)
	}
	if !exists(docs, "datasets/compartilhado.pdf") {
		t.Error("arquivo ainda referenciado por outra nota não deveria ser removido")
	}
	if exists(docs, "datasets/só-minha.pdf") {
		t.Error("arquivo sem referências deveria ter sido removido")
	}
}

// Sem remoções o helper é no-op e não conta arquivo inexistente como removido.
func TestReconcileTabulatorFiles_SemAlteracao(t *testing.T) {
	docs := t.TempDir()
	writeDatasets(t, docs, "a.pdf")

	if removed := ReconcileTabulatorFiles(nil, docs, "notes/n.md", body("datasets/a.pdf"), body("datasets/a.pdf")); removed != 0 {
		t.Errorf("esperado 0 remoções, got %d", removed)
	}
	// Caminho que não existe no disco: nada a remover, sem erro.
	if removed := ReconcileTabulatorFiles(nil, docs, "notes/n.md", body("datasets/sumiu.pdf"), ""); removed != 0 {
		t.Errorf("esperado 0 remoções para arquivo ausente, got %d", removed)
	}
	// Nota sem nenhuma referência em datasets/.
	if removed := ReconcileTabulatorFiles(nil, docs, "notes/n.md", `{"rows":[{"c1":"Ana"}]}`, ""); removed != 0 {
		t.Errorf("esperado 0 remoções, got %d", removed)
	}
}

// Nunca remove nada fora de datasets/ — mesmo que apareça no conteúdo.
func TestReconcileTabulatorFiles_NaoSaiDeDatasets(t *testing.T) {
	docs := t.TempDir()
	outside := filepath.Join(docs, "notes", "x.md")
	if err := os.MkdirAll(filepath.Dir(outside), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("nota"), 0o644); err != nil {
		t.Fatal(err)
	}

	malicious := `{"rows":[{"c1":"datasets/../notes/x.md"},{"c1":"datasets/sub/x.txt"}]}`
	if removed := ReconcileTabulatorFiles(nil, docs, "notes/n.md", malicious, ""); removed != 0 {
		t.Errorf("esperado 0 remoções, got %d", removed)
	}
	if !exists(docs, "notes/x.md") {
		t.Error("arquivo fora de datasets/ nunca pode ser removido")
	}
}
