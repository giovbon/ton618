package services

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ton618/core/internal/core/db"
)

func newFullBackupFixture(t *testing.T) (store *db.Store, svc *FullBackupService, docsDir, stateDir, dbPath string) {
	t.Helper()

	docsDir = t.TempDir()
	stateDir = t.TempDir()
	dbPath = filepath.Join(stateDir, "ton618.db")

	store, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	svc = NewFullBackupService(store, dbPath, docsDir, stateDir)
	return store, svc, docsDir, stateDir, dbPath
}

// writeTestZip cria um ZIP com as entradas informadas e devolve o caminho.
func writeTestZip(t *testing.T, entries map[string]string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "backup.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("criar zip: %v", err)
	}
	zw := zip.NewWriter(f)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("fechar arquivo: %v", err)
	}
	return path
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("escrever %s: %v", path, err)
	}
}

// addDirectoryEntryToZip copia src para um novo ZIP e acrescenta uma entrada de
// diretório (nome terminado em "/"), como fazem ferramentas de arquivamento.
func addDirectoryEntryToZip(t *testing.T, src, dir string) string {
	t.Helper()

	in, err := zip.OpenReader(src)
	if err != nil {
		t.Fatalf("abrir zip de origem: %v", err)
	}
	defer in.Close()

	dst := filepath.Join(t.TempDir(), "com-diretorio.zip")
	out, err := os.Create(dst)
	if err != nil {
		t.Fatalf("criar zip de destino: %v", err)
	}
	zw := zip.NewWriter(out)
	for _, f := range in.File {
		w, err := zw.CreateHeader(&zip.FileHeader{
			Name:     f.Name,
			Method:   f.Method,
			Modified: f.Modified,
		})
		if err != nil {
			t.Fatalf("copiar cabeçalho %s: %v", f.Name, err)
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("abrir entrada %s: %v", f.Name, err)
		}
		if _, err := io.Copy(w, rc); err != nil {
			rc.Close()
			t.Fatalf("copiar entrada %s: %v", f.Name, err)
		}
		rc.Close()
	}
	if _, err := zw.Create(dir); err != nil {
		t.Fatalf("criar diretório %s: %v", dir, err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("fechar zip: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("fechar arquivo: %v", err)
	}
	return dst
}

// TestFullBackup_RoundTripRestauraEstadoCompleto é o teste central da 3ª opção de
// backup: exportar → destruir → restaurar, e conferir que notas, configurações e
// arquivos (inclusive de archives/) voltam, com o estado anterior em quarentena.
func TestFullBackup_RoundTripRestauraEstadoCompleto(t *testing.T) {
	store, svc, docsDir, _, dbPath := newFullBackupFixture(t)

	// ── estado original ──
	mtime := time.Now().UTC().Format(time.RFC3339)
	if err := store.SaveNote("notes/nota.md", "# Nota original\n\nconteudo", mtime); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	if err := store.SetSetting("semantic_search_threshold", "42"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	mustWriteFile(t, filepath.Join(docsDir, "pdfs", "doc.pdf"), "%PDF-1.4 antigo")
	mustWriteFile(t, filepath.Join(docsDir, "archives", "velha.md"), "# velha")

	// ── exporta ──
	var buf bytes.Buffer
	if err := svc.CreateStream(context.Background(), &buf); err != nil {
		t.Fatalf("CreateStream: %v", err)
	}
	zipPath := filepath.Join(t.TempDir(), "total.zip")
	if err := os.WriteFile(zipPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	// O ZIP precisa trazer manifest, banco e arquivos (inclusive archives/).
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("abrir zip: %v", err)
	}
	names := make(map[string]bool)
	for _, f := range zr.File {
		names[f.Name] = true
	}
	zr.Close()
	for _, want := range []string{backupManifestEntry, backupDBEntry, "docs/pdfs/doc.pdf", "docs/archives/velha.md"} {
		if !names[want] {
			t.Errorf("backup total sem a entrada %q", want)
		}
	}

	// ── destrói o estado atual ──
	if err := store.DeleteNote("notes/nota.md"); err != nil {
		t.Fatalf("DeleteNote: %v", err)
	}
	if err := store.SaveNote("notes/depois.md", "# criada depois", mtime); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	if err := store.SetSetting("semantic_search_threshold", "99"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(docsDir, "pdfs")); err != nil {
		t.Fatal(err)
	}
	// Arquivo presente no momento da restauração: é o que deve ir para a quarentena.
	mustWriteFile(t, filepath.Join(docsDir, "attachments", "atual.txt"), "estado atual")

	// ── restaura ──
	res, err := svc.Restore(context.Background(), zipPath)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if res.Notes != 1 {
		t.Errorf("Notes do resultado = %d, esperado 1", res.Notes)
	}
	if res.Files != 2 {
		t.Errorf("Files do resultado = %d, esperado 2", res.Files)
	}

	// O estado ANTERIOR vai para a quarentena (rede de segurança).
	if _, err := os.Stat(filepath.Join(res.Quarantine, "docs", "attachments", "atual.txt")); err != nil {
		t.Errorf("quarentena sem os arquivos anteriores: %v", err)
	}
	if _, err := os.Stat(filepath.Join(res.Quarantine, filepath.Base(dbPath))); err != nil {
		t.Errorf("quarentena sem o banco anterior: %v", err)
	}

	// O banco restaurado é o arquivo do backup: abre do zero como no boot.
	restored, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("abrir banco restaurado: %v", err)
	}
	defer restored.Close()

	notes, err := restored.GetAllNotes()
	if err != nil {
		t.Fatalf("GetAllNotes: %v", err)
	}
	if _, ok := notes["notes/nota.md"]; !ok {
		t.Errorf("nota original não voltou: %v", notes)
	}
	if _, ok := notes["notes/depois.md"]; ok {
		t.Error("nota criada depois do backup não deveria existir após a restauração")
	}
	if got, _ := restored.GetSetting("semantic_search_threshold"); got != "42" {
		t.Errorf("configuração restaurada = %q, esperado 42", got)
	}

	// Arquivos restaurados, incluindo archives/.
	if _, err := os.Stat(filepath.Join(docsDir, "pdfs", "doc.pdf")); err != nil {
		t.Errorf("pdf não restaurado: %v", err)
	}
	if _, err := os.Stat(filepath.Join(docsDir, "archives", "velha.md")); err != nil {
		t.Errorf("arquivo de archives/ não restaurado: %v", err)
	}
}

// TestFullBackup_RestoreNaoContaDiretorios garante que entradas de diretório do
// ZIP (ex.: "docs/pdfs/") não inflam RestoreResult.Files. Elas não são arquivos
// restaurados; antes, o contador somava +1 para cada uma.
func TestFullBackup_RestoreNaoContaDiretorios(t *testing.T) {
	store, svc, docsDir, _, _ := newFullBackupFixture(t)

	mtime := time.Now().UTC().Format(time.RFC3339)
	if err := store.SaveNote("notes/nota.md", "# Nota", mtime); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	mustWriteFile(t, filepath.Join(docsDir, "pdfs", "doc.pdf"), "%PDF-1.4")

	var buf bytes.Buffer
	if err := svc.CreateStream(context.Background(), &buf); err != nil {
		t.Fatalf("CreateStream: %v", err)
	}
	src := filepath.Join(t.TempDir(), "total.zip")
	if err := os.WriteFile(src, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	zipPath := addDirectoryEntryToZip(t, src, "docs/pdfs/")

	res, err := svc.Restore(context.Background(), zipPath)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if res.Files != 1 {
		t.Errorf("Files = %d, esperado 1 (entrada de diretório não conta)", res.Files)
	}
}

// TestFullBackup_RestoreRejeitaZipSlip garante a proteção contra ZIP malicioso que
// tentaria escrever fora do diretório de destino.
func TestFullBackup_RestoreRejeitaZipSlip(t *testing.T) {
	_, svc, docsDir, _, _ := newFullBackupFixture(t)

	zipPath := writeTestZip(t, map[string]string{
		backupManifestEntry:     `{"format":"ton618-backup","format_version":1}`,
		backupDBEntry:           "nao importa: falha antes da validacao",
		"docs/../../escape.txt": "invasao",
	})

	if _, err := svc.Restore(context.Background(), zipPath); err == nil {
		t.Fatal("esperado erro para caminho com '..' no ZIP")
	}
	for _, escaped := range []string{
		filepath.Join(filepath.Dir(docsDir), "escape.txt"),
		filepath.Join(docsDir, "escape.txt"),
	} {
		if _, err := os.Stat(escaped); err == nil {
			t.Errorf("zip slip escreveu %s", escaped)
		}
	}
}

// TestFullBackup_RestoreRejeitaBancoInvalido garante que um backup com banco
// corrompido é recusado ANTES de qualquer alteração no estado atual.
func TestFullBackup_RestoreRejeitaBancoInvalido(t *testing.T) {
	store, svc, docsDir, stateDir, _ := newFullBackupFixture(t)

	mtime := time.Now().UTC().Format(time.RFC3339)
	if err := store.SaveNote("notes/viva.md", "# continua aqui", mtime); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	mustWriteFile(t, filepath.Join(docsDir, "attachments", "a.txt"), "arquivo atual")

	zipPath := writeTestZip(t, map[string]string{
		backupManifestEntry:         `{"format":"ton618-backup","format_version":1}`,
		backupDBEntry:               "isto não é um banco SQLite",
		"docs/attachments/novo.txt": "arquivo do backup",
	})

	if _, err := svc.Restore(context.Background(), zipPath); err == nil {
		t.Fatal("esperado erro para banco inválido no backup")
	}

	// Nada foi substituído nem colocado em quarentena.
	if _, err := os.Stat(filepath.Join(docsDir, "attachments", "a.txt")); err != nil {
		t.Errorf("estado atual foi alterado (arquivo atual sumiu): %v", err)
	}
	if _, err := os.Stat(filepath.Join(docsDir, "attachments", "novo.txt")); err == nil {
		t.Error("arquivo do backup foi aplicado mesmo com o banco inválido")
	}
	quarantines, _ := filepath.Glob(filepath.Join(stateDir, ".pre-restore-*"))
	if len(quarantines) != 0 {
		t.Errorf("quarentena criada apesar da falha: %v", quarantines)
	}

	notes, err := store.GetAllNotes()
	if err != nil {
		t.Fatalf("GetAllNotes: %v", err)
	}
	if _, ok := notes["notes/viva.md"]; !ok {
		t.Error("nota atual sumiu após uma restauração recusada")
	}
}

// TestFullBackup_RestoreRejeitaManifestAusente e formato desconhecido.
func TestFullBackup_RestoreRejeitaManifestAusente(t *testing.T) {
	_, svc, _, _, _ := newFullBackupFixture(t)

	t.Run("sem manifest", func(t *testing.T) {
		zipPath := writeTestZip(t, map[string]string{backupDBEntry: "x"})
		_, err := svc.Restore(context.Background(), zipPath)
		if err == nil || !strings.Contains(err.Error(), "manifest") {
			t.Fatalf("esperado erro de manifest, got %v", err)
		}
	})

	t.Run("formato desconhecido", func(t *testing.T) {
		zipPath := writeTestZip(t, map[string]string{
			backupManifestEntry: `{"format":"outro-app","format_version":1}`,
			backupDBEntry:       "x",
		})
		_, err := svc.Restore(context.Background(), zipPath)
		if err == nil || !strings.Contains(err.Error(), "formato") {
			t.Fatalf("esperado erro de formato, got %v", err)
		}
	})

	t.Run("sem o banco", func(t *testing.T) {
		zipPath := writeTestZip(t, map[string]string{
			backupManifestEntry: `{"format":"ton618-backup","format_version":1}`,
		})
		_, err := svc.Restore(context.Background(), zipPath)
		if err == nil {
			t.Fatal("esperado erro por falta do banco no backup")
		}
	})
}
