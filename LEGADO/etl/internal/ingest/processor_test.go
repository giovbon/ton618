package ingest

import (
	"encoding/base64"
	"etl/internal/config"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProcessPDF(t *testing.T) {
	// PDF minimalista (1 página, "Hello World") gerado via GS
	pdfBase64 := "JVBERi0xLjcKJcfsj6IKJSVJbnZvY2F0aW9uOiBncyAtcSAtZE5PUEFVU0UgLWRCQVRDSCAtc0RFVklDRT1wZGZ3cml0ZSAtc091dHB1dEZpbGU9PyAtCjUgMCBvYmoKPDwvTGVuZ3RoIDYgMCBSL0ZpbHRlciAvRmxhdGVEZWNvZGU+PgpzdHJlYW0KeJwrVDDQM1QwAEEonZzLZaCQzlXIZQgWVYBSybkKTiFc+kHmCoZGCiFpXBDFhkBpAwVzIA7J5dLwSM3JyVcIzy/KSdEMyeJyDeEKBEIA3Y8VOmVuZHN0cmVhbQplbmRvYmoKNCAwIG9iago8PC9UeXBlL1BhZ2UvTWVkaWFCb3ggWzAgMCA1OTUgODQyXQovUm90YXRlIDAvUGFyZW50IDMgMCBSCi9SZXNvdXJjZXM8PC9Qcm9jU2V0Wy9QREYgL1RleHRdCi9Gb250IDkgMCBSCj4+Ci9Db250ZW50cyA1IDAgUgo+PgplbmRvYmoKMyAwIG9iago8PCAvVHlwZSAvUGFnZXMgL0tpZHMgWwo0IDAgUgpdIC9Db3VudCAxCj4+CmVuZG9iagoxIDAgb2JqCjw8L1R5cGUgL0NhdGFsb2cgL1BhZ2VzIDMgMCBSCi9NZXRhZGF0YSAxMSAwIFIKPj4KZW5kb2JqCjEwIDAgb2JqCjw8L0ZpbHRlci9GbGF0ZURlY29kZS9MZW5ndGggMjA3Pj5zdHJlYW0KeJxdkDEOwjAMRfecIjeoW1FgiLzAwgBCwAWC66AMpFEoA7fHcYGB4X3pJ7b0v5vNbrtLcbLNsYx05smGmIbCj/FZiO2VbzGZtrNDpOnjVOnus2k2e58vr8xWBjjM/uDv3Jxa0Jd23qFx4Ef2xMWnGxsHgC4ENJyGv6/1vHANn8lOJisAosYt1qgAiBrXr1ABEDVuuUAFQFRsj4rYvlpCRSxVG1ARK0ncqkMFQLQG+0aoGWvZbzdLz1I4TXoRbVybxsS/o+Ux1y0rmDdMqWm1CmVuZHN0cmVhbQplbmRvYmoKMTEgMCBvYmoKPDwvVHlwZS9NZXRhZGF0YQovU3VidHlwZS9YTUwvTGVuZ3RoIDE0NTc+PnN0cmVhbQo8P3hwYWNrZXQgYmVnaW49J++7vycgaWQ9J1c1TTBNcENlaGlIenJlU3pOVGN6a2M5ZCc/Pgo8P2Fkb2JlLXhhcC1maWx0ZXJzIGVzYz0iQ1JMRiI/Pgo8eDp4bXBtZXRhIHhtbG5zOng9J2Fkb2JlOm5zOm1ldGEvJyB4OnhtcHRrPSdYTVAgdG9vbGtpdCAyLjkuMS0xMywgZnJhbWV3b3JrIDEuNic+CjxyZGY6UkRGIHhtbG5zOnJkZj0naHR0cDovL3d3dy53My5vcmcvMTk5OS8wMi8yMi1yZGYtc3ludGF4LW5zIycgeG1sbnM6aVg9J2h0dHA6Ly9ucy5hZG9iZS5jb20vaVgvMS4wLyc+CjxyZGY6RGVzY3JpcHRpb24gcmRmOmFib3V0PSIiIHhtbG5zOnBkZj0naHR0cDovL25zLmFkb2JlLmNvbS9wZGYvMS4zLycgcGRmOlByb2R1Y2VyPSdHUEwgR2hvc3RzY3JpcHQgMTAuMDcuMCcvPgo8cmRmOkRlc2NyaXB0aW9uIHJkZjphYm91dD0iIiB4bWxuczp4bXA9J2h0dHA6Ly9ucy5hZG9iZS5jb20veGFwLzEuMC8nPjx4bXA6TW9kaWZ5RGF0ZT4yMDI2LTA0LTExVDExOjM5OjU0LTAzOjAwPC94bXA6TW9kaWZ5RGF0ZT4KPHhtcDpDcmVhdGVEYXRlPjIwMjYtMDQtMTFUMTE6Mzk6NTQtMDM6MDA8L3htcDpDcmVhdGVEYXRlPgo8eG1wOk1ldGFkYXRhRGF0ZT4yMDI2LTA0LTExVDExOjM5OjU0LTAzOjAwPC94bXA6TWV0YWRhdGFEYXRlPgo8eG1wOkNyZWF0b3JUb29sPidVbmtub3duQXBwbGljYXRpb24nPC94bXA6Q3JlYXRvclRvb2w+PC9yZGY6RGVzY3JpcHRpb24+CjxyZGY6RGVzY3JpcHRpb24gcmRmOmFib3V0PSIiIHhtbG5zOnhtcE1NPSdodHRwOi8vbnMuYWRvYmUuY29tL3hhcC8xLjAvbW0vJyB4bXBNTTpEb2N1bWVudElEPSd1dWlkOmRhYTMxNGNmLTZkZDAtMTFmYy0wMDAwLTYxZDE5NTdjZTQzNycvPgo8cmRmOkRlc2NyaXB0aW9uIHJkZjphYm91dD0iIiB4bWxuczp4bXBNTT0naHR0cDovL25zLmFkb2JlLmNvbS94YXAvMS4wL21tLycgeG1wTU06UmVuZGl0aW9uQ2xhc3M9J2RlZmF1bHQnLz4KPHJkZjpEZXNjcmlwdGlvbiByZGY6YWJvdXQ9IiIgeG1sbnM6eG1wTU09J2h0dHA6Ly9ucy5hZG9iZS5jb20veGFwLzEuMC9tbS8nIHhtcE1NOlZlcnNpb25JRD0nMScvPgo8cmRmOkRlc2NyaXB0aW9uIHJkZjphYm91dD0iIiB4bWxuczpkYz0naHR0cDovL3B1cmwub3JnL2RjL2VsZW1lbnRzLzEuMS8nIGRjOmZvcm1hdD0nYXBwbGljYXRpb24vcGRmJz48ZGM6dGl0bGU+PHJkZjpBbHQ+PHJkZjpsaSB4bWw6bGFuZz0neC1kZWZhdWx0Jz4nVW50aXRsZWQnPC9yZGY6bGk+PC9yZGY6QWx0PjwvZGM6dGl0bGU+PC9yZGY6RGVzY3JpcHRpb24+CjwvcmRmOlJERj4KPC94OnhtcG1ldGE+CiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKPD94cGFja2V0IGVuZD0ndyc/PgplbmRzdHJlYW0KZW5kb2JqCjggMCBvYmoKPDwvRmlsdGVyL0ZsYXRlRGVjb2RlCi9UeXBlL09ialN0bQovTiA0Ci9GaXJzdCAxOC9MZW5ndGggMTY2Pj5zdHJlYW0KeJx9jLEOgjAYhPc+xb+Bg/SHIgVDOigRB00I4gPU0sQmhhIoJr69NO7ednffXQYIBTDgEHNIIGeQJ6QsacsJX6tWCO8OctYnOzh61q+3dkZJ2tn7YJTtNcToQdp9Rk09ROhteTjvfBT/HprJ9ovSU1g3F6ifdnazmszo1nWEPMINocdJS2fsUEmnw2qfYJJhGq9ixS7dIgsQgxW72v4vIcQXHqU5LQplbmRzdHJlYW0KZW5kb2JqCjEyIDAgb2JqCjw8Ci9UeXBlIC9YUmVmCi9TaXplIDEzCi9Sb290IDEgMCBSIC9JbmZvIDIgMCBSCi9JRCBbPDU1MTdBREI4RjlDNjg2RTY0NzI4NkRBMkQwQjhCMUUxPjw1NTE3QURCOEY5QzY4NkU2NDcyODZEQTJEMEI4QjFFMT5dCi9JbmRleCBbMCAxMyBdCi9XIFsxIDIgMl0KL0ZpbHRlciAvRmxhdGVEZWNvZGUvTGVuZ3RoIDU3Cj4+CnN0cmVhbQp4nGNgYPj/n5FxBwMDEwMHAzMjYy0DAyPDBxARAREDE0yMnFwQFiMj40+gLDMPkODiZ2AAAAgRBekKZW5kc3RyZWFtCmVuZG9iagpzdGFydHhyZWYKMjU3NQolJUVPRgo="

	pdfContent, _ := base64.StdEncoding.DecodeString(pdfBase64)

	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	err := os.WriteFile(pdfPath, pdfContent, 0644)
	if err != nil {
		t.Fatalf("Erro ao criar arquivo PDF de teste: %v", err)
	}

	// 2. Executar ProcessPDF
	modTime := time.Now()
	appState := NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	defer appState.Close()
	docs := ProcessPDF(pdfPath, "test.pdf", modTime, appState)

	// 3. Validar resultados
	if len(docs) != 0 {
		t.Errorf("Esperava 0 documentos (extração desativada), obteve %d", len(docs))
	}
}

func TestProcessMarkdown(t *testing.T) {
	tmpDir := t.TempDir()
	mdPath := filepath.Join(tmpDir, "test.md")

	content := `---
tags: [golang, setup]
---
# Titulo
Conteudo [[Outra Nota]]`

	err := os.WriteFile(mdPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Erro ao criar arquivo MD de teste: %v", err)
	}

	modTime := time.Now()
	appState := NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	defer appState.Close()
	docs, links, _, _, _ := ProcessMarkdown(mdPath, "test.md", modTime, appState)

	if len(docs) == 0 {
		t.Fatalf("ProcessMarkdown não gerou nenhum documento")
	}

	if len(links) != 1 || links[0] != "outra nota.md" {
		t.Errorf("Link esperado 'outra nota.md', obteve %v", links)
	}

	doc := docs[0]
	expectedTags := []string{"golang", "setup"}

	if len(doc.Tags) != len(expectedTags) {
		t.Errorf("Esperado %v tags, obteve %v", len(expectedTags), len(doc.Tags))
	}

	for i, tag := range doc.Tags {
		if tag != expectedTags[i] {
			t.Errorf("Tag no indice %d: esperado %s, obteve %s", i, expectedTags[i], tag)
		}
	}
}

func TestProcessMarkdown_DirtyTags(t *testing.T) {
	tmpDir := t.TempDir()
	mdPath := filepath.Join(tmpDir, "dirty.md")

	content := `---
tags: [  GOLANG , Setup , "" , "com espaço" ]
---
# Test
Conteúdo de teste para garantir processamento.`

	err := os.WriteFile(mdPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Erro ao criar arquivo MD: %v", err)
	}

	appState := NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	defer appState.Close()
	docs, _, _, _, _ := ProcessMarkdown(mdPath, "dirty.md", time.Now(), appState)
	if len(docs) == 0 {
		t.Fatalf("ProcessMarkdown não gerou docs")
	}

	doc := docs[0]
	// "GOLANG" -> "golang", "Setup" -> "setup", "" -> removido, "com espaço" -> "com espaço"
	expected := []string{"golang", "setup", "com espaço"}

	if len(doc.Tags) != len(expected) {
		t.Errorf("Esperado %d tags, obteve %d: %v", len(expected), len(doc.Tags), doc.Tags)
	}
}
