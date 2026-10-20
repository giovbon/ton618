package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// TestValidateDatabaseFile cobre a barreira usada antes de aplicar um backup
// enviado pelo usuário: só passa um SQLite íntegro com a tabela `notes`.
func TestValidateDatabaseFile(t *testing.T) {
	dir := t.TempDir()

	t.Run("banco do TON-618", func(t *testing.T) {
		path := filepath.Join(dir, "ok.db")
		store, err := NewStore(path)
		if err != nil {
			t.Fatalf("NewStore: %v", err)
		}
		store.Close()

		if err := ValidateDatabaseFile(path); err != nil {
			t.Errorf("banco válido reprovado: %v", err)
		}
	})

	t.Run("arquivo que não é sqlite", func(t *testing.T) {
		path := filepath.Join(dir, "lixo.db")
		if err := os.WriteFile(path, []byte("isto não é um banco"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := ValidateDatabaseFile(path); err == nil {
			t.Error("esperado erro para arquivo que não é SQLite")
		}
	})

	t.Run("sqlite sem a tabela notes", func(t *testing.T) {
		path := filepath.Join(dir, "outro.db")
		conn, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(`CREATE TABLE qualquer (a TEXT)`); err != nil {
			t.Fatal(err)
		}
		conn.Close()

		if err := ValidateDatabaseFile(path); err == nil {
			t.Error("esperado erro para banco sem a tabela `notes`")
		}
	})

	t.Run("arquivo ausente", func(t *testing.T) {
		if err := ValidateDatabaseFile(filepath.Join(dir, "nao-existe.db")); err == nil {
			t.Error("esperado erro para arquivo ausente")
		}
	})
}

// TestSnapshot_ServeDeBackup garante que o snapshot sai consistente (reabrível
// com as notas) e sem tocar no banco vivo.
func TestSnapshot_ServeDeBackup(t *testing.T) {
	dir := t.TempDir()
	livePath := filepath.Join(dir, "live.db")

	store, err := NewStore(livePath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	if err := store.SaveNote("notes/a.md", "# A\nconteudo", "2026-01-02T03:04:05Z"); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}

	snapPath := filepath.Join(dir, "snapshot.db")
	if err := store.Snapshot(snapPath); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	// O snapshot é um banco válido e reabrível, com a nota preservada.
	if err := ValidateDatabaseFile(snapPath); err != nil {
		t.Fatalf("snapshot reprovado na validação: %v", err)
	}
	snap, err := NewStore(snapPath)
	if err != nil {
		t.Fatalf("abrir snapshot: %v", err)
	}
	defer snap.Close()

	notes, err := snap.GetAllNotes()
	if err != nil {
		t.Fatalf("GetAllNotes do snapshot: %v", err)
	}
	if _, ok := notes["notes/a.md"]; !ok {
		t.Errorf("snapshot sem a nota: %v", notes)
	}
}
