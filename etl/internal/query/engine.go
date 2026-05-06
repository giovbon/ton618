package query

import (
	"etl/internal/config"
	"etl/internal/ingest"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type QueryResult struct {
	Headers []string        `json:"headers"`
	Rows    [][]interface{} `json:"rows"`
	Type    string          `json:"type"` // "table" or "list"
}

var (
	// Suporte: TABLE f1, f2 [FROM source] [WHERE cond1 AND cond2] [SORT field ASC|DESC]
	queryRegex = regexp.MustCompile(`(?i)^(TABLE|LIST)(?:\s+(.*?))?(?:\s+FROM\s+([^\s]+))?(?:\s+WHERE\s+(.*?))?(?:\s+SORT\s+([a-zA-Z0-9_\.]+)\s+(ASC|DESC))?\s*$`)
)

func Execute(q string, state *ingest.AppState, cfg *config.AppConfig) (*QueryResult, error) {
	q = strings.TrimSpace(q)
	matches := queryRegex.FindStringSubmatch(q)
	if len(matches) < 3 {
		return nil, fmt.Errorf("sintaxe inválida: use TABLE/LIST ... [FROM ...] [WHERE ...] [SORT ...]")
	}

	command := strings.ToUpper(matches[1])
	fieldsRaw := strings.TrimSpace(matches[2])
	source := strings.TrimSpace(matches[3])
	whereClause := strings.TrimSpace(matches[4])
	sortField := strings.TrimSpace(matches[5])
	sortOrder := strings.ToUpper(matches[6])

	// Workaround for LIST without fields where FROM gets caught in fieldsRaw due to regex optional spaces
	if command == "LIST" && source == "" && strings.HasPrefix(strings.ToUpper(fieldsRaw), "FROM ") {
		source = strings.TrimSpace(fieldsRaw[5:])
		fieldsRaw = ""
	}

	// 1. Determinar Headers
	var headers []string
	if command == "TABLE" {
		headers = append(headers, "File")
		if fieldsRaw != "" {
			parts := strings.Split(fieldsRaw, ",")
			for _, v := range parts {
				headers = append(headers, strings.TrimSpace(v))
			}
		}
	} else {
		headers = []string{"File"}
	}

	// 2. Coletar Arquivos
	allMetadata := state.GetAllFileMetadata()
	files := []string{}
	if source == "" {
		for f := range allMetadata {
			if strings.HasSuffix(f, ".md") {
				files = append(files, f)
			}
		}
	} else if strings.HasPrefix(source, "#") {
		tag := strings.ToLower(source[1:])
		for f := range allMetadata {
			if hasTag(state.GetFileTags(f), tag) {
				files = append(files, f)
			}
		}
	} else {
		prefix := strings.Trim(source, "\"")
		for f := range allMetadata {
			if strings.HasPrefix(f, prefix) {
				files = append(files, f)
			}
		}
	}

	log.Printf("[Query] Motor encontrou %d arquivos candidatos para fonte '%s'\n", len(files), source)

	// 3. Filtrar
	filteredFiles := files
	if whereClause != "" {
		filteredFiles = []string{}
		conds, err := parseConditions(whereClause)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			meta := getFullMetadata(f, state, cfg)
			if matchAll(meta, conds) {
				filteredFiles = append(filteredFiles, f)
			}
		}
		log.Printf("[Query] Filtro WHERE '%s' resultou em %d arquivos\n", whereClause, len(filteredFiles))
	}

	// 4. Montar Resultados
	rows := [][]interface{}{}
	isAggregation := false
	for _, h := range headers {
		if strings.HasPrefix(h, "count(") && strings.HasSuffix(h, ")") {
			isAggregation = true
			break
		}
	}

	if isAggregation {
		row := []interface{}{}
		for _, h := range headers {
			if strings.HasPrefix(h, "count(") {
				row = append(row, len(filteredFiles))
			} else if h == "File" {
				row = append(row, "Summary")
			} else {
				row = append(row, nil)
			}
		}
		rows = append(rows, row)
	} else {
		for _, f := range filteredFiles {
			meta := getFullMetadata(f, state, cfg)
			row := []interface{}{}
			if command == "TABLE" {
				row = append(row, f) // File column
				for _, h := range headers[1:] {
					row = append(row, meta[h])
				}
			} else {
				row = append(row, f)
			}
			rows = append(rows, row)
		}
	}

	// 5. Ordenar
	if sortField != "" {
		sortResults(rows, headers, sortField, sortOrder)
	}

	return &QueryResult{
		Headers: headers,
		Rows:    rows,
		Type:    strings.ToLower(command),
	}, nil
}

// getFullMetadata combina Frontmatter com metadados implícitos (file.*)
func getFullMetadata(filename string, state *ingest.AppState, cfg *config.AppConfig) map[string]interface{} {
	meta := state.GetFileMetadata(filename)
	if meta == nil {
		meta = make(map[string]interface{})
	}

	// Injetar campos virtuais "file.*"
	meta["file.name"] = filepath.Base(filename)
	meta["file.path"] = filename

	// Tamanho do arquivo
	if info, err := os.Stat(filepath.Join(cfg.DocsDir, filename)); err == nil {
		meta["file.size"] = fmt.Sprintf("%.2f KB", float64(info.Size())/1024.0)
	}

	// Tentar encontrar a data de modificação (pode estar como path relativo ou absoluto no cache)
	allMods := state.GetAllFileMods()
	if mtime, ok := allMods[filename]; ok {
		meta["file.mtime"] = mtime.Local().Format("2006-01-02 15:04")
	} else {
		// Busca por sufixo (caso esteja absoluto no cache e relativo na query)
		for path, mtime := range allMods {
			if strings.HasSuffix(filepath.ToSlash(path), filepath.ToSlash(filename)) {
				meta["file.mtime"] = mtime.Local().Format("2006-01-02 15:04")
				break
			}
		}
	}

	return meta
}

func parseConditions(clause string) ([]condition, error) {
	parts := strings.Split(clause, " AND ")
	var conds []condition
	re := regexp.MustCompile(`([a-zA-Z0-9_\.]+)\s*(==|!=|>=|<=|>|<|=)\s*["']?([^"']+)["']?`)

	for _, p := range parts {
		m := re.FindStringSubmatch(strings.TrimSpace(p))
		if len(m) < 4 {
			return nil, fmt.Errorf("condição inválida: %s", p)
		}
		conds = append(conds, condition{field: m[1], operator: m[2], value: m[3]})
	}
	return conds, nil
}

func matchAll(meta map[string]interface{}, conds []condition) bool {
	for _, c := range conds {
		val, ok := meta[c.field]
		if !ok {
			return false
		}

		sVal := fmt.Sprintf("%v", val)
		switch c.operator {
		case "==", "=":
			if sVal != c.value {
				return false
			}
		case "!=":
			if sVal == c.value {
				return false
			}
			// Operadores numéricos simplificados podem ser expandidos conforme necessário
		}
	}
	return true
}

func sortResults(rows [][]interface{}, headers []string, field string, order string) {
	colIndex := -1
	for i, h := range headers {
		if h == field {
			colIndex = i
			break
		}
	}
	if colIndex == -1 {
		return
	}

	sort.Slice(rows, func(i, j int) bool {
		valI := fmt.Sprintf("%v", rows[i][colIndex])
		valJ := fmt.Sprintf("%v", rows[j][colIndex])
		if order == "DESC" {
			return valI > valJ
		}
		return valI < valJ
	})
}

func hasTag(tags []string, target string) bool {
	for _, t := range tags {
		if t == target {
			return true
		}
	}
	return false
}

type condition struct {
	field    string
	operator string
	value    string
}
