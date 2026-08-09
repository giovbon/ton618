package services

import (
	"archive/zip"
	"bytes"
	"testing"

	"ton618/core/internal/core/db"
)

func TestScratchBackup(t *testing.T) {
	dbPath := "/home/giobon/Área de trabalho/ton618/core/data/ton618.db"
	docsDir := "/home/giobon/Área de trabalho/ton618/core/docs"

	store, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("ERROR NewStore: %v", err)
	}
	defer store.Close()

	backupSvc := NewBackupService(store, store, docsDir)

	t.Log("--- Testing Quick Backup (full=false) ---")
	dataQuick, err := backupSvc.Create(false)
	if err != nil {
		t.Errorf("ERROR Create(false): %v", err)
	} else {
		t.Logf("SUCCESS Create(false): generated %d bytes", len(dataQuick))
		r, err := zip.NewReader(bytes.NewReader(dataQuick), int64(len(dataQuick)))
		if err != nil {
			t.Errorf("Zip reader quick: %v", err)
		} else {
			t.Logf("Quick zip file count: %d", len(r.File))
			for _, f := range r.File {
				t.Logf("  File: %s (%d bytes)", f.Name, f.UncompressedSize64)
			}
		}
	}

	t.Log("--- Testing Full Backup (full=true) ---")
	dataFull, err := backupSvc.Create(true)
	if err != nil {
		t.Errorf("ERROR Create(true): %v", err)
	} else {
		t.Logf("SUCCESS Create(true): generated %d bytes", len(dataFull))
		r, err := zip.NewReader(bytes.NewReader(dataFull), int64(len(dataFull)))
		if err != nil {
			t.Errorf("Zip reader full: %v", err)
		} else {
			t.Logf("Full zip file count: %d", len(r.File))
			for _, f := range r.File {
				t.Logf("  File: %s (%d bytes)", f.Name, f.UncompressedSize64)
			}
		}
	}
}
