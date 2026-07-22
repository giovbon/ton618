package search

import (
	"ton618/core/internal/features/notes"

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

	"ton618/core/internal/core/db"
	"ton618/core/internal/core/domain"
	"ton618/core/internal/processor"
	"ton618/core/internal/search"
	
	
	"ton618/core/internal/watcher"
)

var searchQuotedRe = regexp.MustCompile(`"([^"]+)"|'([^']+)'`)

func cleanTermForMatching(t string) string {
	t = strings.TrimSpace(t)
	// Remove leading operators like +, -, ~, #
	t = strings.TrimLeft(t, "+-~#")
	// Remove trailing operators like *, ~
	t = strings.TrimRight(t, "*~")
	// Trim quotes
	t = strings.Trim(t, `"'`)
	return strings.TrimSpace(t)
}

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
		
		cleaned := cleanTermForMatching(t)
		if len(cleaned) <= 1 {
			continue
		}
		
		terms = append(terms, cleaned)
	}
	return terms
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
	seenFiles := make(map[string]bool)
	weightCache := make(map[string]float64) // cache para GetSynapticWeight
	var items []domain.SearchResultItem
	for _, hit := range results.Hits {
		// Pula PDFs e anexos na busca global (não fazem sentido como resultado textual)
		if strings.HasPrefix(hit.Doc.Arquivo, "pdfs/") || strings.HasSuffix(strings.ToLower(hit.Doc.Arquivo), ".pdf") {
			continue
		}
		if strings.HasPrefix(hit.Doc.Arquivo, "attachments/") {
			continue
		}

		// Deduplica por arquivo de nota
		if seenFiles[hit.Doc.Arquivo] {
			continue
		}
		seenFiles[hit.Doc.Arquivo] = true

		// Usa o snippet do FTS5 diretamente (já contém contexto ao redor dos termos)
		// Remove as marcações <b> do SQLite snippet(); o JS de highlight cuida da marcação visual
		snippet := ""
		if len(hit.Highlight) > 0 {
			if ftsSnippets, ok := hit.Highlight["texto"]; ok && len(ftsSnippets) > 0 {
				snippet = strings.ReplaceAll(ftsSnippets[0], "<b>", "")
				snippet = strings.ReplaceAll(snippet, "</b>", "")
			}
		}
		if snippet == "" {
			// Fallback: primeiros 300 caracteres do texto
			text := hit.Doc.Texto
			if len(text) > 300 {
				text = text[:300] + "..."
			}
			snippet = text
		}
		// Normaliza espaços em branco
		snippet = strings.Join(strings.Fields(snippet), " ")

		tags := db.TagsToSlice(hit.Doc.Tags)
		// Filtra tags de tipo de nota
		var userTags []string
		for _, t := range tags {
			lowerT := strings.ToLower(t)
			if lowerT != "typst" && lowerT != "drawing" && lowerT != "spreadsheet" && lowerT != "mermaid" && lowerT != "mindmap" && lowerT != "markmap" && lowerT != "map" && lowerT != "mapa" {
				userTags = append(userTags, t)
			}
		}

		// Injeção de tags dinâmicas baseadas no decaimento sináptico
		weight, ok := weightCache[hit.Doc.Arquivo]
		if !ok {
			weight = ctx.Store.GetSynapticWeight(hit.Doc.Arquivo)
			weightCache[hit.Doc.Arquivo] = weight
		}
		if weight <= 0.105 {
			userTags = append(userTags, "esquecida")
		} else if weight <= 0.25 {
			userTags = append(userTags, "fria")
		}

		noteType := string(domain.DetectNoteType(tags, hit.Doc.Texto, hit.Doc.Arquivo))

		// Compute line number from the already-loaded text (sem DB call)
		line := findQueryLineInText(hit.Doc.Texto, query)

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

	total := results.Total
	if len(items) < total {
		total = len(items)
	}

	data := domain.SearchResultsData{
		Query:   query,
		Results: items,
		Total:   total,
	}

	// HTMX: return only the results partial
	w.Header().Set("Content-Type", "text/html")
	SearchResults(data).Render(r.Context(), w)
}

// findQueryLineInText encontra a primeira linha no texto que contém os termos buscados.
// Usa o texto já carregado em memória, sem consultar o banco.
func findQueryLineInText(text, query string) int {
	if query == "" || text == "" {
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

	lines := strings.Split(text, "\n")
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
