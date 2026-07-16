package services

import (
	"archive/zip"
	"bytes"
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

func TestBackup_Conversions(t *testing.T) {
	store, svc, _ := newStoreAndBackup(t)

	// Nota de desenho
	drawingContent := "---\ntype: drawing\n---\n{\"elements\": []}"
	store.SaveNote("notes/meu-desenho.md", drawingContent, time.Now().Format(time.RFC3339))

	// Nota de planilha
	sheetContent := "---\ntype: spreadsheet\n---\n{\"data\": [[\"Header1\", \"Header2\"], [\"Value1\", \"Value2\"]]}"
	store.SaveNote("notes/minha-planilha.md", sheetContent, time.Now().Format(time.RFC3339))

	// Nota de diagrama Mermaid
	mermaidContent := "---\ntype: mermaid\n---\ngraph TD\nA[Inicio] --> B(Fim)"
	store.SaveNote("notes/meu-diagrama.md", mermaidContent, time.Now().Format(time.RFC3339))

	// Nota normal
	markdownContent := "---\ntitle: Minha Nota\n---\n# Ola"
	store.SaveNote("notes/nota-normal.md", markdownContent, time.Now().Format(time.RFC3339))

	data, err := svc.Create(false)
	if err != nil {
		t.Fatalf("Backup Create failed: %v", err)
	}

	// Le o ZIP usando archive/zip
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}

	foundDrawing := false
	foundSpreadsheet := false
	foundMermaid := false
	foundNormal := false

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("failed to open file in zip: %v", err)
		}
		var contentBuf bytes.Buffer
		contentBuf.ReadFrom(rc)
		rc.Close()
		fileContent := contentBuf.String()

		switch f.Name {
		case "notes/meu-desenho.excalidraw":
			foundDrawing = true
			if !strings.Contains(fileContent, `{"elements": []}`) {
				t.Errorf("conteudo de meu-desenho.excalidraw incorreto: %q", fileContent)
			}
		case "notes/minha-planilha.csv":
			foundSpreadsheet = true
			if !strings.Contains(fileContent, "Header1,Header2") {
				t.Errorf("conteudo de minha-planilha.csv incorreto: %q", fileContent)
			}
		case "notes/meu-diagrama.mmd":
			foundMermaid = true
			if !strings.Contains(fileContent, "graph TD") || strings.Contains(fileContent, "type: mermaid") {
				t.Errorf("conteudo de meu-diagrama.mmd incorreto (deveria conter apenas o corpo de codigo): %q", fileContent)
			}
		case "notes/nota-normal.md":
			foundNormal = true
			if !strings.Contains(fileContent, "# Ola") {
				t.Errorf("conteudo de nota-normal.md incorreto: %q", fileContent)
			}
		}
	}

	if !foundDrawing {
		t.Error("meu-desenho.excalidraw nao encontrado no zip")
	}
	if !foundSpreadsheet {
		t.Error("minha-planilha.csv nao encontrado no zip")
	}
	if !foundMermaid {
		t.Error("meu-diagrama.mmd nao encontrado no zip")
	}
	if !foundNormal {
		t.Error("nota-normal.md nao encontrado no zip")
	}
}


