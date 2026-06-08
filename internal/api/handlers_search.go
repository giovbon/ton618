package api

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"ton618/internal/db"
	"ton618/internal/processor"
	"ton618/internal/search"
	"ton618/internal/template"
	"ton618/internal/watcher"
)

// buildContextSnippet gera um trecho do texto com contexto ao redor de termos encontrados.
// Suporta "frases exatas" entre aspas como termo único.
func buildContextSnippet(query, text string) string {
	const before = 80
	const after = 120

	if text == "" {
		return "..."
	}

	// Extrai frases exatas e termos individuais
	var terms []string
	remaining := query

	// Primeiro: extrai frases entre aspas
	quotedRe := regexp.MustCompile(`"([^"]+)"`)
	for {
		m := quotedRe.FindStringSubmatch(remaining)
		if m == nil {
			break
		}
		phrase := strings.TrimSpace(m[1])
		if phrase == "" {
			phrase = strings.TrimSpace(m[2])
		}
		if len(phrase) > 1 {
			terms = append(terms, phrase)
			// Adiciona palavras individuais como fallback
			for _, pw := range strings.Fields(phrase) {
				if len(pw) > 1 {
					terms = append(terms, pw)
				}
			}
		}
		remaining = strings.Replace(remaining, m[0], " ", 1)
	}

	// Depois: extrai termos individuais do restante
	rawTerms := strings.Fields(remaining)
	for _, t := range rawTerms {
		t = strings.TrimSpace(t)
		if len(t) <= 1 {
			continue
		}
		if t[0] == '-' || t[0] == '#' || strings.HasPrefix(t, "+tags:") {
			continue
		}
		t = strings.Trim(t, `"`)
		if len(t) <= 1 {
			continue
		}
		cleaned := strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return r
			}
			return -1
		}, t)
		if len(cleaned) > 1 {
			terms = append(terms, cleaned)
		}
	}

	if len(terms) == 0 {
		if len(text) > 250 {
			return text[:250] + "..."
		}
		return text
	}

	textLower := strings.ToLower(text)

	// Find first occurrence of each term
	type match struct {
		pos  int
		term string
	}
	var matches []match
	seen := make(map[string]bool)
	for _, term := range terms {
		termLower := strings.ToLower(term)
		if seen[termLower] {
			continue
		}
		seen[termLower] = true
		if pos := strings.Index(textLower, termLower); pos >= 0 {
			matches = append(matches, match{pos: pos, term: termLower})
		}
	}

	if len(matches) == 0 {
		if len(text) > 250 {
			return text[:250] + "..."
		}
		return text
	}

	// Sort by position
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].pos < matches[j].pos
	})

	// Build context windows, merging close ones
	const gapThreshold = 150
	type window struct {
		start, end int
	}
	var windows []window

	for _, m := range matches {
		start := m.pos - before
		if start < 0 {
			start = 0
		}
		end := m.pos + len(m.term) + after
		if end > len(text) {
			end = len(text)
		}

		if len(windows) > 0 {
			last := &windows[len(windows)-1]
			// If this window overlaps or is close enough, merge
			if start <= last.end+gapThreshold {
				if end > last.end {
					last.end = end
				}
				continue
			}
		}
		windows = append(windows, window{start: start, end: end})
	}

	// Build final snippet with ellipsis
	var parts []string
	for i, w := range windows {
		part := text[w.start:w.end]
		// Trim to sentence boundaries at edges when possible
		if w.start > 0 {
			part = "... " + part
		}
		if w.end < len(text) {
			part = part + " ..."
		}

		// If this is not the first window and previous window is far, add separator
		if i > 0 {
			parts = append(parts, "...")
		}
		parts = append(parts, part)
	}

	return strings.Join(parts, " ")
}

// ── Search (HTMX partial) ──

func (ctx *HandlerContext) HandleSearch(w http.ResponseWriter, r *http.Request) {
	// Set request timeout
	rCtx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	query := r.FormValue("q")
	if query == "" && r.Method == "POST" {
		if r.Body != nil {
			body, _ := io.ReadAll(r.Body)
			query = string(body)
			// parse form-encoded or simple string
			if strings.HasPrefix(query, "q=") {
				query = strings.TrimPrefix(query, "q=")
			}
		}
	}

	from, _ := strconv.Atoi(r.FormValue("from"))
	size, _ := strconv.Atoi(r.FormValue("size"))
	if size <= 0 {
		size = 20
	}

	results, err := search.Search(rCtx, ctx.Store, query, from, size,
		ctx.Store.GetLinkCount, ctx.Store.GetPopularity)
	if err != nil {
		slog.Error("search error", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build template data
	var items []template.SearchResultItem
	for _, hit := range results.Hits {
		// Clean snippet: strip HTML, show context around query
		snippet := hit.Doc.Texto
		// Strip basic HTML tags
		snippet = strings.ReplaceAll(snippet, "<p>", "")
		snippet = strings.ReplaceAll(snippet, "</p>", " ")
		snippet = strings.ReplaceAll(snippet, "<br>", " ")
		snippet = strings.ReplaceAll(snippet, "<br/>", " ")
		snippet = strings.ReplaceAll(snippet, "<strong>", "")
		snippet = strings.ReplaceAll(snippet, "</strong>", "")
		snippet = strings.ReplaceAll(snippet, "<em>", "")
		snippet = strings.ReplaceAll(snippet, "</em>", "")
		snippet = strings.ReplaceAll(snippet, "<code>", "")
		snippet = strings.ReplaceAll(snippet, "</code>", "")
		snippet = strings.ReplaceAll(snippet, "<pre>", "")
		snippet = strings.ReplaceAll(snippet, "</pre>", "")
		snippet = strings.ReplaceAll(snippet, "<h1>", "")
		snippet = strings.ReplaceAll(snippet, "</h1>", " - ")
		snippet = strings.ReplaceAll(snippet, "<h2>", "")
		snippet = strings.ReplaceAll(snippet, "</h2>", " - ")
		snippet = strings.ReplaceAll(snippet, "<h3>", "")
		snippet = strings.ReplaceAll(snippet, "</h3>", " - ")
		snippet = strings.ReplaceAll(snippet, "<ul>", "")
		snippet = strings.ReplaceAll(snippet, "</ul>", "")
		snippet = strings.ReplaceAll(snippet, "<li>", "  ")
		snippet = strings.ReplaceAll(snippet, "</li>", "")
		snippet = strings.ReplaceAll(snippet, "<a[^>]*>", "")
		snippet = strings.ReplaceAll(snippet, "</a>", "")
		// Normalize whitespace
		snippet = strings.Join(strings.Fields(snippet), " ")

		// Extract multi-term context windows with ellipsis between distant terms
		snippet = buildContextSnippet(query, snippet)
		tags := db.TagsToSlice(hit.Doc.Tags)
		// Pula PDFs e anexos na busca global (nao fazem sentido como resultado textual)
		if strings.HasPrefix(hit.Doc.Arquivo, "pdfs/") || strings.HasSuffix(strings.ToLower(hit.Doc.Arquivo), ".pdf") {
			continue
		}
		if strings.HasPrefix(hit.Doc.Arquivo, "attachments/") {
			continue
		}
		// Compute line number: find first line in the note that matches the query
		line := findQueryLine(ctx, hit.Doc.Arquivo, query)

		items = append(items, template.SearchResultItem{
			Arquivo:   hit.Doc.Arquivo,
			Secao:     hit.Doc.Secao,
			Tags:      tags,
			Snippet:   snippet,
			Tipo:      hit.Doc.Tipo,
			Timestamp: hit.Doc.Timestamp,
			Line:      line,
		})
	}

	data := template.SearchResultsData{
		Query:   query,
		Results: items,
		Total:   results.Total,
	}

	// HTMX: return only the results partial
	w.Header().Set("Content-Type", "text/html")
	template.SearchResults(data).Render(r.Context(), w)
}

// findQueryLine encontra a primeira linha no conteúdo da nota que contém o termo buscado.
func findQueryLine(ctx *HandlerContext, arquivo, query string) int {
	if query == "" {
		return 0
	}
	content, err := ctx.Store.GetNote(arquivo)
	if err != nil || content == "" {
		return 0
	}

	queryLower := strings.ToLower(query)
	terms := strings.Fields(queryLower)

	lines := strings.Split(content, "\n")
	// Skip frontmatter
	startIdx := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				startIdx = i + 1
				break
			}
		}
	}

	for i := startIdx; i < len(lines); i++ {
		lineLower := strings.ToLower(lines[i])
		// Check if all query terms appear in this line
		allMatch := true
		for _, term := range terms {
			if !strings.Contains(lineLower, term) {
				allMatch = false
				break
			}
		}
		if allMatch {
			return i + 1 // 1-indexed line number
		}
	}

	// Fallback: return the section's approximate position
	return startIdx + 1
}

// ── Bulk Delete (Config → Exclusão) ──
// Aceita tanto filtros (by_age, by_tag) quanto lista explícita (files[]).
func (ctx *HandlerContext) HandleBulkDelete(w http.ResponseWriter, r *http.Request) {
	byAge := r.FormValue("by_age") == "true"
	byTag := r.FormValue("by_tag") == "true"
	ageYears, _ := strconv.Atoi(r.FormValue("age_years"))
	tagNamesRaw := strings.TrimSpace(r.FormValue("tag_name"))
	isPreview := r.FormValue("preview") == "true"

	// Suporta lista explícita de arquivos (enviada pelo frontend com checkboxes)
	explicitFiles := r.Form["files"]

	filesToDelete := make(map[string]bool)
	firstFilter := true

	// Se recebeu lista explícita, usa ela diretamente
	if len(explicitFiles) > 0 {
		for _, f := range explicitFiles {
			f = strings.TrimSpace(f)
			if f != "" {
				filesToDelete[f] = true
			}
		}
		firstFilter = false
	}

	if len(explicitFiles) == 0 && !byAge && !byTag {
		http.Error(w, "pelo menos um filtro ou lista de arquivos deve estar ativo", http.StatusBadRequest)
		return
	}

	// Filter 1: by age
	if byAge {
		if ageYears < 1 || ageYears > 10 {
			http.Error(w, "age_years invalido (1-10)", http.StatusBadRequest)
			return
		}
		cutoff := time.Now().AddDate(-ageYears, 0, 0)
		allNotes, err := ctx.Store.GetAllNotes()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for arquivo, mtimeStr := range allNotes {
			if !isNoteOrPdf(arquivo) {
				continue
			}
			mtime, err := time.Parse(time.RFC3339, mtimeStr)
			if err != nil {
				continue
			}
			if mtime.Before(cutoff) {
				filesToDelete[arquivo] = true
			}
		}
		firstFilter = false
	}

	// Filter 2: by tag(s) — múltiplas tags separadas por vírgula
	if byTag {
		if tagNamesRaw == "" {
			http.Error(w, "tag_name obrigatorio", http.StatusBadRequest)
			return
		}
		tagNames := strings.Split(tagNamesRaw, ",")
		tagSet := make(map[string]bool)
		for _, tn := range tagNames {
			tn = strings.TrimSpace(tn)
			if tn == "" {
				continue
			}
			tagFiles, err := ctx.Store.GetFilesByTag(tn)
			if err != nil {
				continue
			}
			for _, f := range tagFiles {
				if isNoteOrPdf(f) {
					tagSet[f] = true
				}
			}
		}

		if firstFilter {
			filesToDelete = tagSet
		} else {
			for f := range filesToDelete {
				if !tagSet[f] {
					delete(filesToDelete, f)
				}
			}
		}
		firstFilter = false
	}

	// Preview mode: return list of files without deleting
	if isPreview {
		fileList := make([]string, 0, len(filesToDelete))
		for f := range filesToDelete {
			fileList = append(fileList, f)
		}
		sort.Strings(fileList)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"files": fileList,
			"total": len(fileList),
		})
		return
	}

	if len(filesToDelete) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"deleted": 0,
			"message": "nenhuma nota encontrada com os filtros selecionados",
		})
		return
	}

	deleted := 0
	var errors []string
	for arquivo := range filesToDelete {
		isMd := strings.HasSuffix(strings.ToLower(arquivo), ".md")

		if isMd {
			// Note: delete from DB
			ctx.Store.DeleteNote(arquivo)
			// Also remove from disk (backwards compat)
			fullPath := filepath.Join(ctx.Cfg.DocsDir, arquivo)
			os.Remove(fullPath)
		} else {
			// PDF or other: delete from disk
			fullPath := filepath.Join(ctx.Cfg.DocsDir, arquivo)
			if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
				errors = append(errors, arquivo+": "+err.Error())
				continue
			}
		}

		ctx.Store.DeleteDocumentsByFile(arquivo)
		ctx.Store.DeleteFTSByFile(arquivo)
		ctx.Store.DeleteFileMod(arquivo)
		ctx.Store.ResetPopularity(arquivo)
		ctx.Store.SetFileTags(arquivo, nil)
		ctx.Store.ClearLinks(arquivo)

		deleted++
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"deleted": deleted,
		"errors":  errors,
	})
}

// ── Bulk Archive (Config → Arquivamento) ──
// HandleBulkArchive recebe uma lista de arquivos selecionados, cria um ZIP
// em docs/archives/ e remove os arquivos originais + indices do DB.
func (ctx *HandlerContext) HandleBulkArchive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	files := r.Form["files"]
	if len(files) == 0 {
		http.Error(w, "nenhum arquivo selecionado", http.StatusBadRequest)
		return
	}

	// Gera nome legivel pro archive
	archiveName := processor.GenerateCUID2() + ".zip"

	archiveDir := filepath.Join(ctx.Cfg.DocsDir, "archives")
	os.MkdirAll(archiveDir, 0755)
	archivePath := filepath.Join(archiveDir, archiveName)

	// Cria o ZIP com os arquivos selecionados
	zipFile, err := os.Create(archivePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("erro ao criar archive: %v", err), http.StatusInternalServerError)
		return
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	var archivedFiles []string
	var archiveErrors []string

	for _, arquivo := range files {
		arquivo = strings.TrimSpace(arquivo)
		if arquivo == "" {
			continue
		}

		isMd := strings.HasSuffix(strings.ToLower(arquivo), ".md")
		var content []byte

		if isMd {
			// Read note content from DB
			noteContent, err := ctx.Store.GetNote(arquivo)
			if err != nil || noteContent == "" {
				archiveErrors = append(archiveErrors, fmt.Sprintf("%s: nao encontrado no banco", arquivo))
				continue
			}
			content = []byte(noteContent)
			// Remove from DB
			ctx.Store.DeleteNote(arquivo)
		} else {
			// Read from disk (PDFs, etc)
			fullPath := filepath.Join(ctx.Cfg.DocsDir, arquivo)
			data, err := os.ReadFile(fullPath)
			if err != nil {
				if os.IsNotExist(err) {
					archiveErrors = append(archiveErrors, fmt.Sprintf("%s: nao encontrado", arquivo))
				} else {
					archiveErrors = append(archiveErrors, fmt.Sprintf("%s: %v", arquivo, err))
				}
				continue
			}
			content = data
			// Remove from disk
			if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
				archiveErrors = append(archiveErrors, fmt.Sprintf("%s: erro ao remover: %v", arquivo, err))
			}
		}

		// Adiciona ao ZIP preservando o caminho relativo (ex: notes/foo.md, pdfs/bar.pdf)
		f, err := zw.Create(arquivo)
		if err != nil {
			archiveErrors = append(archiveErrors, fmt.Sprintf("%s: erro no zip: %v", arquivo, err))
			continue
		}
		if _, err := f.Write(content); err != nil {
			archiveErrors = append(archiveErrors, fmt.Sprintf("%s: erro ao escrever: %v", arquivo, err))
			continue
		}

		ctx.Store.DeleteDocumentsByFile(arquivo)
		ctx.Store.DeleteFTSByFile(arquivo)
		ctx.Store.DeleteFileMod(arquivo)
		ctx.Store.ResetPopularity(arquivo)
		ctx.Store.SetFileTags(arquivo, nil)
		ctx.Store.ClearLinks(arquivo)

		archivedFiles = append(archivedFiles, arquivo)
	}

	zw.Close()

	// Registra o archive no file_mods (para aparecer na busca compacta)
	// mas NÃO cria documento FTS — archives não têm conteúdo pesquisável.
	ctx.Store.SetFileMod("archives/"+archiveName, time.Now().UTC().Format(time.RFC3339))
	ctx.Store.SetFileTags("archives/"+archiveName, []string{"arquivo"})

	slog.Info("Archive criado", "archive", archiveName, "arquivos", len(archivedFiles))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"archive":  archiveName,
		"archived": len(archivedFiles),
		"errors":   archiveErrors,
	})
}

// ── List Archives ──
// HandleListArchives retorna a lista de archives disponiveis (arquivos ZIP em docs/archives/).
func (ctx *HandlerContext) HandleListArchives(w http.ResponseWriter, r *http.Request) {
	archiveDir := filepath.Join(ctx.Cfg.DocsDir, "archives")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		// Directory might not exist yet
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"archives": []map[string]string{},
		})
		return
	}

	type archiveInfo struct {
		Name      string `json:"name"`
		Size      int64  `json:"size"`
		Modified  string `json:"modified"`
		FileCount int    `json:"file_count"`
	}

	var archives []archiveInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".zip") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Conta quantos arquivos estao no ZIP (le o index do ZIP)
		zipPath := filepath.Join(archiveDir, entry.Name())
		fc := countFilesInZip(zipPath)

		archives = append(archives, archiveInfo{
			Name:      entry.Name(),
			Size:      info.Size(),
			Modified:  info.ModTime().Format(time.RFC3339),
			FileCount: fc,
		})
	}

	// Ordena do mais recente para o mais antigo
	sort.Slice(archives, func(i, j int) bool {
		return archives[i].Modified > archives[j].Modified
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"archives": archives,
	})
}

func countFilesInZip(zipPath string) int {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0
	}
	defer r.Close()
	return len(r.File)
}

// ── Restore Archive ──
// HandleRestoreArchive extrai um ZIP de archives/ de volta para os diretorios originais
// e reindexa todos os arquivos restaurados.
func (ctx *HandlerContext) HandleRestoreArchive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	archiveName := strings.TrimSpace(r.FormValue("archive"))
	if archiveName == "" {
		http.Error(w, "archive name required", http.StatusBadRequest)
		return
	}

	// Seguranca: impede path traversal
	if strings.Contains(archiveName, "..") || strings.Contains(archiveName, "/") {
		http.Error(w, "invalid archive name", http.StatusBadRequest)
		return
	}

	archivePath := filepath.Join(ctx.Cfg.DocsDir, "archives", archiveName)

	// Abre o ZIP
	rZip, err := zip.OpenReader(archivePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("erro ao abrir archive: %v", err), http.StatusInternalServerError)
		return
	}
	defer rZip.Close()

	var restoredFiles []string
	var restoreErrors []string

	for _, f := range rZip.File {
		// Seguranca: impede path traversal dentro do ZIP
		if strings.Contains(f.Name, "..") {
			restoreErrors = append(restoreErrors, fmt.Sprintf("%s: caminho invalido", f.Name))
			continue
		}

		// Lê o conteúdo do arquivo do ZIP
		rc, err := f.Open()
		if err != nil {
			restoreErrors = append(restoreErrors, fmt.Sprintf("%s: erro ao abrir no zip: %v", f.Name, err))
			continue
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			restoreErrors = append(restoreErrors, fmt.Sprintf("%s: erro ao ler: %v", f.Name, err))
			continue
		}

		isMd := strings.HasSuffix(strings.ToLower(f.Name), ".md")

		if isMd {
			// Note: save to DB
			now := time.Now()
			if err := ctx.Store.SaveNote(f.Name, string(data), now.Format(time.RFC3339)); err != nil {
				restoreErrors = append(restoreErrors, fmt.Sprintf("%s: erro ao salvar no banco: %v", f.Name, err))
				continue
			}
		} else {
			// PDFs and others: write to disk
			targetPath := filepath.Join(ctx.Cfg.DocsDir, f.Name)
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				restoreErrors = append(restoreErrors, fmt.Sprintf("%s: erro ao criar diretorio: %v", f.Name, err))
				continue
			}
			if err := os.WriteFile(targetPath, data, 0644); err != nil {
				restoreErrors = append(restoreErrors, fmt.Sprintf("%s: erro ao criar: %v", f.Name, err))
				continue
			}
		}

		restoredFiles = append(restoredFiles, f.Name)
	}

	// Reindexa todos os arquivos restaurados
	for _, arquivo := range restoredFiles {
		isMd := strings.HasSuffix(strings.ToLower(arquivo), ".md")

		if isMd {
			// Note: read from DB and reindex
			content, err := ctx.Store.GetNote(arquivo)
			if err == nil && content != "" {
				now := time.Now()
				if err := ctx.reindexNote(arquivo, content, now); err != nil {
					slog.Error("reindex archive note", "arquivo", arquivo, "error", err)
				}
			}
		} else {
			// PDF/image: reindex from disk
			fullPath := filepath.Join(ctx.Cfg.DocsDir, arquivo)
			info, err := os.Stat(fullPath)
			if err != nil {
				continue
			}
			ev := watcher.FileEvent{
				Path:     fullPath,
				Filename: arquivo,
				ModTime:  info.ModTime(),
				Type:     "create",
			}
			if err := watcher.ProcessFile(ctx.Store, ev); err != nil {
				slog.Error("reindex archive file", "arquivo", arquivo, "error", err)
			}
		}
	}

	// Remove o arquivo ZIP do archive (ja foi restaurado)
	ctx.Store.DeleteDocumentsByFile("archives/" + archiveName)
	ctx.Store.DeleteFTSByFile("archives/" + archiveName)
	ctx.Store.DeleteFileMod("archives/" + archiveName)
	ctx.Store.SetFileTags("archives/"+archiveName, nil)
	ctx.Store.ClearLinks("archives/" + archiveName)
	os.Remove(archivePath)

	slog.Info("Archive restaurado", "archive", archiveName, "arquivos", len(restoredFiles))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"restored": len(restoredFiles),
		"files":    restoredFiles,
		"errors":   restoreErrors,
	})
}
