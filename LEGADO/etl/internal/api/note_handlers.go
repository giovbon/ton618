package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// HandleGetNotes retorna a lista de todos os arquivos .md (apenas o nome base) para autocomplete de WikiLinks
func (ctx *HandlerContext) HandleGetNotes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != http.MethodGet {
		http.Error(w, "Método não suportado", http.StatusMethodNotAllowed)
		return
	}

	notes := make(map[string]bool)
	subDirs := []string{"notes", "links"}

	for _, sub := range subDirs {
		dirPath := filepath.Join(ctx.Cfg.DocsDir, sub)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
				name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
				notes[name] = true
			}
		}
	}

	var noteList []string
	for n := range notes {
		noteList = append(noteList, n)
	}
	sort.Strings(noteList)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"notes": noteList,
		"total": len(noteList),
	})
}
