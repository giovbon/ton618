package system

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ton618/core/internal/core/domain"
	"ton618/core/internal/features/notes"
	"ton618/core/internal/ui/icons"
	"ton618/core/internal/watcher"
)

// ── Tabulator Database Handlers ──

func (ctx *HandlerContext) HandleDatabasePage(w http.ResponseWriter, r *http.Request) {
	notes.Database("Tabulator — TON-618").Render(r.Context(), w)
}

func (ctx *HandlerContext) HandleGetDatabaseData(w http.ResponseWriter, r *http.Request) {
	noteList, err := ctx.Notes.GetMany()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	embeddedFiles, err := ctx.Store.GetEmbeddedFiles()
	if err != nil {
		slog.Warn("erro ao obter arquivos com embedding", "error", err)
		embeddedFiles = make(map[string]bool)
	}

	notesContent, err := ctx.Store.GetAllNotesContent()
	if err != nil {
		slog.Error("erro ao pré-carregar conteúdo das notas", "error", err)
		notesContent = make(map[string]string)
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
			fm, _, err := notes.ParseFrontmatter(content)
			if err != nil {
				fm = make(map[string]interface{})
			}

			row = make(map[string]interface{})
			row["arquivo"] = n.Arquivo

			displayMtime := n.Mtime
			if t, err := time.Parse(time.RFC3339, n.Mtime); err == nil {
				displayMtime = t.Local().Format("2006-01-02 15:04:05")
			}
			row["mtime"] = displayMtime

			for k, v := range fm {
				lowerK := strings.ToLower(k)
				if lowerK != "tags" && lowerK != "title" && lowerK != "titulo" {
					row[k] = v
				}
			}

			row["titulo"] = domain.DisplayName(n.Arquivo)

			if len(n.Tags) > 0 {
				row["tags"] = strings.Join(n.Tags, ", ")
			} else {
				row["tags"] = ""
			}

			row["type"] = n.Type
			row["Type"] = n.Type

			row["_icon"] = icons.SVGString(icons.GetIcon(n.Type), "w-3.5 h-3.5")
			openURL, openBlank := domain.NoteOpenTarget(domain.NoteType(n.Type), n.Arquivo)
			row["_url"] = openURL
			row["_blank"] = openBlank

			newCacheEntries[n.Arquivo] = dbCacheEntry{
				Mtime: n.Mtime,
				Row:   row,
			}
		}

		row["embeded"] = embeddedFiles[n.Arquivo]

		normalizeParentKey(row)

		for k := range row {
			if k != "tags" && k != "title" && k != "titulo" && k != "embeded" && !strings.HasPrefix(k, "_") {
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
	columns = append(columns, map[string]interface{}{"title": "Abrir", "field": "abrir_link", "visible": false, "headerSort": false, "width": 80, "hozAlign": "center"})
	columns = append(columns, map[string]interface{}{"title": "Arquivo", "field": "arquivo", "visible": false})
	columns = append(columns, map[string]interface{}{"title": "Título", "field": "titulo", "editor": "input"})
	columns = append(columns, map[string]interface{}{"title": "Tags", "field": "tags", "editor": "input"})
	columns = append(columns, map[string]interface{}{"title": "Tipo", "field": "type", "editor": false, "width": 110})
	columns = append(columns, map[string]interface{}{"title": "Embeded", "field": "embeded", "editor": false, "width": 110, "hozAlign": "center"})

	for col := range columnSet {
		lowerCol := strings.ToLower(col)
		if lowerCol != "arquivo" && lowerCol != "titulo" && lowerCol != "tags" && lowerCol != "mtime" && lowerCol != "type" {
			title := strings.ToUpper(col[:1]) + col[1:]
			columns = append(columns, map[string]interface{}{
				"title":  title,
				"field":  col,
				"editor": "input",
			})
		}
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

type UpdatePropertyRequest struct {
	File  string      `json:"file"`
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

func (ctx *HandlerContext) HandleUpdateNoteProperty(w http.ResponseWriter, r *http.Request) {
	var req UpdatePropertyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.File == "" || req.Key == "" {
		http.Error(w, "file and key are required", http.StatusBadRequest)
		return
	}

	if req.Key == "titulo" {
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

		rawOld := req.File
		rawNew := newValStr

		ext := strings.ToLower(filepath.Ext(rawOld))
		isNote := ext == ".md" || strings.HasPrefix(rawOld, "notes/") || (!strings.HasPrefix(rawOld, "pdfs/") && !strings.HasPrefix(rawOld, "attachments/") && !strings.HasPrefix(rawOld, "archives/") && !strings.HasPrefix(rawOld, "epubs/"))

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
					dir = "pdfs"
				} else if ext == ".epub" {
					dir = "epubs"
				} else if ext == ".zip" || ext == ".rar" {
					dir = "attachments"
				} else {
					dir = "notes"
				}
			}

			oldName = dir + "/" + basename
			newName = dir + "/" + newBasename
			oldPath := filepath.Join(ctx.Cfg.DocsDir, oldName)
			newPath := filepath.Join(ctx.Cfg.DocsDir, newName)

			if _, err := os.Stat(oldPath); os.IsNotExist(err) {
				found := false
				for _, sd := range []string{"pdfs", "epubs", "attachments", "archives", "notes"} {
					testPath := filepath.Join(ctx.Cfg.DocsDir, sd, basename)
					if _, err := os.Stat(testPath); err == nil {
						oldName = sd + "/" + basename
						newName = sd + "/" + newBasename
						oldPath = testPath
						newPath = filepath.Join(ctx.Cfg.DocsDir, newName)
						found = true
						break
					}
				}
				if !found {
					http.Error(w, "arquivo não encontrado", http.StatusNotFound)
					return
				}
			}

			if err := os.Rename(oldPath, newPath); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if err := ctx.Store.DeleteAllFileRecords(oldName); err != nil {
				slog.Error("delete old file records on rename", "file", oldName, "error", err)
			}

			info, err := os.Stat(newPath)
			if err == nil {
				watcher.ProcessFile(ctx.Store, watcher.FileEvent{
					Path: newPath, Filename: newName, ModTime: info.ModTime(), Type: "create",
				})
			}
			if ctx.Notes != nil {
				if err := ctx.Notes.UpdateBacklinksOnRename(oldName, newName); err != nil {
					slog.Error("update backlinks on non-note property rename", "old", oldName, "new", newName, "error", err)
				}
			}
		} else {
			if err := ctx.Notes.Rename(rawOld, rawNew); err != nil {
				msg := err.Error()
				if strings.Contains(msg, "já existe uma nota") || strings.Contains(msg, "nota não encontrada") || strings.Contains(msg, "UNIQUE constraint failed") {
					http.Error(w, msg, http.StatusBadRequest)
				} else {
					http.Error(w, msg, http.StatusInternalServerError)
				}
				return
			}
			newName = notes.NoteFilename(rawNew)
			oldName = notes.NoteFilename(rawOld)
		}

		ctx.dbCacheMu.Lock()
		delete(ctx.dbCache, req.File)
		delete(ctx.dbCache, oldName)
		if newName != "" {
			delete(ctx.dbCache, newName)
		}
		ctx.dbCacheMu.Unlock()

		w.WriteHeader(http.StatusOK)
		return
	}

	ext := strings.ToLower(filepath.Ext(req.File))
	if ext == ".zip" || ext == ".pdf" || ext == ".epub" {
		if req.Key == "tags" {
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
		}

		ctx.dbCacheMu.Lock()
		delete(ctx.dbCache, req.File)
		ctx.dbCacheMu.Unlock()

		w.WriteHeader(http.StatusOK)
		return
	}

	removeLegacyParentKey := false
	if isParentKey(req.Key) {
		ref := parentRefFromMap(map[string]interface{}{parentKey: req.Value})
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

	ctx.dbCacheMu.Lock()
	delete(ctx.dbCache, req.File)
	ctx.dbCacheMu.Unlock()

	w.WriteHeader(http.StatusOK)
}
