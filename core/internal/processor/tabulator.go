package processor

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"ton618/core/internal/core/domain"
)

// ── Nota Tabulator ──
//
// Uma nota Tabulator é uma nota markdown comum com frontmatter
//
//	---
//	type: tabulator
//	---
//
// e o corpo guarda o payload JSON da planilha:
//
//	{
//	  "columns": [{"field": "c1", "title": "Nome", "type": "string"}, ...],
//	  "rows":    [{"c1": "Ana", "c2": 30}, ...]
//	}
//
// Diferente de desenho/markmap (cujo corpo NÃO é indexado), as células de uma
// nota Tabulator SÃO indexadas na busca global: cada linha vira um fragmento
// (Document) com as células no formato "Título: valor", e a linha de cabeçalho
// vira um fragmento com os nomes das colunas. Assim, buscar pelo conteúdo de uma
// célula ou pelo nome de uma coluna encontra a nota.

// TabulatorColumn descreve uma coluna da planilha.
type TabulatorColumn struct {
	Field string `json:"field"`
	Title string `json:"title"`
	Type  string `json:"type,omitempty"`
	Width int    `json:"width,omitempty"`
}

// TabulatorData é o payload persistido no corpo da nota Tabulator.
type TabulatorData struct {
	Columns []TabulatorColumn        `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
}

// TabulatorDefaultBody é o corpo inicial de uma nota Tabulator recém-criada:
// uma tabela mínima com duas colunas e uma linha vazia pronta para digitar.
const TabulatorDefaultBody = `{"columns":[{"field":"c1","title":"Coluna 1"},{"field":"c2","title":"Coluna 2"}],"rows":[{"c1":"","c2":""}]}`

// TabulatorColumnTitle devolve o título de exibição de uma coluna, caindo para
// o field quando o título está vazio.
func TabulatorColumnTitle(c TabulatorColumn) string {
	if strings.TrimSpace(c.Title) != "" {
		return c.Title
	}
	return c.Field
}

// ParseTabulatorData desserializa o corpo JSON de uma nota Tabulator. Números são
// preservados como json.Number para não perder a forma original (ex: "30" vs "30.0").
func ParseTabulatorData(body string) (*TabulatorData, error) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return nil, fmt.Errorf("tabulator: corpo vazio")
	}

	dec := json.NewDecoder(strings.NewReader(trimmed))
	dec.UseNumber()

	var data TabulatorData
	if err := dec.Decode(&data); err != nil {
		return nil, fmt.Errorf("tabulator: JSON inválido: %w", err)
	}
	// O decoder lê apenas o primeiro valor. Sem esta checagem, um corpo
	// truncado seguido de lixo (ou de outro JSON) era indexado como válido.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("tabulator: dados extras após o JSON")
	}
	return &data, nil
}

// FormatCellValue converte um valor de célula em texto para indexação/CSV.
func FormatCellValue(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case json.Number:
		return t.String()
	case bool:
		if t {
			return "true"
		}
		return "false"
	case float64:
		return fmt.Sprintf("%v", t)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", t))
	}
}

// ProcessTabulatorContent transforma o corpo JSON de uma nota Tabulator em
// fragmentos indexáveis (um por linha + um com os nomes das colunas).
func ProcessTabulatorContent(body, filename, timestampStr, creationStr string, fileTags []string) []Document {
	data, err := ParseTabulatorData(body)
	if err != nil {
		// Corpo inválido: ainda assim indexa o texto cru para a nota não sumir da
		// busca. Não é o caminho esperado.
		raw := strings.TrimSpace(body)
		if raw == "" {
			return nil
		}
		return []Document{{
			ID:        HashFunc(filename + "#tabulator-raw"),
			Tipo:      string("tabulator"),
			Arquivo:   filename,
			Secao:     "Geral",
			Ordem:     0,
			Texto:     raw,
			Timestamp: timestampStr,
			Created:   creationStr,
			Hash:      CalculateHash("Geral", raw, fileTags),
			Tags:      fileTags,
		}}
	}

	var docs []Document

	// 1. Fragmento com os nomes das colunas (busca por coluna encontra a nota).
	headers := make([]string, 0, len(data.Columns))
	for _, c := range data.Columns {
		if title := TabulatorColumnTitle(c); title != "" {
			headers = append(headers, title)
		}
	}
	if len(headers) > 0 {
		texto := "Colunas: " + strings.Join(headers, ", ")
		docs = append(docs, Document{
			ID:        HashFunc(filename + "#tabulator-cols"),
			Tipo:      string("tabulator"),
			Arquivo:   filename,
			Secao:     "Colunas",
			Ordem:     0,
			Texto:     texto,
			Timestamp: timestampStr,
			Created:   creationStr,
			Hash:      CalculateHash("Colunas", texto, fileTags),
			Tags:      fileTags,
		})
	}

	// 2. Um fragmento por linha.
	knownFields := make(map[string]bool, len(data.Columns))
	for _, c := range data.Columns {
		knownFields[c.Field] = true
	}

	for i, row := range data.Rows {
		parts := make([]string, 0, len(data.Columns)+2)
		for _, c := range data.Columns {
			val := FormatCellValue(row[c.Field])
			if val == "" {
				continue
			}
			parts = append(parts, TabulatorColumnTitle(c)+": "+val)
		}
		// Campos fora da definição de colunas (defensivo) também entram.
		extraKeys := make([]string, 0)
		for k := range row {
			if !knownFields[k] && !strings.HasPrefix(k, "_") {
				extraKeys = append(extraKeys, k)
			}
		}
		sort.Strings(extraKeys)
		for _, k := range extraKeys {
			val := FormatCellValue(row[k])
			if val == "" {
				continue
			}
			parts = append(parts, k+": "+val)
		}

		secao := fmt.Sprintf("Registro %d", i+1)
		texto := strings.Join(parts, " | ")
		if texto == "" {
			// Linha totalmente vazia: usa um marcador para continuar contável e
			// representar a linha na busca.
			texto = secao
		}
		docs = append(docs, Document{
			ID:        HashFunc(fmt.Sprintf("%s#tabulator-row-%d", filename, i)),
			Tipo:      string("tabulator"),
			Arquivo:   filename,
			Secao:     secao,
			Ordem:     i + 1,
			Texto:     texto,
			Timestamp: timestampStr,
			Created:   creationStr,
			Hash:      CalculateHash(secao, texto, fileTags),
			Tags:      fileTags,
		})
	}

	return docs
}

// TabulatorUploadFilename normaliza o nome de um arquivo enviado para uma célula
// de uma coluna "arquivo" de nota Tabulator:
//
//   - descarta qualquer diretório (só o basename — nem "/" nem "\" do Windows);
//   - remove caracteres de controle e espaços nas bordas;
//   - evita nomes vazios ou compostos só de pontos ("." / ".." → "arquivo");
//   - limita o tamanho (em runes) preservando a extensão.
//
// O resultado nunca contém separadores de caminho nem bytes de controle.
func TabulatorUploadFilename(raw string) string {
	name := raw
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}

	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	name = strings.Trim(name, " \t.")
	if name == "" {
		return "arquivo"
	}

	const maxRunes = 120
	if utf8.RuneCountInString(name) > maxRunes {
		ext := filepath.Ext(name)
		if utf8.RuneCountInString(ext) > 20 {
			ext = ""
		}
		base := []rune(strings.TrimSuffix(name, ext))
		keep := maxRunes - utf8.RuneCountInString(ext)
		if keep < 1 {
			keep = 1
		}
		if len(base) > keep {
			base = base[:keep]
		}
		name = string(base) + ext
	}
	return name
}

// tabulatorFileRefRe casa o valor de uma célula de coluna "arquivo" no JSON da
// nota: uma string JSON que começa em "datasets/". Nomes podem ter espaços e
// acentos; \" e \\ escapados são aceitos (e desescapados depois).
var tabulatorFileRefRe = regexp.MustCompile(`"datasets/((?:[^"\\]|\\.)+)"`)

// IsTabulatorFilePath informa se p é um caminho válido de arquivo de célula de
// nota Tabulator: exatamente "datasets/<nome>", com um único nível (os uploads
// são gravados "flat" em datasets/) e sem separador/escape de caminho. É a
// guarda usada antes de apagar qualquer arquivo.
func IsTabulatorFilePath(p string) bool {
	if !strings.HasPrefix(p, domain.DatasetPrefix) {
		return false
	}
	name := strings.TrimPrefix(p, domain.DatasetPrefix)
	if name == "" || name == "." || name == ".." {
		return false
	}
	return !strings.ContainsAny(name, `/\`)
}

// ExtractTabulatorFiles devolve, sem repetição, os caminhos "datasets/..."
// referenciados no conteúdo de uma nota (os valores das células de coluna
// "arquivo"). Só devolve caminhos que passam em IsTabulatorFilePath, de modo que
// o resultado é seguro para apagar/verificar no disco.
func ExtractTabulatorFiles(content string) []string {
	if !strings.Contains(content, domain.DatasetPrefix) {
		return nil
	}

	var out []string
	seen := make(map[string]bool)
	for _, m := range tabulatorFileRefRe.FindAllStringSubmatch(content, -1) {
		name := strings.ReplaceAll(m[1], `\"`, `"`)
		name = strings.ReplaceAll(name, `\\`, `\`)
		p := domain.DatasetPrefix + name
		if seen[p] || !IsTabulatorFilePath(p) {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// TabulatorToCSV serializa os dados de uma planilha como CSV. Inclui o BOM UTF-8
// para que o Excel reconheça acentos corretamente.
func TabulatorToCSV(data *TabulatorData) []byte {
	if data == nil {
		return nil
	}

	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // BOM UTF-8
	w := csv.NewWriter(&buf)

	headers := make([]string, len(data.Columns))
	for i, c := range data.Columns {
		headers[i] = TabulatorColumnTitle(c)
	}
	_ = w.Write(headers)

	for _, row := range data.Rows {
		record := make([]string, len(data.Columns))
		for i, c := range data.Columns {
			record[i] = FormatCellValue(row[c.Field])
		}
		_ = w.Write(record)
	}

	w.Flush()
	return buf.Bytes()
}

// TabulatorCSVFromNote devolve o CSV de uma nota Tabulator completa (frontmatter
// + corpo JSON). ok=false quando a nota não é do tipo tabulator ou o corpo não é
// um payload válido. Usado pela exportação/backup.
func TabulatorCSVFromNote(content string) ([]byte, bool) {
	body, ok := tabulatorBodyFromNote(content)
	if !ok {
		return nil, false
	}
	data, err := ParseTabulatorData(body)
	if err != nil || data == nil {
		return nil, false
	}
	return TabulatorToCSV(data), true
}

// tabulatorBodyFromNote extrai o corpo JSON de uma nota cujo frontmatter declara
// o tipo tabulator (`tipo:` canônico ou `type:` legado, case-insensitive).
// Usa o mesmo parser/normalização do indexador, em vez de procurar a substring
// "tabulator" no YAML (que casava até `description: "type: tabulator"`).
func tabulatorBodyFromNote(content string) (string, bool) {
	text := strings.TrimLeft(content, " \t\r\n\xef\xbb\xbf")
	body, _, tipo, _ := parseFrontmatterForIndex(text)
	if tipo != "tabulator" {
		return "", false
	}
	return strings.TrimSpace(body), true
}
