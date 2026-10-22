package processor

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

const tabulatorSample = `{"columns":[{"field":"c1","title":"Nome"},{"field":"c2","title":"Idade","type":"number"},{"field":"c3","title":"Cidade"}],"rows":[{"c1":"Ana","c2":30,"c3":"São Paulo"},{"c1":"Bruno","c2":25,"c3":"Recife"}]}`

func TestParseTabulatorData_PreservaNumeros(t *testing.T) {
	data, err := ParseTabulatorData(tabulatorSample)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(data.Columns) != 3 {
		t.Fatalf("esperado 3 colunas, got %d", len(data.Columns))
	}
	if len(data.Rows) != 2 {
		t.Fatalf("esperado 2 linhas, got %d", len(data.Rows))
	}
	// json.Number preserva a forma original "30".
	if got := FormatCellValue(data.Rows[0]["c2"]); got != "30" {
		t.Errorf("idade deveria ser \"30\", got %q", got)
	}
}

func TestParseTabulatorData_Vazio(t *testing.T) {
	if _, err := ParseTabulatorData("   "); err == nil {
		t.Error("corpo vazio deveria retornar erro")
	}
}

func TestProcessTabulatorContent_IndexaColunasELinhas(t *testing.T) {
	now := time.Now()
	docs := ProcessTabulatorContent(tabulatorSample, "notes/planilha.md", now.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339), []string{"tabulator"})

	// 1 fragmento das colunas + 2 linhas.
	if len(docs) != 3 {
		t.Fatalf("esperado 3 fragmentos, got %d", len(docs))
	}

	for _, d := range docs {
		if d.Tipo != "tabulator" {
			t.Errorf("tipo do fragmento deveria ser \"tabulator\", got %q", d.Tipo)
		}
		if d.Arquivo != "notes/planilha.md" {
			t.Errorf("arquivo errado: %q", d.Arquivo)
		}
	}

	// Colunas
	if docs[0].Secao != "Colunas" {
		t.Errorf("primeiro fragmento deveria ser \"Colunas\", got %q", docs[0].Secao)
	}
	for _, col := range []string{"Nome", "Idade", "Cidade"} {
		if !strings.Contains(docs[0].Texto, col) {
			t.Errorf("fragmento de colunas deveria conter %q, got %q", col, docs[0].Texto)
		}
	}

	// Primeira linha
	if docs[1].Secao != "Registro 1" {
		t.Errorf("esperado \"Registro 1\", got %q", docs[1].Secao)
	}
	for _, needle := range []string{"Nome: Ana", "Idade: 30", "Cidade: São Paulo"} {
		if !strings.Contains(docs[1].Texto, needle) {
			t.Errorf("linha 1 deveria conter %q, got %q", needle, docs[1].Texto)
		}
	}

	// Segunda linha
	if docs[2].Secao != "Registro 2" || !strings.Contains(docs[2].Texto, "Bruno") {
		t.Errorf("linha 2 inesperada: secao=%q texto=%q", docs[2].Secao, docs[2].Texto)
	}
}

func TestProcessTabulatorContent_SemLinhas(t *testing.T) {
	docs := ProcessTabulatorContent(`{"columns":[{"field":"c1","title":"A"}],"rows":[]}`, "notes/x.md", "", "", nil)
	if len(docs) != 1 {
		t.Fatalf("esperado só o fragmento de colunas, got %d", len(docs))
	}
}

func TestProcessTabulatorContent_JSONInvalido(t *testing.T) {
	docs := ProcessTabulatorContent("isso não é json", "notes/x.md", "", "", nil)
	if len(docs) != 1 {
		t.Fatalf("esperado 1 fragmento de fallback, got %d", len(docs))
	}
	if !strings.Contains(docs[0].Texto, "isso não é json") {
		t.Errorf("fallback deveria indexar o texto cru, got %q", docs[0].Texto)
	}
}

// Integração: uma nota com frontmatter "type: tabulator" deve indexar as células
// e ganhar a tag canônica "tabulator".
func TestProcessMarkdownContent_Tabulator(t *testing.T) {
	now := time.Now()
	content := "---\ntype: tabulator\n---\n" + tabulatorSample
	docs, links, tags := ProcessMarkdownContent([]byte(content), "notes/planilha.md", now, now)

	if len(links) != 0 {
		t.Errorf("tabulator não deveria produzir links, got %v", links)
	}
	found := false
	for _, tag := range tags {
		if tag == "tabulator" {
			found = true
		}
	}
	if !found {
		t.Errorf("esperado tag canônica \"tabulator\", got %v", tags)
	}

	if len(docs) != 3 {
		t.Fatalf("esperado 3 fragmentos indexados, got %d", len(docs))
	}
	// O corpo JSON NÃO deve virar um fragmento markdown "Geral".
	for _, d := range docs {
		if d.Secao == SectionDefault {
			t.Errorf("JSON não deveria ser indexado como markdown (secao %q)", d.Secao)
		}
	}
}

// A exportação CSV precisa reconhecer a chave canônica `tipo:` e não pode se
// deixar enganar por "tabulator" citado em outro campo do frontmatter.
func TestTabulatorCSVFromNote_TipoCanonico(t *testing.T) {
	for _, fm := range []string{"tipo: tabulator", "type: tabulator", "type: TABULATOR"} {
		content := "---\n" + fm + "\n---\n" + tabulatorSample
		csv, ok := TabulatorCSVFromNote(content)
		if !ok {
			t.Fatalf("%s: esperado CSV válido", fm)
		}
		if !strings.Contains(string(csv), "Nome") || !strings.Contains(string(csv), "Ana") {
			t.Errorf("%s: CSV não contém cabeçalho/valores esperados: %q", fm, string(csv))
		}
	}
}

func TestTabulatorCSVFromNote_TabulatorEmOutroCampoNaoDetecta(t *testing.T) {
	content := "---\ndescription: 'type: tabulator'\n---\n" + tabulatorSample
	if _, ok := TabulatorCSVFromNote(content); ok {
		t.Error("descrição contendo 'type: tabulator' não deveria classificar a nota como Tabulator")
	}
}

func TestTabulatorToCSV(t *testing.T) {
	data, _ := ParseTabulatorData(tabulatorSample)
	out := TabulatorToCSV(data)

	if !strings.HasPrefix(string(out), "\xEF\xBB\xBF") {
		t.Error("CSV deveria começar com BOM UTF-8")
	}

	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(out), "\xEF\xBB\xBF")))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("CSV inválido: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("esperado header + 2 linhas, got %d", len(records))
	}
	if records[0][0] != "Nome" || records[0][2] != "Cidade" {
		t.Errorf("header errado: %v", records[0])
	}
	if records[1][1] != "30" {
		t.Errorf("valor numérico deveria ser preservado, got %q", records[1][1])
	}
	if records[1][2] != "São Paulo" {
		t.Errorf("acento perdido: %q", records[1][2])
	}
}

func TestTabulatorCSVFromNote(t *testing.T) {
	note := "---\ntype: tabulator\ntags:\n  - dados\n---\n" + tabulatorSample
	out, ok := TabulatorCSVFromNote(note)
	if !ok {
		t.Fatal("esperado CSV válido para nota tabulator")
	}
	if !strings.Contains(string(out), "Ana") {
		t.Errorf("CSV deveria conter os dados, got %q", string(out))
	}

	// Nota que não é tabulator -> não gera CSV.
	if _, ok := TabulatorCSVFromNote("---\ntype: drawing\n---\n{}"); ok {
		t.Error("nota drawing não deveria gerar CSV")
	}
	if _, ok := TabulatorCSVFromNote("texto solto sem frontmatter"); ok {
		t.Error("texto sem frontmatter não deveria gerar CSV")
	}
}

func TestTabulatorCSVFromNote_EscapaCaracteres(t *testing.T) {
	note := `---
type: tabulator
---
{"columns":[{"field":"c1","title":"Texto"}],"rows":[{"c1":"a,b \"aspas\"\ne quebra"}]}`
	out, ok := TabulatorCSVFromNote(note)
	if !ok {
		t.Fatal("esperado CSV válido")
	}
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(out), "\xEF\xBB\xBF")))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("CSV inválido (escape quebrado): %v", err)
	}
	if records[1][0] != "a,b \"aspas\"\ne quebra" {
		t.Errorf("valor escapado errado: %q", records[1][0])
	}
}

func TestTabulatorUploadFilename_Sanitiza(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"relatorio.pdf", "relatorio.pdf"},
		{"relatório final.PDF", "relatório final.PDF"},
		{`C:\Users\giovani\Nota Final.docx`, "Nota Final.docx"},
		{"../../etc/passwd", "passwd"},
		{"pasta/sub/arquivo.txt", "arquivo.txt"},
		{"  espacos.txt  ", "espacos.txt"},
		{"a\x00b\x1fc.txt", "abc.txt"},
		{"", "arquivo"},
		{"   ", "arquivo"},
		{"..", "arquivo"},
		{".env", "env"},
		{".", "arquivo"},
	}

	for _, tt := range tests {
		if got := TabulatorUploadFilename(tt.in); got != tt.want {
			t.Errorf("TabulatorUploadFilename(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTabulatorUploadFilename_LimitaTamanhoPreservandoExtensao(t *testing.T) {
	long := strings.Repeat("á", 300) + ".pdf"
	got := TabulatorUploadFilename(long)

	if n := utf8.RuneCountInString(got); n != 120 {
		t.Errorf("esperado 120 runes, got %d (%q)", n, got)
	}
	if !strings.HasSuffix(got, ".pdf") {
		t.Errorf("extensão perdida: %q", got)
	}
	if strings.ContainsAny(got, `/\`) {
		t.Errorf("nome não pode conter separador de caminho: %q", got)
	}
}

// ── Referências a arquivos de célula (datasets/) ─────────────────

func TestIsTabulatorFilePath(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"datasets/relatorio.pdf", true},
		{"datasets/relatório final.pdf", true},
		{"datasets/a_b.c.png", true},
		{"datasets/", false},
		{"datasets", false},
		{"datasets/.", false},
		{"datasets/..", false},
		{"datasets/../etc/passwd", false},
		{"datasets/sub/arquivo.txt", false},
		{`datasets/sub\arquivo.txt`, false},
		{"notes/relatorio.pdf", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsTabulatorFilePath(tt.in); got != tt.want {
			t.Errorf("IsTabulatorFilePath(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestExtractTabulatorFiles_SomenteValoresDeCelulaValidos(t *testing.T) {
	body := `{"columns":[{"field":"c1","title":"Anexo","type":"file"}],"rows":[` +
		`{"c1":"datasets/relatório final.pdf"},` +
		`{"c1":"datasets/contrato.pdf"},` +
		`{"c1":"datasets/relatório final.pdf"},` +
		`{"c1":"datasets/../etc/passwd"},` +
		`{"c1":"datasets/sub/x.txt"},` +
		`{"c1":""},{"c1":"texto solto"}]}`

	got := ExtractTabulatorFiles(body)
	want := []string{"datasets/relatório final.pdf", "datasets/contrato.pdf"}
	if len(got) != len(want) {
		t.Fatalf("esperado %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("posição %d: esperado %q, got %q", i, want[i], got[i])
		}
	}
}

func TestExtractTabulatorFiles_SemReferencias(t *testing.T) {
	if got := ExtractTabulatorFiles(`{"rows":[{"c1":"Ana"}]}`); len(got) != 0 {
		t.Errorf("esperado nenhum caminho, got %v", got)
	}
	// A palavra "datasets/" no meio de um texto qualquer não vira caminho.
	if got := ExtractTabulatorFiles("veja a pasta datasets/ para detalhes"); len(got) != 0 {
		t.Errorf("esperado nenhum caminho, got %v", got)
	}
}

func TestExtractTabulatorFiles_DesescapaJSON(t *testing.T) {
	body := `{"rows":[{"c1":"datasets/aspa\"nome.pdf"}]}`
	got := ExtractTabulatorFiles(body)
	if len(got) != 1 || got[0] != `datasets/aspa"nome.pdf` {
		t.Errorf("esperado o caminho desescapado, got %v", got)
	}
}
