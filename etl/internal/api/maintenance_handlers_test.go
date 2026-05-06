package api

import (
	"encoding/json"
	"etl/internal/config"
	"etl/internal/ingest"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// createTempNote creates a temporary .md file with the given content for testing.
func createTempNote(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp note %s: %v", name, err)
	}
	return path
}

func TestAnalyseNote_Zombie_Empty(t *testing.T) {
	dir := t.TempDir()
	path := createTempNote(t, dir, "empty.md", "")

	info, _ := os.Stat(path)
	a := analyseNote(path, info.Size())

	if !a.isZombie {
		t.Error("esperava isZombie=true para arquivo vazio")
	}
}

func TestAnalyseNote_Zombie_OnlyTitle(t *testing.T) {
	dir := t.TempDir()
	path := createTempNote(t, dir, "title_only.md", "# 2026-04-20 00:18:07\n\n")

	info, _ := os.Stat(path)
	a := analyseNote(path, info.Size())

	if !a.isZombie {
		t.Error("esperava isZombie=true para nota com apenas título H1")
	}
}

func TestAnalyseNote_Zombie_OnlyFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := createTempNote(t, dir, "frontmatter_only.md", "---\ntags: [teste]\n---\n")

	info, _ := os.Stat(path)
	a := analyseNote(path, info.Size())

	if !a.isZombie {
		t.Error("esperava isZombie=true para nota com apenas Frontmatter")
	}
}

func TestAnalyseNote_Zombie_FrontmatterPlusTitle(t *testing.T) {
	dir := t.TempDir()
	path := createTempNote(t, dir, "fm_title_only.md", "---\ntags: [teste]\n---\n# Meu Título\n\n")

	info, _ := os.Stat(path)
	a := analyseNote(path, info.Size())

	if !a.isZombie {
		t.Error("esperava isZombie=true para nota com Frontmatter + só título H1")
	}
}

func TestAnalyseNote_NotZombie_WithContent(t *testing.T) {
	dir := t.TempDir()
	path := createTempNote(t, dir, "real.md", "# Meu Título\n\nEsse é um conteúdo real e válido.")

	info, _ := os.Stat(path)
	a := analyseNote(path, info.Size())

	if a.isZombie {
		t.Error("esperava isZombie=false para nota com conteúdo real")
	}
}

func TestAnalyseNote_Abandoned_ThreePlusCheckboxes(t *testing.T) {
	dir := t.TempDir()
	content := "# Projeto Abandonado\n\n- [ ] Tarefa 1\n- [ ] Tarefa 2\n- [ ] Tarefa 3\n"
	path := createTempNote(t, dir, "abandoned.md", content)

	info, _ := os.Stat(path)
	a := analyseNote(path, info.Size())

	if !a.abandoned {
		t.Error("esperava abandoned=true para nota com 3 checkboxes pendentes")
	}
}

func TestAnalyseNote_NotAbandoned_TwoCheckboxes(t *testing.T) {
	dir := t.TempDir()
	content := "# Projeto Ativo\n\n- [ ] Tarefa 1\n- [ ] Tarefa 2\n- [x] Feito\n"
	path := createTempNote(t, dir, "active.md", content)

	info, _ := os.Stat(path)
	a := analyseNote(path, info.Size())

	if a.abandoned {
		t.Error("esperava abandoned=false para nota com apenas 2 checkboxes pendentes")
	}
}

func TestAnalyseNote_BothZombieAndAbandoned(t *testing.T) {
	// A note that is only a title cannot also be "abandoned" (no checkboxes),
	// but a note with ONLY 3 checkboxes and no real text IS a zombie.
	dir := t.TempDir()
	content := "# Título\n\n- [ ] A\n- [ ] B\n- [ ] C\n"
	path := createTempNote(t, dir, "both.md", content)

	info, _ := os.Stat(path)
	a := analyseNote(path, info.Size())

	if a.isZombie {
		t.Error("nota com apenas checkboxes e nenhum outro texto: não deveria ser zumbi")
	}
	if !a.abandoned {
		t.Error("nota com 3 checkboxes pendentes: deveria ser abandoned")
	}
}

func TestMatchTags(t *testing.T) {
	tests := []struct {
		name       string
		fileTags   []string
		targetTags []string
		mode       string
		want       bool
	}{
		{"Any - Simple Match", []string{"#a", "#b"}, []string{"#a"}, "any", true},
		{"Any - Multi target Match", []string{"#a", "#c"}, []string{"#a", "#b"}, "any", true},
		{"Any - Case Insensitive", []string{"#TEST"}, []string{"#test"}, "any", true},
		{"Any - No Match", []string{"#a", "#b"}, []string{"#c"}, "any", false},
		{"Only - Exact Match", []string{"#a", "#b"}, []string{"#a", "#b"}, "only", true},
		{"Only - Diff order", []string{"#b", "#a"}, []string{"#a", "#b"}, "only", true},
		{"Only - More tags on file", []string{"#a", "#b", "#c"}, []string{"#a", "#b"}, "only", false},
		{"Only - Fewer tags on file", []string{"#a"}, []string{"#a", "#b"}, "only", false},
		{"Only - Case Insensitive", []string{"#TEST"}, []string{"#test"}, "only", true},
		{"Empty Target", []string{"#a"}, []string{}, "any", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchTags(tt.fileTags, tt.targetTags, tt.mode); got != tt.want {
				t.Errorf("matchTags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandleGetStaleFiles_WithTags(t *testing.T) {
	dir := t.TempDir()
	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{DocsDir: dir},
		State: appState,
	}

	// Criar arquivos de teste
	createTempNote(t, dir, "a.md", "# A")
	createTempNote(t, dir, "b.md", "# B")
	createTempNote(t, dir, "c.md", "# C")

	appState.SetFileTags("a.md", []string{"#t1", "#t2"})
	appState.SetFileTags("b.md", []string{"#t1"})
	appState.SetFileTags("c.md", []string{"#t3"})

	t.Run("Match Any t1", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/maintenance/stale?targetTags=%23t1&tagMode=any&days=0&notes=true", nil)
		w := httptest.NewRecorder()
		ctx.HandleGetStaleFiles(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Code: %d", w.Code)
		}

		var res MaintenanceResult
		json.NewDecoder(w.Body).Decode(&res)

		if res.TotalCount != 2 {
			t.Errorf("Esperava 2 arquivos (#t1), obteve %d", res.TotalCount)
		}
	})

	t.Run("Match Only t1", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/maintenance/stale?targetTags=%23t1&tagMode=only&days=0&notes=true", nil)
		w := httptest.NewRecorder()
		ctx.HandleGetStaleFiles(w, req)

		var res MaintenanceResult
		json.NewDecoder(w.Body).Decode(&res)

		if res.TotalCount != 1 {
			t.Errorf("Esperava 1 arquivo (#t1 exclusivo), obteve %d", res.TotalCount)
		}
		if res.Files[0].Name != "b.md" {
			t.Errorf("Esperava b.md, obteve %s", res.Files[0].Name)
		}
	})
}
