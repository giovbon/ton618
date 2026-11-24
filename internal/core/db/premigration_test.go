package db

import (
	"os"
	"path/filepath"
	"testing"
)

// Ao reabrir um banco existente, NewStore dispara o backup pré-migration: um
// snapshot do arquivo principal é gravado em <dir>/backups/pre-migration.db.
// É a rede de segurança para restaurar caso uma migration falhe (#6).
func TestNewStore_CriaSnapshotPreMigration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore (1º): %v", err)
	}
	if err := s.SetSetting("chave", "valor"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	s.Close()

	// Reabre (nova instância): aqui o backupBeforeMigration roda internamente.
	s2, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore (2º): %v", err)
	}
	defer s2.Close()

	backup := filepath.Join(dir, "backups", "pre-migration.db")
	fi, err := os.Stat(backup)
	if err != nil {
		t.Fatalf("esperava snapshot pré-migration em %s: %v", backup, err)
	}
	if fi.Size() <= 0 {
		t.Errorf("snapshot não deveria estar vazio (size=%d)", fi.Size())
	}
}

// Primeiro boot (banco ainda não existe) não precisa de backup — não há estado
// anterior a preservar. O NewStore não deve falhar nem criar backup nesse caso.
func TestNewStore_SemBackupNoPrimeiroBoot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nada.db")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore em banco novo: %v", err)
	}
	defer s.Close()

	if _, err := os.Stat(filepath.Join(dir, "backups", "pre-migration.db")); err == nil {
		t.Error("não deveria haver snapshot no primeiro boot")
	}
}
