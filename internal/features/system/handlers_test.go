package system

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// createMinimalPDF escreve um PDF valido com o texto informado.
func createMinimalPDF(t *testing.T, path, text string) {
	t.Helper()
	// Usa padding para manter o texto com pelo menos 11 chars, igual ao "Hello PDF"
	// usado nos offsets fixos do xref.
	paddedText := text
	if len(paddedText) < 11 {
		paddedText = paddedText + strings.Repeat(" ", 11-len(paddedText))
	}
	os.MkdirAll(filepath.Dir(path), 0755)
	content := fmt.Sprintf("%%PDF-1.4\n"+
		"1 0 obj\n"+
		"<< /Type /Catalog /Pages 2 0 R >>\n"+
		"endobj\n"+
		"2 0 obj\n"+
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>\n"+
		"endobj\n"+
		"3 0 obj\n"+
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792]\n"+
		"   /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>\n"+
		"endobj\n"+
		"4 0 obj\n"+
		"<< /Length 44 >>\n"+
		"stream\n"+
		"BT /F1 12 Tf 100 700 Td (%s) Tj ET\n"+
		"endstream\n"+
		"endobj\n"+
		"5 0 obj\n"+
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\n"+
		"endobj\n"+
		"xref\n"+
		"0 6\n"+
		"0000000000 65535 f \n"+
		"0000000009 00000 n \n"+
		"0000000058 00000 n \n"+
		"0000000115 00000 n \n"+
		"0000000266 00000 n \n"+
		"0000000363 00000 n \n"+
		"trailer\n"+
		"<< /Size 6 /Root 1 0 R >>\n"+
		"startxref\n"+
		"442\n"+
		"%%EOF", paddedText)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("escrever PDF: %v", err)
	}
}
