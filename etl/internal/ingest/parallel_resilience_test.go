package ingest

import (
	"context"
	"etl/internal/config"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParallelPanicRecovery(t *testing.T) {
	// 1. Setup temporary directory structure
	tmpDir := t.TempDir()
	notesDir := filepath.Join(tmpDir, "notes")
	os.MkdirAll(notesDir, 0755)

	// 2. Create a GOOD file
	goodFile := filepath.Join(notesDir, "good.md")
	os.WriteFile(goodFile, []byte("# Good File\nThis should be processed."), 0644)

	// 3. Create a PANIC-TRIGGERING file
	panicFile := filepath.Join(notesDir, "panic.md")
	os.WriteFile(panicFile, []byte("# Poison File\nPANIC_TEST_TRIGGER"), 0644)

	// 4. Create another GOOD file to ensure parallelism continues
	anotherGoodFile := filepath.Join(notesDir, "another.md")
	os.WriteFile(anotherGoodFile, []byte("# Another Good File\nThis should also be processed."), 0644)

	cfg := &config.AppConfig{
		DocsDir:  tmpDir,
		StateDir: tmpDir,
	}

	// 5. Run ProcessDocs
	// We use defer recover here just in case the top-level ProcessDocs fails,
	// but the goal is to see that it handles the internal goroutine panic.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ProcessDocs crashed! Panic was not recovered internally: %v", r)
		}
	}()

	appState := NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	defer appState.Close()
	// Note: ProcessDocs is synchronous in waiting for workers to finish (Wait Group)
	docs, _, _ := ProcessDocs(cfg, false, appState)

	// 6. Verify results
	foundGood := false
	foundAnother := false
	for _, doc := range docs {
		if doc.Arquivo == "notes/good.md" {
			foundGood = true
		}
		if doc.Arquivo == "notes/another.md" {
			foundAnother = true
		}
		if doc.Arquivo == "notes/panic.md" {
			t.Errorf("Panic file should not produce documents")
		}
	}

	if !foundGood {
		t.Error("Good file was not processed")
	}
	if !foundAnother {
		t.Error("Second good file was not processed")
	}

	// 7. Test StartWatcher graceful shutdown while we are here
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// StartWatcher is a blocking loop, it should exit when ctx is cancelled
	StartWatcher(ctx, cfg, appState, nil)

	t.Log("Resilience test passed: survived panic and exited watcher gracefully.")
}
