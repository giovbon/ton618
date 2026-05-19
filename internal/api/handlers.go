package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"ton618/internal/db"
	"ton618/internal/search"
	"ton618/internal/semantic"
	"ton618/internal/watcher"
)

// ── Helpers de normalizacao ──

// sanitizeFilename garante que o nome do arquivo:
// 1. Nao tenha subdiretorios (strips tudo antes de /)
// 2. Tenha extensao .md
// 3. Esteja no diretorio notes/
func sanitizeFilename(name string) string {
	// Remove qualquer prefixo de diretorio
	base := filepath.Base(name)
	// Garante extensao .md
	if !strings.HasSuffix(base, ".md") {
		base += ".md"
	}
	// Forca prefixo notes/
	if !strings.HasPrefix(base, "notes/") {
		base = "notes/" + base
	}
	return base
}

// displayName retorna apenas o nome do arquivo (sem diretorio e sem .md) para exibicao.
func displayName(name string) string {
	base := filepath.Base(name)
	return strings.TrimSuffix(base, ".md")
}

// buildContextSnippet extrai janelas de contexto ao redor de cada termo da query.
// Se os termos estiverem proximos (dentro de gapThreshold), mostra tudo junto.
// Se estiverem distantes, usa "..." entre as janelas.
func buildContextSnippet(text, query string, before, after int) string {
	// Parse query into individual search terms
	rawTerms := strings.Fields(query)
	var terms []string
	for _, t := range rawTerms {
		t = strings.TrimSpace(t)
		// Skip operators, single chars, quoted phrase boundaries
		if len(t) <= 1 {
			continue
		}
		if t[0] == '-' || t[0] == '#' || strings.HasPrefix(t, "+tags:") {
			continue
		}
		// Strip surrounding quotes for phrase search
		t = strings.Trim(t, `"`)
		if len(t) <= 1 {
			continue
		}
		// Only keep alphabetic and numeric chars for matching
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

// ── Pages ──

func (ctx *HandlerContext) HandleIndex(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":        "TON-618",
		"Query":        r.URL.Query().Get("q"),
		"ContentBlock": "indexContent",
	}
	ctx.render(w, "index.html", data)
}

func (ctx *HandlerContext) HandleEditor(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		filename = "notes/novo.md"
	}

	// Normaliza o filename: garante prefixo notes/ e extensao .md
	sanitized := sanitizeFilename(filename)

	// Se a URL nao estava normalizada, redireciona para a URL canonica
	// Isso evita que o browser fique com uma URL sem o prefixo notes/
	// e o conteudo se perca ao dar refresh.
	if sanitized != filename {
		canonical := "/editor?file=" + url.QueryEscape(sanitized)
		http.Redirect(w, r, canonical, http.StatusFound)
		return
	}

	var content string
	var tags []string

	// So ignora conteudo para o template exato "notes/novo.md"
	// (para evitar que auto-save anterior polua o template de nova nota).
	// Notas com nomes unicos (novo-*) devem carregar seu conteudo normalmente.
	if filename != "notes/novo.md" {
		fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)
		if data, err := os.ReadFile(fullPath); err == nil {
			content = string(data)
		}
	}
	fileTags, err := ctx.Store.GetFileTags(filename)
	if err == nil {
		tags = fileTags
	}

	allTags, err := ctx.Store.GetAllTags()
	if err != nil {
		allTags = nil
	}

	data := map[string]interface{}{
		"Title":        "Editor - " + filename,
		"Filename":     filename,
		"DisplayName":  displayName(filename),
		"Content":      content,
		"Tags":         tags,
		"AllTags":      allTags,
		"LoadTipTap":   true,
		"ContentBlock": "editorContent",
	}
	ctx.render(w, "editor.html", data)
}

func (ctx *HandlerContext) HandleGraph(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":        "Mapa Semântico - TON-618",
		"ContentBlock": "graphContent",
	}
	ctx.render(w, "graph.html", data)
}

// ── Search (HTMX partial) ──

func (ctx *HandlerContext) HandleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.FormValue("q")
	if query == "" && r.Method == "POST" {
		body, _ := io.ReadAll(r.Body)
		query = string(body)
		// parse form-encoded or simple string
		if strings.HasPrefix(query, "q=") {
			query = strings.TrimPrefix(query, "q=")
		}
	}

	from, _ := strconv.Atoi(r.FormValue("from"))
	size, _ := strconv.Atoi(r.FormValue("size"))
	if size <= 0 {
		size = 20
	}

	results, err := search.Search(r.Context(), ctx.Store, query, from, size,
		ctx.Store.GetLinkCount, ctx.Store.GetPopularity)
	if err != nil {
		slog.Error("search error", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Track popularity
	for _, hit := range results.Hits {
		ctx.Store.IncrementPopularity(hit.Doc.Arquivo)
	}

	// Build template data
	type resultItem struct {
		ID         string
		Arquivo    string
		Secao      string
		Tags       []string
		Snippet    string
		Score      float64
		Tipo       string
		Timestamp  string
		IsIndexing bool
	}

	var items []resultItem
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
		snippet = buildContextSnippet(snippet, query, 80, 120)
		tags := db.TagsToSlice(hit.Doc.Tags)
		items = append(items, resultItem{
			ID:         hit.Doc.ID,
			Arquivo:    hit.Doc.Arquivo,
			Secao:      hit.Doc.Secao,
			Tags:       tags,
			Snippet:    snippet,
			Score:      hit.FinalScore,
			Tipo:       hit.Doc.Tipo,
			Timestamp:  hit.Doc.Timestamp,
			IsIndexing: hit.Doc.IsIndexing,
		})
	}

	data := map[string]interface{}{
		"Query":   query,
		"Results": items,
		"Total":   results.Total,
	}

	// HTMX: return only the results partial
	w.Header().Set("Content-Type", "text/html")
	ctx.renderPartial(w, "search_results.html", data)
}

// ── File handlers ──

func (ctx *HandlerContext) HandleFile(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("name")
	if raw == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}

	ext := strings.ToLower(filepath.Ext(raw))
	isPdf := ext == ".pdf"

	if isPdf {
		// PDF pode estar em pdfs/ ou notes/
		basename := filepath.Base(raw)
		subdirs := []string{"pdfs", "notes"}
		for _, sd := range subdirs {
			testPath := filepath.Join(ctx.Cfg.DocsDir, sd, basename)
			if _, err := os.Stat(testPath); err == nil {
				w.Header().Set("Content-Type", "application/pdf")
				w.Header().Set("Content-Disposition", "inline")
				http.ServeFile(w, r, testPath)
				return
			}
		}
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	// Para arquivos .md e imagens, usa o comportamento anterior
	filename := sanitizeFilename(raw)
	fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)
	http.ServeFile(w, r, fullPath)
}

func (ctx *HandlerContext) HandleFileSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raw := r.FormValue("filename")
	content := r.FormValue("content")
	tags := r.FormValue("tags")

	if raw == "" {
		http.Error(w, "filename required", http.StatusBadRequest)
		return
	}

	// Normaliza: remove subdiretorios, garante .md e prefixo notes/
	filename := sanitizeFilename(raw)

	// Garante que o diretorio notes/ existe
	fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write file
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update tags
	if tags != "" {
		tagList := strings.Split(tags, ",")
		var cleanTags []string
		for _, t := range tagList {
			t = strings.TrimSpace(t)
			if t != "" {
				cleanTags = append(cleanTags, t)
			}
		}
		ctx.Store.SetFileTags(filename, cleanTags)
	}

	// Process immediately (sync)
	ev := watcher.FileEvent{
		Path:     fullPath,
		Filename: filename,
		ModTime:  time.Now(),
		Type:     "modify",
	}
	if err := watcher.ProcessFile(ctx.Store, ev, ctx.Embed, ctx.Cfg.EmbeddingAll); err != nil {
		slog.Error("process file after save", "error", err)
	}

	// Redirect back to editor
	http.Redirect(w, r, "/editor?file="+url.QueryEscape(filename), http.StatusSeeOther)
}

func (ctx *HandlerContext) HandleFileDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raw := r.FormValue("filename")
	if raw == "" {
		http.Error(w, "filename required", http.StatusBadRequest)
		return
	}

	ext := strings.ToLower(filepath.Ext(raw))
	var filename string
	var fullPath string

	if ext == ".pdf" {
		// PDF files: search in pdfs/ or notes/
		basename := filepath.Base(raw)
		subdirs := []string{"pdfs", "notes"}
		found := false
		for _, sd := range subdirs {
			testPath := filepath.Join(ctx.Cfg.DocsDir, sd, basename)
			if err := os.Remove(testPath); err == nil {
				filename = sd + "/" + basename
				fullPath = testPath
				found = true
				break
			}
		}
		if !found {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
	} else {
		filename = sanitizeFilename(raw)
		fullPath = filepath.Join(ctx.Cfg.DocsDir, filename)
	}
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Remove from DB
	ctx.Store.DeleteDocumentsByFile(filename)
	ctx.Store.DeleteFTSByFile(filename)
	ctx.Store.DeleteEmbeddingsByFile(filename)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (ctx *HandlerContext) HandleFileRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rawOld := r.FormValue("old")
	rawNew := r.FormValue("new")

	if rawOld == "" || rawNew == "" {
		http.Error(w, "old and new required", http.StatusBadRequest)
		return
	}

	oldName := sanitizeFilename(rawOld)
	newName := sanitizeFilename(rawNew)

	if oldName == newName {
		http.Redirect(w, r, "/editor?file="+newName, http.StatusSeeOther)
		return
	}

	oldPath := filepath.Join(ctx.Cfg.DocsDir, oldName)
	newPath := filepath.Join(ctx.Cfg.DocsDir, newName)

	if err := os.Rename(oldPath, newPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update DB: delete old, re-index new
	ctx.Store.DeleteDocumentsByFile(oldName)
	ctx.Store.DeleteFTSByFile(oldName)
	ctx.Store.DeleteEmbeddingsByFile(oldName)

	info, err := os.Stat(newPath)
	if err == nil {
		watcher.ProcessFile(ctx.Store, watcher.FileEvent{
			Path: newPath, Filename: newName, ModTime: info.ModTime(), Type: "create",
		}, ctx.Embed, ctx.Cfg.EmbeddingAll)
	}

	http.Redirect(w, r, "/editor?file="+url.QueryEscape(newName), http.StatusSeeOther)
}

func (ctx *HandlerContext) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.ParseMultipartForm(10 << 20) // 10MB

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	isPdf := ext == ".pdf"
	isImage := ext == ".png" || ext == ".jpg" || ext == ".jpeg"

	if !isPdf && !isImage {
		http.Error(w, "apenas arquivos PDF ou imagens (.png, .jpg) sao permitidos", http.StatusForbidden)
		return
	}

	var filename string
	if isPdf {
		filename = "pdfs/" + filepath.Base(header.Filename)
		// Garante extensao .pdf
		if !strings.HasSuffix(filename, ".pdf") {
			filename += ".pdf"
		}
	} else {
		// Imagem: salva em notes/ com prefixo img_ para evitar conflito
		timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
		cleanName := strings.ReplaceAll(filepath.Base(header.Filename), " ", "_")
		filename = fmt.Sprintf("notes/img_%s_%s", timestamp, cleanName)
	}

	fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)
	os.MkdirAll(filepath.Dir(fullPath), 0755)

	dst, err := os.Create(fullPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	io.Copy(dst, file)

	// Check if user requested embedding
	wantEmbed := r.FormValue("embed") == "true"

	// Process the file (index, embed)
	info, _ := os.Stat(fullPath)
	watcher.ProcessFile(ctx.Store, watcher.FileEvent{
		Path: fullPath, Filename: filename, ModTime: info.ModTime(), Type: "create",
	}, ctx.Embed, ctx.Cfg.EmbeddingAll || wantEmbed)

	// If embedding was requested but embedAll is false, force-tag the document with "embed"
	// so the embedding persists across reprocessings
	if wantEmbed && !ctx.Cfg.EmbeddingAll && isPdf {
		tags := ctx.Store.GetFileTags(filename)
		// Add "embed" to tags if not already present
		hasEmbed := false
		for _, t := range tags {
			if t == "embed" {
				hasEmbed = true
				break
			}
		}
		if !hasEmbed {
			tags = append(tags, "embed")
			ctx.Store.SetFileTags(filename, tags)
		}
	}

	// Return JSON for editor integration
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":     filename,
		"filename": filepath.Base(filename),
		"url":      "/file?name=" + url.QueryEscape(filename),
	})
}

// ── API ──

func (ctx *HandlerContext) HandleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	embCount := ctx.Store.GetEmbeddingCount()
	fmt.Fprintf(w, `{"status":"ok","documents":%d,"embeddings":%d}`, ctx.Store.GetDocumentCount(), embCount)
}

func (ctx *HandlerContext) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"up","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
}

func (ctx *HandlerContext) HandleGetTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	tags, err := ctx.Store.GetAllTags()
	if err != nil {
		tags = nil
	}
	fmt.Fprintf(w, `{"tags":[`)
	for i, t := range tags {
		if i > 0 {
			w.Write([]byte(","))
		}
		fmt.Fprintf(w, `"%s"`, t)
	}
	w.Write([]byte(`]}`))
}

// scalePoints redimensiona um conjunto de pontos 2D para caber dentro de [-targetRange, targetRange]
// mantendo a proporcao entre os eixos.
func scalePoints(pts map[string]semantic.Point2D, targetRange float64) {
	if len(pts) < 2 {
		return
	}

	// Encontra bounding box
	minX, maxX := math.MaxFloat64, -math.MaxFloat64
	minY, maxY := math.MaxFloat64, -math.MaxFloat64
	for _, p := range pts {
		if p.X < minX {
			minX = p.X
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}

	rangeX := maxX - minX
	rangeY := maxY - minY
	if rangeX < 1e-10 && rangeY < 1e-10 {
		return // todos no mesmo ponto
	}

	// Usa o maior range para escalar (preserva proporcao)
	maxRange := math.Max(rangeX, rangeY)
	scale := (targetRange * 2) / maxRange

	// Centraliza e escala
	midX := (minX + maxX) / 2
	midY := (minY + maxY) / 2

	for id, p := range pts {
		pts[id] = semantic.Point2D{
			X: (p.X - midX) * scale,
			Y: (p.Y - midY) * scale,
		}
	}
}

func (ctx *HandlerContext) HandleGraphData(w http.ResponseWriter, r *http.Request) {
	embeddings, _ := ctx.Store.GetAllEmbeddings()
	links, _ := ctx.Store.GetAllLinks()

	type node struct {
		ID        string  `json:"id"`
		Title     string  `json:"title"`
		X         float64 `json:"x"`
		Y         float64 `json:"y"`
		ClusterID int     `json:"cluster_id"`
	}
	type link struct {
		Source string `json:"source"`
		Target string `json:"target"`
	}

	// Agrupa embeddings por arquivo (um arquivo pode ter varios docs/embeddings)
	// e usa o primeiro embedding encontrado para cada arquivo.
	fileNodes := make(map[string]node) // arquivo -> node
	fileSeen := make(map[string]bool)

	// Coleciona vetores para projecao PCA
	vecsForProjection := make(map[string][]float32)

	for docID, nv := range embeddings {
		doc, _ := ctx.Store.GetDocument(docID)
		if doc == nil || doc.Arquivo == "" {
			continue
		}
		arquivo := doc.Arquivo
		if fileSeen[arquivo] {
			continue // ja processamos este arquivo
		}
		fileSeen[arquivo] = true

		// Nome de exibicao: apenas o nome do arquivo sem extensao
		parts := strings.Split(arquivo, "/")
		baseName := parts[len(parts)-1]
		baseName = strings.TrimSuffix(baseName, ".md")
		baseName = strings.TrimSuffix(baseName, ".pdf")

		// Se ja tem coordenadas 2D armazenadas, usa-as
		x, y := nv.X, nv.Y
		if x == 0 && y == 0 && len(nv.Vector) > 0 {
			// Guarda para recalcular via PCA
			vecsForProjection[arquivo] = nv.Vector
		}

		fileNodes[arquivo] = node{
			ID:    arquivo,
			Title: baseName,
			X:     x,
			Y:     y,
		}
	}

	// Se ha vetores sem projecao 2D, executa PCA
	if len(vecsForProjection) > 1 {
		projected := semantic.Project2DReduce(vecsForProjection)

		// Escala as coordenadas PCA para um range visivel (~[-500, 500])
		scalePoints(projected, 500)

		for arquivo, pt := range projected {
			if n, ok := fileNodes[arquivo]; ok && n.X == 0 && n.Y == 0 {
				n.X = pt.X
				n.Y = pt.Y
				fileNodes[arquivo] = n
			}
			// Armazena no banco para reuso
			docs, _ := ctx.Store.GetDocumentsByFile(arquivo)
			if len(docs) > 0 {
				ctx.Store.SetEmbedding2D(docs[0].ID, pt.X, pt.Y)
			}
		}
	}

	// Clustering com silhouette score no servidor
	var clusterMap map[string]int
	var clusterCount int
	{
		pts := make(map[string]semantic.Point2D)
		for arquivo, n := range fileNodes {
			pts[arquivo] = semantic.Point2D{X: n.X, Y: n.Y}
		}
		clusterMap, clusterCount = semantic.ClusterPoints(pts)
	}

	var nodes []node
	for _, n := range fileNodes {
		// Atribui cluster_id
		clusterID := 0
		if c, ok := clusterMap[n.ID]; ok {
			clusterID = c
		}

		// Se ainda estao em (0,0) apos tentativa de projecao, espalha num grid
		if n.X == 0 && n.Y == 0 {
			idx := len(nodes)
			cols := math.Ceil(math.Sqrt(float64(len(fileNodes))))
			if cols < 3 {
				cols = 3
			}
			n.X = float64(int(idx)%int(cols))*120 + 60
			n.Y = float64(int(idx)/int(cols))*120 + 60
		}

		// Adiciona cluster_id ao nó
		nodes = append(nodes, node{
			ID:        n.ID,
			Title:     n.Title,
			X:         n.X,
			Y:         n.Y,
			ClusterID: clusterID,
		})
	}

	// Filtra links: so inclui se ambos os arquivos existirem como nodes
	var edgeList []link
	for fromFile, toFiles := range links {
		if !fileSeen[fromFile] {
			continue
		}
		for _, toFile := range toFiles {
			if fileSeen[toFile] {
				edgeList = append(edgeList, link{Source: fromFile, Target: toFile})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	result := map[string]interface{}{
		"nodes":   nodes,
		"links":   edgeList,
		"clusters": clusterCount,
	}
	json.NewEncoder(w).Encode(result)
}

func (ctx *HandlerContext) HandleGetAllNotes(w http.ResponseWriter, r *http.Request) {
	mods, err := ctx.Store.GetAllFileMods()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type noteItem struct {
		Arquivo  string   `json:"arquivo"`
		Tags     []string `json:"tags"`
		Mtime    string   `json:"mtime"`
		Embedded bool     `json:"embedded"`
	}

	var notes []noteItem
	for arquivo, mtime := range mods {
		tags, _ := ctx.Store.GetFileTags(arquivo)
		embedded := ctx.Store.HasFileEmbedding(arquivo)
		notes = append(notes, noteItem{
			Arquivo:  arquivo,
			Tags:     tags,
			Mtime:    mtime,
			Embedded: embedded,
		})
	}

	sort.Slice(notes, func(i, j int) bool {
		return notes[i].Mtime > notes[j].Mtime
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"notes": notes,
	})
}

func (ctx *HandlerContext) HandleManualSync(w http.ResponseWriter, r *http.Request) {
	// Trigger poll
	ctx.Watcher.PollAll()
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<div class="text-green-500">✓ Sincronização iniciada</div>`))
}

func (ctx *HandlerContext) HandleGraphProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	embeddings, _ := ctx.Store.GetAllEmbeddings()
	vecs := make(map[string][]float32)
	fileToDoc := make(map[string]string)
	for docID, nv := range embeddings {
		doc, _ := ctx.Store.GetDocument(docID)
		if doc == nil || doc.Arquivo == "" || len(nv.Vector) == 0 {
			continue
		}
		if _, ok := fileToDoc[doc.Arquivo]; ok {
			continue
		}
		fileToDoc[doc.Arquivo] = docID
		vecs[doc.Arquivo] = nv.Vector
	}
	if len(vecs) < 2 {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":true,"nodes":%d}`, len(vecs))
		return
	}
	projected := semantic.Project2DReduce(vecs)
	scalePoints(projected, 500)
	count := 0
	for arquivo, pt := range projected {
		if docID, ok := fileToDoc[arquivo]; ok {
			if err := ctx.Store.SetEmbedding2D(docID, pt.X, pt.Y); err == nil {
				count++
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"ok":true,"nodes":%d,"projected":%d}`, len(vecs), count)
}

func (ctx *HandlerContext) HandleLogin(w http.ResponseWriter, r *http.Request) {
	ctx.renderLogin(w, "login.html", map[string]interface{}{
		"Title": "Login - TON-618",
	})
}
