package system

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"ton618/core/internal/core/domain"
	"ton618/core/internal/features/notes"
	"ton618/core/internal/processor"
	"ton618/core/internal/ui/icons"
)

// ── Tabulator Database Handlers ──

func (ctx *HandlerContext) HandleDatabasePage(w http.ResponseWriter, r *http.Request) {
	notes.Database("Acervo — TON-618").Render(r.Context(), w)
}

func (ctx *HandlerContext) HandleGetDatabaseData(w http.ResponseWriter, r *http.Request) {
	noteList, err := ctx.Notes.GetMany()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	notesContent, err := ctx.Store.GetAllNotesContent()
	if err != nil {
		slog.Error("erro ao pré-carregar conteúdo das notas", "error", err)
		notesContent = make(map[string]string)
	}

	// Metadados externos de arquivos (ex: `pai` de PDF/EPUB/ZIP). Arquivos não
	// têm frontmatter — ver DECISIONS §6.24.
	fileMeta, err := ctx.Store.GetAllFileMetadata()
	if err != nil {
		slog.Warn("erro ao obter metadados de arquivos", "error", err)
		fileMeta = nil
	}

	newCacheEntries := make(map[string]dbCacheEntry)
	var data []map[string]interface{}
	columnSet := make(map[string]bool)

	columnSet["arquivo"] = true
	columnSet["titulo"] = true
	columnSet["mtime"] = true
	columnSet["tags"] = true

	for _, n := range noteList {
		var row map[string]interface{}

		ctx.dbCacheMu.RLock()
		cached, exists := ctx.dbCache[n.Arquivo]
		ctx.dbCacheMu.RUnlock()

		if exists && cached.Mtime == n.Mtime {
			row = make(map[string]interface{})
			for k, v := range cached.Row {
				row[k] = v
			}
		} else {
			content := notesContent[n.Arquivo]
			fm := notes.ParseFrontmatterOrEmpty(content)

			row = make(map[string]interface{})
			row["arquivo"] = n.Arquivo

			displayMtime := n.Mtime
			if t, err := time.Parse(time.RFC3339, n.Mtime); err == nil {
				displayMtime = t.Local().Format("2006-01-02 15:04:05")
			}
			row["mtime"] = displayMtime

			for k, v := range fm {
				lowerK := strings.ToLower(k)
				// `tipo` (campo do frontmatter) é o MESMO dado da coluna "type"
				// (Tipo) — não vira coluna dinâmica duplicada; tags/title/titulo
				// já têm tratamento próprio abaixo.
				if lowerK != "tags" && lowerK != "title" && lowerK != "titulo" && lowerK != "tipo" {
					row[k] = v
				}
			}

			row["titulo"] = displayTitle(n.Arquivo, content, fm)

			if len(n.Tags) > 0 {
				row["tags"] = strings.Join(n.Tags, ", ")
			} else {
				row["tags"] = ""
			}

			// Coluna "Tipo" no vocabulário do campo `tipo:` do frontmatter
			// (PT snake_case: mapa_mental, anexo, ...). Para notas com o campo
			// já gravado o frontmatter é a fonte; para os demais (e para
			// arquivos, que não têm frontmatter) vale a detecção por
			// tags + caminho (n.Type).
			tipoValue := domain.FrontmatterTipo(domain.NoteType(n.Type))
			if fm != nil {
				if v, ok := fm["tipo"]; ok {
					if s := strings.TrimSpace(fmt.Sprint(v)); s != "" {
						tipoValue = domain.NormalizeNoteTipo(s)
					}
				}
			}
			row["type"] = tipoValue
			row["Type"] = tipoValue

			row["_icon"] = icons.SVGString(icons.GetIcon(n.Type), "w-3.5 h-3.5")
			openURL, openBlank := domain.NoteOpenTarget(domain.NoteType(n.Type), n.Arquivo)
			row["_url"] = openURL
			row["_blank"] = openBlank

			newCacheEntries[n.Arquivo] = dbCacheEntry{
				Mtime: n.Mtime,
				Row:   row,
			}
		}

		// Arquivos não têm frontmatter: os metadados da tabela são a ÚNICA fonte
		// das propriedades deles. A linha pode vir do cache (cuja chave é o mtime,
		// que não muda ao editar um metadado nem ao renomear o pai), então além de
		// aplicar os valores atuais é preciso REMOVER as chaves que não existem
		// mais — senão um `pai` antigo ficaria preso na linha cacheada.
		if !isNoteFile(n.Arquivo) {
			for k := range row {
				if strings.HasPrefix(k, "_") || isFileRowBuiltin(k) {
					continue
				}
				if _, ok := fileMeta[n.Arquivo][k]; !ok {
					delete(row, k)
				}
			}
			for k, v := range fileMeta[n.Arquivo] {
				row[k] = v
			}
		}

		normalizeParentKey(row)

		for k := range row {
			if k != "tags" && k != "title" && k != "titulo" && !strings.HasPrefix(k, "_") {
				columnSet[k] = true
			}
		}
		columnSet["type"] = true
		columnSet[parentKey] = true

		data = append(data, row)
	}

	if len(newCacheEntries) > 0 {
		ctx.dbCacheMu.Lock()
		for k, v := range newCacheEntries {
			ctx.dbCache[k] = v
		}
		ctx.dbCacheMu.Unlock()
	}

	data, hasTree := buildNoteTree(data)

	var columns []map[string]interface{}
	columns = append(columns, map[string]interface{}{"title": "Arquivo", "field": "arquivo", "visible": false})
	columns = append(columns, map[string]interface{}{"title": "Título", "field": "titulo", "editor": "input"})
	columns = append(columns, map[string]interface{}{"title": "Tags", "field": "tags", "editor": "input"})
	columns = append(columns, map[string]interface{}{"title": "Tipo", "field": "type", "editor": false, "width": 110})
	// Hierarquia: a coluna "Pai" é onde o usuário edita a nota-mãe (fixa).
	columns = append(columns, map[string]interface{}{"title": "Pai", "field": parentKey, "editor": "input"})

	// Campos dinâmicos (frontmatter / metadados): apenas os que o usuário
	// liberou na aba "Colunas" viram coluna (allowlist salva em `db_columns`).
	for _, col := range ctx.dynamicColumnWhiteList(columnSet) {
		columns = append(columns, map[string]interface{}{
			"title":  fieldTitle(col),
			"field":  col,
			"editor": "input",
		})
	}
	columns = append(columns, map[string]interface{}{"title": "Modificação", "field": "mtime", "editor": false, "visible": false})

	w.Header().Set("Content-Type", "application/json")
	meta := map[string]interface{}{"hasTree": hasTree, "total": len(noteList)}
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		json.NewEncoder(gz).Encode(map[string]interface{}{
			"columns": columns,
			"data":    data,
			"meta":    meta,
		})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"columns": columns,
			"data":    data,
			"meta":    meta,
		})
	}
}

// ── Preferência de colunas (aba "Colunas" nas configurações) ──

// ColumnFieldOption é uma opção da aba "Colunas": um campo do frontmatter que
// o usuário pode marcar para virar (ou não) coluna no Tabulator.
type ColumnFieldOption struct {
	Field   string
	Title   string
	Checked bool
}

// readAllowedColumns devolve (a) o conjunto de campos que o usuário permitiu
// virar coluna e (b) se a preferência já foi salva. Distingue "nunca
// configurado" (cfg=false ⇒ todos habilitados, comportamento padrão) de
// "desmarquei tudo" (cfg=true com allowed vazio ⇒ nenhum campo liberado). O
// marker "-" representa justamente a allowlist vazia explícita, porque um valor
// vazio em `settings` é indistinguível de "não salvo" (GetSetting devolve "").
func (ctx *HandlerContext) readAllowedColumns() (allowed map[string]bool, cfg bool) {
	allowed = make(map[string]bool)
	raw, _ := ctx.Store.GetSetting("db_columns")
	if raw == "" {
		return allowed, false
	}
	if raw == "-" {
		return allowed, true
	}
	for _, f := range strings.Split(raw, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			allowed[f] = true
		}
	}
	return allowed, true
}

// isDynamicColumnField decide se um campo é dinâmico — isto é, controlável
// pela aba "Colunas". Campos embutidos/fixos (arquivo, titulo, tags, type,
// mtime, pai) e campos de configuração interna (markmap) ficam de fora: nunca
// são colunas dinâmicas, por construção. `tipo` (campo do frontmatter) é o
// mesmo dado da coluna fixa "type" (Tipo) — também fica de fora, senão viraria
// uma coluna duplicada.
func isDynamicColumnField(field string) bool {
	switch strings.ToLower(field) {
	case "arquivo", "titulo", "tags", "mtime", "type", "tipo", "pai", "parent", "markmap":
		return false
	}
	return true
}

// dynamicColumnWhiteList filtra um conjunto de campos candidatos, respeitando
// a allowlist salva (`readAllowedColumns`; vazia = inclui todos). Ordena a
// saída para o payload ser determinístico.
func (ctx *HandlerContext) dynamicColumnWhiteList(candidates map[string]bool) []string {
	allowed, cfg := ctx.readAllowedColumns()
	var out []string
	for col := range candidates {
		if !isDynamicColumnField(col) {
			continue
		}
		// Sem preferência salva (cfg=false) => todos os campos viram coluna.
		// Com preferência salva => só os liberados (allowed), mesmo que vazio.
		if !cfg || allowed[col] {
			out = append(out, col)
		}
	}
	sort.Strings(out)
	return out
}

// fieldTitle formata o rótulo de uma coluna dinâmica (primeira letra maiúscula).
func fieldTitle(field string) string {
	if field == "" {
		return field
	}
	return strings.ToUpper(field[:1]) + field[1:]
}

// collectColumnCandidates devolve a lista ordenada de campos dinâmicos
// disponíveis em alguma nota (frontmatter) ou arquivo (metadados externos).
// É a fonte da verdade da aba "Colunas".
func (ctx *HandlerContext) collectColumnCandidates(noteList []domain.NoteItem) ([]string, error) {
	notesContent, err := ctx.Store.GetAllNotesContent()
	if err != nil {
		notesContent = make(map[string]string)
	}
	fileMeta, err := ctx.Store.GetAllFileMetadata()
	if err != nil {
		fileMeta = nil
	}

	set := make(map[string]bool)
	for _, n := range noteList {
		content := notesContent[n.Arquivo]
		fm := notes.ParseFrontmatterOrEmpty(content)
		for k := range fm {
			if isDynamicColumnField(k) {
				set[k] = true
			}
		}
		// Arquivos (PDF/EPUB/ZIP) não têm frontmatter: os metadados da tabela
		// são a única fonte das propriedades (ver DECISIONS §6.24).
		if !isNoteFile(n.Arquivo) {
			for k := range fileMeta[n.Arquivo] {
				if isDynamicColumnField(k) {
					set[k] = true
				}
			}
		}
	}

	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

// columnFieldOptions monta a lista de opções da aba "Colunas" com o estado de
// marcação atual (vazio na allowlist = todos marcados).
func (ctx *HandlerContext) columnFieldOptions() ([]ColumnFieldOption, error) {
	noteList, err := ctx.Notes.GetMany()
	if err != nil {
		return nil, err
	}
	candidates, err := ctx.collectColumnCandidates(noteList)
	if err != nil {
		return nil, err
	}
	allowed, cfg := ctx.readAllowedColumns()
	opts := make([]ColumnFieldOption, 0, len(candidates))
	for _, f := range candidates {
		// Sem preferência salva => todos marcados; com preferência => só os liberados.
		opts = append(opts, ColumnFieldOption{
			Field:   f,
			Title:   fieldTitle(f),
			Checked: !cfg || allowed[f],
		})
	}
	return opts, nil
}

// ── Título exibido na coluna "Título" do Tabulator ──

// displayTitle escolhe o rótulo da coluna Título. O padrão continua sendo o
// nome do arquivo (domain.DisplayName) — é a SSOT da exibição. Mas um nome que
// não tem NENHUM caractere alfanumérico (ex: "...", "…", "- - -") não comunica
// nada e é justamente o sintoma de um rename acidental; nesse caso caímos para
// o título real da nota, na ordem:
//
//  1. primeiro heading markdown (`# Título`) — processor.ExtractTitle;
//  2. `titulo:` / `title:` do frontmatter.
//
// Arquivos (PDF/EPUB/ZIP) não têm conteúdo nem frontmatter: neles a função
// continua devolvendo o nome do arquivo (não há outra fonte).
func displayTitle(arquivo, content string, fm map[string]interface{}) string {
	name := domain.DisplayName(arquivo)
	if isMeaningfulName(name) {
		return name
	}
	if h := strings.TrimSpace(processor.ExtractTitle(content, arquivo)); isMeaningfulName(h) {
		return h
	}
	if t := notes.FrontmatterTitle(fm); isMeaningfulName(t) {
		return t
	}
	return name
}

// isMeaningfulName informa se o texto tem ao menos uma letra ou dígito. Nomes
// só de pontuação/espaço (ex: "...") não são considerados um título útil.
func isMeaningfulName(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

type UpdatePropertyRequest struct {
	File  string      `json:"file"`
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

func (ctx *HandlerContext) HandleUpdateNoteProperty(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeUpdateProperty(w, r)
	if !ok {
		return
	}

	if req.Key == "titulo" {
		ctx.handleTitleRename(w, req)
		return
	}

	// Arquivos (PDF/EPUB/ZIP/...): não são notas. Só `tags` e metadados
	// (`pai`) se aplicam — e o `pai` vive na tabela file_metadata (DECISIONS
	// §6.24), não no frontmatter (que não existe).
	if !isNoteFile(req.File) {
		ctx.handleNonNoteProperty(w, req)
		return
	}

	ctx.handleNoteProperty(w, req)
}

func decodeUpdateProperty(w http.ResponseWriter, r *http.Request) (UpdatePropertyRequest, bool) {
	var req UpdatePropertyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return req, false
	}
	if req.File == "" || req.Key == "" {
		http.Error(w, "file and key are required", http.StatusBadRequest)
		return req, false
	}
	return req, true
}

// handleTitleRename trata `titulo`: renomeia a nota (ou o arquivo) e invalida
// as entradas correspondentes do cache da tabela.
func (ctx *HandlerContext) handleTitleRename(w http.ResponseWriter, req UpdatePropertyRequest) {
	var newValStr string
	switch v := req.Value.(type) {
	case string:
		newValStr = v
	case float64:
		newValStr = fmt.Sprintf("%.0f", v)
	case int:
		newValStr = strconv.Itoa(v)
	default:
		if req.Value != nil {
			newValStr = fmt.Sprintf("%v", req.Value)
		}
	}
	newValStr = strings.TrimSpace(newValStr)
	if newValStr == "" {
		http.Error(w, "invalid title value", http.StatusBadRequest)
		return
	}
	// Um título sem nenhuma letra/dígito (ex: "...") vira um nome de arquivo
	// inútil e some da leitura da tabela — recusa na origem (ver displayTitle).
	if !isMeaningfulName(newValStr) {
		http.Error(w, "o título precisa conter ao menos uma letra ou número", http.StatusBadRequest)
		return
	}

	rawOld := req.File
	rawNew := newValStr

	ext := strings.ToLower(filepath.Ext(rawOld))
	isNote := ext == ".md" || strings.HasPrefix(rawOld, domain.NotePrefix) || !domain.HasNonNoteFilePrefix(rawOld)

	var oldName, newName string

	if !isNote {
		basename := filepath.Base(rawOld)
		newBasename := filepath.Base(rawNew)
		if ext != "" && !strings.HasSuffix(strings.ToLower(newBasename), ext) {
			newBasename += ext
		}

		dir := filepath.Dir(rawOld)
		if dir == "." || dir == "" {
			if ext == ".pdf" {
				dir = domain.PDFDir
			} else if ext == ".epub" {
				dir = domain.EPUBDir
			} else if ext == ".zip" || ext == ".rar" {
				dir = domain.AttachmentDir
			} else {
				dir = domain.NotesDir
			}
		}

		oldName = dir + "/" + basename
		newName = dir + "/" + newBasename
		oldPath := filepath.Join(ctx.Cfg.DocsDir, oldName)

		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			found := false
			for _, sd := range domain.SearchDirs {
				testPath := filepath.Join(ctx.Cfg.DocsDir, sd, basename)
				if _, err := os.Stat(testPath); err == nil {
					oldName = sd + "/" + basename
					newName = sd + "/" + newBasename
					oldPath = testPath
					found = true
					break
				}
			}
			if !found {
				http.Error(w, "arquivo não encontrado", http.StatusNotFound)
				return
			}
		}

		if err := notes.RenameNonNoteFile(ctx.Store, ctx.Notes, ctx.Cfg.DocsDir, oldName, newName); err != nil {
			notes.WriteRenameError(w, err)
			return
		}
	} else {
		if err := ctx.Notes.Rename(rawOld, rawNew); err != nil {
			notes.WriteRenameError(w, err)
			return
		}
		newName = notes.NoteFilename(rawNew)
		oldName = notes.NoteFilename(rawOld)
	}

	ctx.invalidateDBCache(req.File, oldName, newName)
	w.WriteHeader(http.StatusOK)
}

// handleNonNoteProperty trata tags e `pai` de arquivos (PDF/EPUB/ZIP/...).
func (ctx *HandlerContext) handleNonNoteProperty(w http.ResponseWriter, req UpdatePropertyRequest) {
	switch {
	case strings.EqualFold(req.Key, "tags"):
		rawVal, _ := req.Value.(string)
		var tagList []string
		for _, t := range strings.Split(rawVal, ",") {
			t = strings.TrimSpace(t)
			t = strings.TrimPrefix(t, "#")
			if t != "" {
				tagList = append(tagList, t)
			}
		}
		if err := ctx.Store.SetFileTags(req.File, tagList); err != nil {
			http.Error(w, "error updating tags", http.StatusInternalServerError)
			return
		}
	case isParentKey(req.Key):
		ref := parentRefFromMap(map[string]interface{}{parentKey: req.Value})
		if strings.Contains(ref, ",") {
			http.Error(w, "um arquivo só pode ter UM pai — remova a vírgula do campo 'Pai'", http.StatusBadRequest)
			return
		}
		// Arquivos são apenas FILHAS: valida que o pai é uma nota e que não
		// há ciclo (o alvo é resolvido só entre notas — ver §6.24).
		if err := ctx.validateParentAssignment(req.File, ref); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := ctx.Notes.SetFileParent(req.File, ref); err != nil {
			http.Error(w, "error saving file parent", http.StatusInternalServerError)
			return
		}
	}

	ctx.invalidateDBCache(req.File)
	w.WriteHeader(http.StatusOK)
}

// handleNoteProperty grava uma propriedade no frontmatter da nota.
func (ctx *HandlerContext) handleNoteProperty(w http.ResponseWriter, req UpdatePropertyRequest) {
	removeLegacyParentKey := false
	if isParentKey(req.Key) {
		ref := parentRefFromMap(map[string]interface{}{parentKey: req.Value})
		// Uma nota tem NO MÁXIMO 1 pai (a hierarquia é uma árvore, não um grafo
		// — ver DECISIONS 6.17/6.19). Uma vírgula no valor normalmente significa
		// que o usuário tentou declarar mais de um pai; sem esta guarda o valor
		// `a, b` viraria um nome inexistente e a nota cairia na raiz em silêncio.
		// O caso YAML de lista (`[a, b]`, `[[a, b]]`) é capturado pelo mesmo
		// padrão após a normalização.
		if strings.Contains(ref, ",") {
			http.Error(w, "uma nota só pode ter UM pai — remova a vírgula do campo 'Pai'", http.StatusBadRequest)
			return
		}
		if err := ctx.validateParentAssignment(req.File, ref); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		req.Key = parentKey
		if ref == "" {
			req.Value = ""
		} else {
			req.Value = ref
		}
		removeLegacyParentKey = true
	}

	content, err := ctx.Store.GetNote(req.File)
	if err != nil {
		http.Error(w, "note not found", http.StatusNotFound)
		return
	}

	newContent, err := notes.UpdateFrontmatterProperty(content, req.Key, req.Value)
	if err != nil {
		http.Error(w, "error updating frontmatter", http.StatusInternalServerError)
		return
	}

	if removeLegacyParentKey {
		newContent, err = notes.UpdateFrontmatterProperty(newContent, parentKeyLegacy, "")
		if err != nil {
			http.Error(w, "error updating frontmatter", http.StatusInternalServerError)
			return
		}
	}

	if err := ctx.Notes.Save(req.File, newContent, nil); err != nil {
		http.Error(w, "error saving note", http.StatusInternalServerError)
		return
	}

	ctx.invalidateDBCache(req.File)
	w.WriteHeader(http.StatusOK)
}

// invalidateDBCache remove as entradas do cache da tabela. Chaves vazias são
// ignoradas naturalmente (delete de chave inexistente é no-op).
func (ctx *HandlerContext) invalidateDBCache(keys ...string) {
	ctx.dbCacheMu.Lock()
	for _, k := range keys {
		delete(ctx.dbCache, k)
	}
	ctx.dbCacheMu.Unlock()
}
