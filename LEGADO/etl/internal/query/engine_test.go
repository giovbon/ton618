package query

import (
	"etl/internal/config"
	"etl/internal/ingest"
	"testing"
)

func TestQueryExecute(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.AppConfig{StateDir: tmpDir, DocsDir: tmpDir}
	state := ingest.NewAppState(cfg)
	defer state.Close()

	// Setup data
	state.SetFileTags("note1.md", []string{"projeto"})
	state.SetFileMetadata("note1.md", map[string]interface{}{
		"status":   "doing",
		"priority": 1,
	})

	state.SetFileTags("note2.md", []string{"projeto"})
	state.SetFileMetadata("note2.md", map[string]interface{}{
		"status":   "done",
		"priority": 2,
	})

	state.SetFileTags("other.md", []string{"personal"})
	state.SetFileMetadata("other.md", map[string]interface{}{
		"status": "doing",
	})

	t.Run("TABLE with tags and WHERE", func(t *testing.T) {
		q := "TABLE status, priority FROM #projeto WHERE status == 'done'"
		res, err := Execute(q, state, cfg)
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		if len(res.Rows) != 1 {
			t.Errorf("Expected 1 row, got %d", len(res.Rows))
		}

		if res.Rows[0][1] != "done" {
			t.Errorf("Expected status 'done', got %v", res.Rows[0][1])
		}
	})

	t.Run("LIST from tag", func(t *testing.T) {
		q := "LIST FROM #projeto"
		res, err := Execute(q, state, cfg)
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		if len(res.Rows) != 2 {
			t.Errorf("Expected 2 rows, got %d", len(res.Rows))
		}
	})

	t.Run("TABLE from folder prefix", func(t *testing.T) {
		q := "TABLE status FROM \"other\""
		res, err := Execute(q, state, cfg)
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		if len(res.Rows) != 1 {
			t.Errorf("Expected 1 row, got %d", len(res.Rows))
		}
	})

	t.Run("Aggregation count", func(t *testing.T) {
		q := "TABLE count(file.name) FROM #projeto"
		res, err := Execute(q, state, cfg)
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		if len(res.Rows) != 1 {
			t.Errorf("Expected 1 summary row, got %d", len(res.Rows))
		}

		countVal := res.Rows[0][1] // Index 0 is "Summary", Index 1 is count
		if countVal != 2 {
			t.Errorf("Expected count 2, got %v", countVal)
		}
	})
}
