package services

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ton618/core/internal/core/db"
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
	os.MkdirAll(filepath.Join(docsDir, "attachments"), 0755)
	os.WriteFile(filepath.Join(docsDir, "attachments", "foto.png"), []byte("fake image data"), 0644)

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
	if !strings.Contains(zipStr, "attachments/foto.png") {
		t.Error("backup full deveria conter attachments/foto.png")
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
	if !foundNormal {
		t.Error("nota-normal.md nao encontrado no zip")
	}
}

// TestBackup_ParallelZipValido garante que o ZIP gerado pela compressão
// paralela (flate + zip.Writer.CreateRaw) é um ZIP válido, com CRC correto e
// conteúdo idêntico ao do banco. Sem isso, um erro no CRC/tamanho de qualquer
// entrada corromperia o backup silenciosamente.
func TestBackup_ParallelZipValido(t *testing.T) {
	store, svc, _ := newStoreAndBackup(t)

	// Notas suficientes para o pool rodar com >1 worker
	const n = 12
	for i := 0; i < n; i++ {
		content := fmt.Sprintf("# Nota %d\n\n%s\n", i, strings.Repeat("conteudo de teste para compressao repetida ", 20))
		if err := store.SaveNote(fmt.Sprintf("notes/nota%d.md", i), content, time.Now().Format(time.RFC3339)); err != nil {
			t.Fatalf("SaveNote: %v", err)
		}
	}

	data, err := svc.Create(false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("ZIP inválido após compressão paralela: %v", err)
	}

	found := 0
	for _, f := range zr.File {
		if f.Method != zip.Deflate {
			t.Errorf("entrada %s deveria usar DEFLATE, got %d", f.Name, f.Method)
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("abrir %s: %v", f.Name, err)
		}
		buf, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("ler %s: %v", f.Name, err)
		}
		if f.CRC32 != crc32.ChecksumIEEE(buf) {
			t.Errorf("CRC inválido em %s", f.Name)
		}
		if strings.HasPrefix(f.Name, "notes/nota") {
			found++
			if !strings.Contains(string(buf), "conteudo de teste") {
				t.Errorf("conteúdo de %s não bate com o banco", f.Name)
			}
		}
	}
	if found != n {
		t.Errorf("esperava %d notas no ZIP, got %d", n, found)
	}
}

// TestBackup_CreateStreamContext_Cancelled garante que um contexto cancelado
// (ex: cliente desconectou) aborta o backup com context.Canceled em vez de
// continuar comprimindo uma conexão morta.
func TestBackup_CreateStreamContext_Cancelled(t *testing.T) {
	store, svc, _ := newStoreAndBackup(t)

	for i := 0; i < 20; i++ {
		if err := store.SaveNote(fmt.Sprintf("notes/nota%d.md", i), fmt.Sprintf("# Nota %d\n%s", i, strings.Repeat("x", 2000)), time.Now().Format(time.RFC3339)); err != nil {
			t.Fatalf("SaveNote: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // simula desconexão já ocorrida antes do início

	var buf bytes.Buffer
	err := svc.CreateStreamContext(ctx, &buf, false)
	if err == nil {
		t.Fatal("esperava erro de contexto cancelado")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("esperava context.Canceled, got %v", err)
	}
}

// failingWriter falha em qualquer escrita (simula cliente desconectado / conexão quebrada).
type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) { return 0, io.ErrClosedPipe }

// TestBackup_CreateStreamContext_PropagaErroDeEscrita garante que um erro de
// escrita não é engolido (antes, `_ = addToZip(...)` escondia falhas e entregava
// um ZIP truncado sem nenhum log no servidor).
func TestBackup_CreateStreamContext_PropagaErroDeEscrita(t *testing.T) {
	store, svc, _ := newStoreAndBackup(t)
	if err := store.SaveNote("notes/uma.md", "# Uma\nconteudo para backup", time.Now().Format(time.RFC3339)); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}

	err := svc.CreateStreamContext(context.Background(), failingWriter{}, false)
	if err == nil {
		t.Fatal("esperava erro de escrita propagado")
	}
}


