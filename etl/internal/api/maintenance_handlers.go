package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"etl/internal/ingest"
	"etl/internal/search"
	"etl/internal/utils"
)

type StaleFile struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
	AgeDays int       `json:"ageDays"`
	Reason  string    `json:"reason"`
}

type MaintenanceResult struct {
	Files      []StaleFile `json:"files"`
	TotalSize  int64       `json:"totalSize"`
	TotalCount int         `json:"totalCount"`
}

// noteAnalysis holds the result of analysing a single note file.
type noteAnalysis struct {
	content   string // file content (read once)
	isZombie  bool
	abandoned bool // has ≥3 unchecked tasks
	isCapture bool // YouTube or Article capture
}

// analyseNote reads the file once and classifies it.
func analyseNote(path string, size int64) noteAnalysis {
	var a noteAnalysis
	raw, err := os.ReadFile(path)
	if err != nil {
		return a
	}
	a.content = string(raw)
	text := strings.TrimSpace(a.content)

	// Zombie detection: empty, only H1 title, or only Frontmatter (+ optional H1)
	if size == 0 || text == "" {
		a.isZombie = true
	} else if strings.HasPrefix(text, "# ") {
		parts := strings.SplitN(text, "\n", 2)
		if len(parts) == 1 || strings.TrimSpace(parts[1]) == "" {
			a.isZombie = true
		}
	} else if strings.HasPrefix(text, "---") {
		parts := strings.SplitN(text, "---", 3)
		if len(parts) == 3 {
			remainder := strings.TrimSpace(parts[2])
			if remainder == "" {
				a.isZombie = true
			} else if strings.HasPrefix(remainder, "# ") {
				lines := strings.SplitN(remainder, "\n", 2)
				if len(lines) == 1 || strings.TrimSpace(lines[1]) == "" {
					a.isZombie = true
				}
			}
		}
	}

	// Abandoned: ≥3 open checkboxes
	a.abandoned = strings.Count(a.content, "- [ ]") >= 3

	// Capture detection: tags, specific URL patterns, or "Fonte:" prefix
	cLower := strings.ToLower(a.content)

	a.isCapture = strings.Contains(cLower, "#youtube") ||
		strings.Contains(cLower, "#artigo") ||
		strings.Contains(cLower, "#captura") ||
		strings.Contains(cLower, "youtube.com/") ||
		strings.Contains(cLower, "youtu.be/") ||
		strings.Contains(cLower, "> fonte:") ||
		strings.Contains(filepath.ToSlash(path), "/links/")

	return a
}

// matchTags checks if a file's tags match the target tags based on the mode.
func matchTags(fileTags []string, targetTags []string, mode string) bool {
	if len(targetTags) == 0 {
		return false
	}

	// Normalize tags for comparison
	normalTarget := make(map[string]bool)
	for _, t := range targetTags {
		normalTarget[strings.ToLower(strings.TrimSpace(t))] = true
	}

	normalFile := make(map[string]bool)
	for _, t := range fileTags {
		normalFile[strings.ToLower(strings.TrimSpace(t))] = true
	}

	if mode == "only" {
		if len(normalFile) != len(normalTarget) {
			return false
		}
		for t := range normalTarget {
			if !normalFile[t] {
				return false
			}
		}
		return true
	}

	// Default mode: "any"
	for t := range normalTarget {
		if normalFile[t] {
			return true
		}
	}
	return false
}

func (ctx *HandlerContext) HandleGetStaleFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método não suportado", http.StatusMethodNotAllowed)
		return
	}

	daysStr := r.URL.Query().Get("days")
	includePhotos := r.URL.Query().Get("photos") == "true"
	includeNotes := r.URL.Query().Get("notes") == "true"
	includePdfs := r.URL.Query().Get("pdfs") == "true"
	zombies := r.URL.Query().Get("zombies") == "true"
	abandoned := r.URL.Query().Get("abandoned") == "true"
	captures := r.URL.Query().Get("captures") == "true"
	inactivity := r.URL.Query().Get("inactivity") != "false" // Default true
	minSizeMb, _ := strconv.ParseFloat(r.URL.Query().Get("minSizeMb"), 64)
	targetTagsRaw := r.URL.Query().Get("targetTags")
	tagMode := r.URL.Query().Get("tagMode") // "any" ou "only"

	var targetTags []string
	if targetTagsRaw != "" {
		for _, t := range strings.Split(targetTagsRaw, ",") {
			trimmed := strings.TrimSpace(t)
			if trimmed != "" {
				targetTags = append(targetTags, trimmed)
			}
		}
	}

	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 0 {
		days = 30
	}

	threshold := time.Now().AddDate(0, 0, -days)
	var result MaintenanceResult

	err = filepath.Walk(ctx.Cfg.DocsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(ctx.Cfg.DocsDir, path)
		ext := strings.ToLower(filepath.Ext(path))

		isNote := ext == ".md"
		isPhoto := ext == ".png" || ext == ".jpg" || ext == ".jpeg"
		isPdf := ext == ".pdf"

		if !((includeNotes && isNote) || (includePhotos && isPhoto) || (includePdfs && isPdf)) {
			return nil
		}

		if minSizeMb > 0 && float64(info.Size())/(1024*1024) < minSizeMb {
			return nil
		}

		ageDays := int(time.Since(info.ModTime()).Hours() / 24)
		if inactivity && !info.ModTime().Before(threshold) && ageDays < days {
			return nil
		}

		var reasons []string

		if isNote && includeNotes {
			advancedActive := zombies || abandoned || captures

			if advancedActive {
				// Read file once for all note-level checks
				analysis := analyseNote(path, info.Size())
				if zombies && analysis.isZombie {
					reasons = append(reasons, "Zumbi (Vazia / Só Título)")
				}
				if abandoned && analysis.abandoned {
					reasons = append(reasons, "Tarefas Abandonadas")
				}
				if captures && analysis.isCapture {
					reasons = append(reasons, "Captura (Artigo/YouTube)")
				}

				// Skip notes that don't match any advanced criterion
				if len(reasons) == 0 {
					return nil
				}
			}

			// Tag-based filtering
			if len(targetTags) > 0 {
				fileTags := ctx.State.GetFileTags(relPath)
				if matchTags(fileTags, targetTags, tagMode) {
					msg := "Hashtag específica"
					if tagMode == "only" {
						msg = "Hashtags exclusivas"
					}
					reasons = append(reasons, msg)
				} else {
					// Se não casou com as tags e o usuário filtrou por elas,
					// ignoramos o arquivo independente de outros critérios.
					return nil
				}
			}
		}

		if len(reasons) == 0 {
			if inactivity {
				reasons = append(reasons, "Inatividade")
			} else {
				reasons = append(reasons, "Seleção Manual")
			}
		}

		if len(reasons) == 0 {
			return nil
		}

		result.Files = append(result.Files, StaleFile{
			Name:    relPath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			AgeDays: ageDays,
			Reason:  strings.Join(reasons, " | "),
		})
		result.TotalSize += info.Size()
		result.TotalCount++
		return nil
	})

	if err != nil {
		http.Error(w, "Erro ao analisar arquivos: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (ctx *HandlerContext) HandleCleanupFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não suportado", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Days       int     `json:"days"`
		Photos     bool    `json:"photos"`
		Notes      bool    `json:"notes"`
		Pdfs       bool    `json:"pdfs"`
		Zombies    bool    `json:"zombies"`
		Abandoned  bool    `json:"abandoned"`
		Captures   bool    `json:"captures"`
		Inactivity bool    `json:"inactivity"`
		MinSizeMb  float64 `json:"minSizeMb"`
		TargetTags string  `json:"targetTags"`
		TagMode    string  `json:"tagMode"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	if req.Days < 0 {
		req.Days = 30
	}

	threshold := time.Now().AddDate(0, 0, -req.Days)
	deletedCount := 0
	var deletedFiles []string

	err := filepath.Walk(ctx.Cfg.DocsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(ctx.Cfg.DocsDir, path)
		ext := strings.ToLower(filepath.Ext(path))

		isNote := ext == ".md"
		isPhoto := ext == ".png" || ext == ".jpg" || ext == ".jpeg"
		isPdf := ext == ".pdf"

		if !((req.Notes && isNote) || (req.Photos && isPhoto) || (req.Pdfs && isPdf)) {
			return nil
		}

		if req.MinSizeMb > 0 && float64(info.Size())/(1024*1024) < req.MinSizeMb {
			return nil
		}

		ageDays := int(time.Since(info.ModTime()).Hours() / 24)
		if req.Inactivity && !info.ModTime().Before(threshold) && ageDays < req.Days {
			return nil
		}

		shouldDelete := true

		if isNote && req.Notes {
			advancedActive := req.Zombies || req.Abandoned || req.Captures
			if advancedActive {
				shouldDelete = false
				// Read file once for all note-level checks
				analysis := analyseNote(path, info.Size())
				if req.Zombies && analysis.isZombie {
					shouldDelete = true
				}
				if !shouldDelete && req.Abandoned && analysis.abandoned {
					shouldDelete = true
				}
				if !shouldDelete && req.Captures && analysis.isCapture {
					shouldDelete = true
				}
			}

			// Inactivity or Manual Selection check
			if !shouldDelete {
				if req.Inactivity || !req.Inactivity {
					shouldDelete = true
				}
			}

			// Tag-based filtering
			var targetTags []string
			if req.TargetTags != "" {
				for _, t := range strings.Split(req.TargetTags, ",") {
					trimmed := strings.TrimSpace(t)
					if trimmed != "" {
						targetTags = append(targetTags, trimmed)
					}
				}
			}

			if len(targetTags) > 0 {
				fileTags := ctx.State.GetFileTags(relPath)
				if !matchTags(fileTags, targetTags, req.TagMode) {
					shouldDelete = false
				}
			}
		}

		if shouldDelete {
			if err := os.Remove(path); err == nil {
				deletedCount++
				deletedFiles = append(deletedFiles, relPath)
				// Bug 3 Fix: coletar IDs antes de deletar para limpeza cirúrgica do state
				deletedIDs := ingest.CollectBleveIDsForFile(ctx.Cfg, relPath)
				// Bug 6 Fix: uma única goroutine por arquivo (era 2) para evitar goroutine flood
				utils.SafeGo(func() {
					// Variáveis locais para a closure assíncrona
					dCfg := ctx.Cfg
					dRp := relPath
					ingest.DeleteFileFromBleve(dCfg, dRp)
					ctx.State.DeleteVectorHash(dRp)
				})
				// Limpar state do arquivo deletado
				ctx.State.DeleteFileTags(relPath)
				ctx.State.DeleteHashesByIDs(deletedIDs)
				ctx.State.DeleteFileMod(path) // path é o absoluto neste contexto
			} else {
				log.Printf("[Cleanup] Erro ao deletar %s: %v\n", relPath, err)
			}
		}
		return nil
	})

	if err != nil {
		http.Error(w, "Erro durante a limpeza: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Persistir o state uma única vez após processar todos os arquivos
	ctx.State.Save(ctx.Cfg)
	search.ClearCache()
	ctx.State.RebuildKnownTagsCache()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "success",
		"deletedCount": deletedCount,
		"files":        deletedFiles,
	})
}
