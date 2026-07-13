package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ton618/internal/core/db"
)

func newStoreAndBackup(t *testing.T) (*db.Store, *BackupService, string) {
	t.Helper()
	docsDir := t.TempDir()
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "test.db")

	store, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	svc := NewBackupService(store, store, docsDir)
	return store, svc, docsDir
}

func TestBackup_QuickCreatesZip(t *testing.T) {
	store, svc, docsDir := newStoreAndBackup(t)

	// Cria algumas notas no disco e no banco
	os.MkdirAll(filepath.Join(docsDir, "notes"), 0755)
	os.WriteFile(filepath.Join(docsDir, "notes", "nota1.md"), []byte("# Nota 1\nConteudo"), 0644)
	os.WriteFile(filepath.Join(docsDir, "notes", "nota2.md"), []byte("# Nota 2\nOutro conteudo"), 0644)
	store.SaveNote("notes/nota1.md", "# Nota 1\nConteudo", time.Now().Format(time.RFC3339))
	store.SaveNote("notes/nota2.md", "# Nota 2\nOutro conteudo", time.Now().Format(time.RFC3339))

	// Cria um PDF (que deve ser incluido no backup completo mas nao no rapido)
	os.MkdirAll(filepath.Join(docsDir, "pdfs"), 0755)
	pdfContent := []byte("%PDF-1.4 fake content for testing")
	os.WriteFile(filepath.Join(docsDir, "pdfs", "doc.pdf"), pdfContent, 0644)

	// Backup rapido (full=false) — apenas notas + dados, sem PDFs
	data, err := svc.Create(false)
	if err != nil {
		t.Fatalf("Backup Create (quick): %v", err)
	}
	if len(data) == 0 {
		t.Fatal("backup vazio")
	}

	// Verifica que o ZIP contem as notas
	zipStr := string(data)
	if !strings.Contains(zipStr, "nota1.md") {
		t.Error("backup rapido deveria conter nota1.md")
	}
	if !strings.Contains(zipStr, "nota2.md") {
		t.Error("backup rapido deveria conter nota2.md")
	}

	t.Logf("Backup rapido gerado: %d bytes", len(data))
}

func TestBackup_FullIncludesPDFs(t *testing.T) {
	store, svc, docsDir := newStoreAndBackup(t)

	os.MkdirAll(filepath.Join(docsDir, "notes"), 0755)
	os.WriteFile(filepath.Join(docsDir, "notes", "nota1.md"), []byte("# Nota"), 0644)
	os.MkdirAll(filepath.Join(docsDir, "pdfs"), 0755)
	os.WriteFile(filepath.Join(docsDir, "pdfs", "doc.pdf"), []byte("%PDF-1.4 fake content for testing backup full mode"), 0644)
	store.SetFileMod("pdfs/doc.pdf", time.Now().Format(time.RFC3339))
	store.SaveNote("notes/nota1.md", "# Nota", time.Now().Format(time.RFC3339))

	// Backup completo (full=true)
	data, err := svc.Create(true)
	if err != nil {
		t.Fatalf("Backup Create (full): %v", err)
	}

	zipStr := string(data)
	if !strings.Contains(zipStr, "nota1.md") {
		t.Error("backup full deveria conter nota1.md")
	}
	if !strings.Contains(zipStr, "pdfs/doc.pdf") {
		t.Error("backup full deveria conter pdfs/doc.pdf")
	}

	t.Logf("Backup full gerado: %d bytes", len(data))
}

func TestBackup_EmptyDocs(t *testing.T) {
	_, svc, _ := newStoreAndBackup(t)

	data, err := svc.Create(false)
	if err != nil {
		t.Fatalf("Backup Create (empty): %v", err)
	}
	if len(data) == 0 {
		t.Fatal("backup de diretorio vazio nao deveria ser vazio (deve conter ao menos metadados)")
	}
	t.Logf("Backup vazio gerado: %d bytes", len(data))
}
