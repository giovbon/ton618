package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewReaderStore_ArquivoInexistente(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nao-existe.db")

	store, err := NewReaderStore(path)
	if err == nil {
		store.Close()
		t.Fatal("esperava erro para banco inexistente")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("NewReaderStore não deveria criar o arquivo: %v", statErr)
	}
}

func TestNewReaderStore_LeSemMigrar(t *testing.T) {
	path := filepath.Join(t.TempDir(), "acervo.db")

	writer, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := writer.SaveNote("notes/a.md", "conteúdo original", "2026-10-25T00:00:00Z"); err != nil {
		writer.Close()
		t.Fatalf("SaveNote: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	reader, err := NewReaderStore(path)
	if err != nil {
		t.Fatalf("NewReaderStore: %v", err)
	}
	defer reader.Close()

	got, err := reader.GetNote("notes/a.md")
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if got != "conteúdo original" {
		t.Errorf("GetNote = %q", got)
	}

	// mode=ro: escrita tem que ser rejeitada pelo SQLite.
	if err := reader.SaveNote("notes/b.md", "não pode gravar", "2026-10-25T00:00:00Z"); err == nil {
		t.Error("SaveNote deveria falhar em um Store somente leitura")
	}
}
