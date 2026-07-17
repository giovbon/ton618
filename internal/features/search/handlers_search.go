package search

import (
	"ton618/internal/features/notes"

	"archive/zip"
	"context"
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

	"ton618/internal/core/db"
	"ton618/internal/core/domain"
	"ton618/internal/processor"
	"ton618/internal/search"
	
	
	"ton618/internal/watcher"
)

var searchQuotedRe = regexp.MustCompile(`"([^"]+)"|'([^']+)'`)

// extractSearchTerms extrai os termos de busca da query, ignorando filtros e operadores.
func extractSearchTerms(query string) []string {
	var terms []string
	remaining := query

	// 1. Extrai frases entre aspas duplas ou simples
	quotedRe := searchQuotedRe
	for {
		m := quotedRe.FindStringSubmatch(remaining)
		if m == nil {
			break
		}
		phrase := m[1]
		if phrase == "" {
			phrase = m[2]
		}
		phrase = strings.TrimSpace(phrase)
		if len(phrase) > 1 {
			terms = append(terms, phrase)
		}
		remaining = strings.Replace(remaining, m[0], " ", 1)
	}

	// 2. Extrai termos individuais do restante
	rawTerms := strings.Fields(remaining)
	for _, t := range rawTerms {
		t = strings.TrimSpace(t)
		if len(t) <= 1 {
			continue
		}
		// Ignora termos de exclusão (-termo), tags do FTS (+tags:nome) e hashtags nativas (#tag)
		if strings.HasPrefix(t, "-") || strings.HasPrefix(t, "+tags:") || strings.HasPrefix(t, "#") {
			continue
		}
		
		// Remove aspas adicionais se sobrarem nas bordas
		t = strings.Trim(t, `"'`)
		if len(t) <= 1 {
			continue
		}
		
		terms = append(terms, t)
	}
	return terms
}

// buildContextSnippet gera um trecho do texto com contexto ao redor de termos encontrados.
// Suporta "frases exatas" entre aspas como termo único.
func buildContextSnippet(query, text string) string {
	const before = 80
	const after = 120

	if text == "" {
		return "..."
	}

	terms := extractSearchTerms(query)

	if len(terms) == 0 {
		if len(text) > 250 {
			return text[:250] + "..."
		}
		return text
	}

	textLower := strings.ToLower(text)
	textLowerNormalized := removeAccents(textLower)

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
		termLowerNormalized := removeAccents(termLower)
		if pos := strings.Index(textLowerNormalized, termLowerNormalized); pos >= 0 {
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

func removeAccents(s string) string {
	r := strings.NewReplacer(
		"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
		"é", "e", "è", "e", "ê", "e", "ë", "e",
		"í", "i", "ì", "i", "î", "i", "ï", "i",
		"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
		"ú", "u", "ù", "u", "û", "u", "ü", "u",
		"ç", "c",
		"Á", "a", "À", "a", "Â", "a", "Ã", "a", "Ä", "a",
		"É", "e", "È", "e", "Ê", "e", "Ë", "e",
		"Í", "i", "Ì", "i", "Î", "i", "Ï", "i",
		"Ó", "o", "Ò", "o", "Ô", "o", "Õ", "o", "Ö", "o",
		"Ú", "u", "Ù", "u", "Û", "u", "Ü", "u",
		"Ç", "c",
	)
	return r.Replace(s)
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
		ctx.Store.GetBacklinkCount, ctx.Store.GetSynapticWeight)
	if err != nil {
		slog.Error("search error", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build template data
	var items []domain.SearchResultItem
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

		// Fallback: se o buildContextSnippet não encontrou o termo literalmente no texto
		// (match ocorreu só via stem/lematização), usa o snippet do FTS5 como contexto.
		// O snippet do FTS5 já aponta para a região correta do texto, mas usa <b> para marcar.
		// Removemos as tags <b>/<\/b> e deixamos o JS de highlight cuidar da marcação visual.
		if len(hit.Highlight) > 0 {
			ftsSnippets := hit.Highlight["texto"]
			if len(ftsSnippets) > 0 {
				ftsSnip := ftsSnippets[0]
				// Verifica se o snippet atual contém algum dos termos buscados
				terms := extractSearchTerms(query)
				snippetHasMatch := false
				snippetLower := strings.ToLower(snippet)
				snippetNorm := removeAccents(snippetLower)
				for _, term := range terms {
					if strings.Contains(snippetNorm, removeAccents(strings.ToLower(term))) {
						snippetHasMatch = true
						break
					}
				}
				// Se o snippet atual não tem match literal, usa o snippet do FTS5 limpo
				if !snippetHasMatch && ftsSnip != "" {
					// Remove as marcações <b>/<\/b> que o SQLite snippet() inseriu
					ftsSnip = strings.ReplaceAll(ftsSnip, "<b>", "")
					ftsSnip = strings.ReplaceAll(ftsSnip, "</b>", "")
					snippet = ftsSnip
				}
			}
		}
		tags := db.TagsToSlice(hit.Doc.Tags)
		// Filtra tags de tipo de nota para que não apareçam como tags comuns na interface do usuário
		var userTags []string
		for _, t := range tags {
			lowerT := strings.ToLower(t)
			if lowerT != "typst" && lowerT != "drawing" && lowerT != "spreadsheet" && lowerT != "mermaid" && lowerT != "mindmap" && lowerT != "markmap" && lowerT != "map" && lowerT != "mapa" {
				userTags = append(userTags, t)
			}
		}

		// Injeção de tags dinâmicas baseadas no decaimento sináptico
		weight := ctx.Store.GetSynapticWeight(hit.Doc.Arquivo)
		if weight <= 0.105 {
			userTags = append(userTags, "esquecida")
		} else if weight <= 0.25 {
			userTags = append(userTags, "fria")
		}

		noteType := "nota"
		isDrawing := false
		isSpreadsheet := false
		isTypst := false
		isMermaid := false
		isMarkmap := false
		isMap := false
		isYoutube := false
		isArticle := false
		isCapture := false
		for _, t := range tags {
			lowerT := strings.ToLower(t)
			switch lowerT {
			case "drawing":
				isDrawing = true
			case "spreadsheet":
				isSpreadsheet = true
			case "typst":
				isTypst = true
			case "mermaid":
				isMermaid = true
			case "mindmap", "markmap":
				isMarkmap = true
			case "map", "mapa":
				isMap = true
			case "youtube":
				isYoutube = true
			case "artigo", "article":
				isArticle = true
			case "captura", "capture":
				isCapture = true
			}
		}
		lowerFile := strings.ToLower(hit.Doc.Arquivo)
		if !isMarkmap && (strings.Contains(lowerFile, "mindmap") || strings.Contains(lowerFile, "markmap")) {
			isMarkmap = true
		}
		if !isMap && (strings.Contains(lowerFile, "mapa-") || strings.Contains(lowerFile, "mapa.") || strings.HasSuffix(lowerFile, "/map")) {
			isMap = true
		}

		if isDrawing {
			noteType = "desenho"
		} else if isSpreadsheet {
			noteType = "planilha"
		} else if isTypst {
			noteType = "typst"
		} else if isMermaid {
			noteType = "mermaid"
		} else if isMarkmap {
			noteType = "markmap"
		} else if isMap {
			noteType = "mapa"
		} else if isYoutube {
			noteType = "youtube"
		} else if isArticle {
			noteType = "artigo"
		} else if isCapture {
			noteType = "captura"
		} else if strings.HasPrefix(hit.Doc.Arquivo, "pdfs/") {
			noteType = "pdf"
		} else if strings.HasPrefix(hit.Doc.Arquivo, "attachments/") {
			noteType = "anexo"
		} else if strings.HasPrefix(hit.Doc.Arquivo, "archives/") {
			noteType = "arquivo"
		}

		// Pula PDFs e anexos na busca global (nao fazem sentido como resultado textual)
		if strings.HasPrefix(hit.Doc.Arquivo, "pdfs/") || strings.HasSuffix(strings.ToLower(hit.Doc.Arquivo), ".pdf") {
			continue
		}
		if strings.HasPrefix(hit.Doc.Arquivo, "attachments/") {
			continue
		}
		// Compute line number: find first line in the note that matches the query
		line := findQueryLine(ctx, hit.Doc.Arquivo, query)

		displayTime := hit.Doc.Timestamp
		if t, err := time.Parse(time.RFC3339, hit.Doc.Timestamp); err == nil {
			displayTime = t.Local().Format("2006-01-02 15:04:05")
		}

		items = append(items, domain.SearchResultItem{
			Arquivo:   hit.Doc.Arquivo,
			Secao:     hit.Doc.Secao,
			Tags:      userTags,
			RawTags:   tags,
			Snippet:   snippet,
			Tipo:      noteType,
			Timestamp: displayTime,
			Line:      line,
		})
	}

	data := domain.SearchResultsData{
		Query:   query,
		Results: items,
		Total:   results.Total,
	}

	// HTMX: return only the results partial
	w.Header().Set("Content-Type", "text/html")
	SearchResults(data).Render(r.Context(), w)
}

// findQueryLine encontra a primeira linha no conteúdo da nota que contém os termos buscados.
func findQueryLine(ctx *HandlerContext, arquivo, query string) int {
	if query == "" {
		return 0
	}
	content, err := ctx.Store.GetNote(arquivo)
	if err != nil || content == "" {
		return 0
	}

	terms := extractSearchTerms(query)
	if len(terms) == 0 {
		return 0
	}

	// Lowercase and remove accents for matching
	var normalizedTerms []string
	for _, term := range terms {
		normalizedTerms = append(normalizedTerms, removeAccents(strings.ToLower(term)))
	}

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
		lineNormalized := removeAccents(strings.ToLower(lines[i]))
		// Check if all normalized query terms appear in this line
		allMatch := true
		for _, term := range normalizedTerms {
			if !strings.Contains(lineNormalized, term) {
				allMatch = false
				break
			}
		}
		if allMatch {
			return i + 1 // 1-indexed line number
		}
	}

	// Se não encontrar uma única linha com todos os termos, busca a linha com o primeiro termo
	for i := startIdx; i < len(lines); i++ {
		lineNormalized := removeAccents(strings.ToLower(lines[i]))
		if strings.Contains(lineNormalized, normalizedTerms[0]) {
			return i + 1
		}
	}

	// Fallback: retorna a posição aproximada após o frontmatter
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
			if !notes.IsNoteOrPdf(arquivo) {
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
				if notes.IsNoteOrPdf(f) {
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
		notes.ArchivePreview(fileList).Render(r.Context(), w)
		return
	}

	if len(filesToDelete) == 0 {
		notes.ArchiveAlert("Nenhuma nota selecionada para exclusão.", false).Render(r.Context(), w)
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
		ctx.Store.DeleteTodosByFile(arquivo)
		ctx.Store.DeleteFileMod(arquivo)
		ctx.Store.ResetPopularity(arquivo)
		ctx.Store.SetFileTags(arquivo, nil)
		ctx.Store.ClearLinks(arquivo)

		deleted++
	}

	if len(errors) > 0 {
		notes.ArchiveAlert(fmt.Sprintf("%d notas excluídas permanentemente com %d erros.", deleted, len(errors)), false).Render(r.Context(), w)
	} else {
		notes.ArchiveAlert(fmt.Sprintf("%d notas excluídas permanentemente.", deleted), true).Render(r.Context(), w)
	}
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
		ctx.Store.DeleteTodosByFile(arquivo)
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

	slog.Info("Archive criado", "archive", archiveName, "arquivos", len(archivedFiles))

	if len(archiveErrors) > 0 {
		notes.ArchiveAlert(fmt.Sprintf("%d notas arquivadas no pacote %s com %d erros.", len(archivedFiles), archiveName, len(archiveErrors)), false).Render(r.Context(), w)
	} else {
		notes.ArchiveAlert(fmt.Sprintf("%d notas arquivadas com sucesso (%s).", len(archivedFiles), archiveName), true).Render(r.Context(), w)
	}
}

// ── List Archives ──
// HandleListArchives retorna a lista de archives disponiveis (arquivos ZIP em docs/archives/).
func (ctx *HandlerContext) HandleListArchives(w http.ResponseWriter, r *http.Request) {
	archiveDir := filepath.Join(ctx.Cfg.DocsDir, "archives")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		notes.ArchivesList([]domain.ArchiveInfo{}).Render(r.Context(), w)
		return
	}

	var archives []domain.ArchiveInfo
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

		archives = append(archives, domain.ArchiveInfo{
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

	for i := range archives {
		if t, err := time.Parse(time.RFC3339, archives[i].Modified); err == nil {
			archives[i].Modified = t.Format("02/01/2006 15:04")
		}
	}

	notes.ArchivesList(archives).Render(r.Context(), w)
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
			
			if err := ctx.Store.SaveNote(f.Name, string(data), time.Now().Format(time.RFC3339)); err != nil {
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
				
				if false {
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

	if len(restoreErrors) > 0 {
		notes.ArchiveAlert(fmt.Sprintf("%d notas restauradas com %d erros.", len(restoredFiles), len(restoreErrors)), false).Render(r.Context(), w)
	} else {
		notes.ArchiveAlert(fmt.Sprintf("%d notas restauradas com sucesso. Atualize a página para ver na árvore.", len(restoredFiles)), true).Render(r.Context(), w)
	}
}
