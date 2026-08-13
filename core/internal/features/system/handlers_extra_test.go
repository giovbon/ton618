package system

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleGetDatabaseData_CaseInsensitiveFrontmatter(t *testing.T) {
	ctx := newTestContext(t)

	// Save a note with "Tags" and "Type" (capitalized) in frontmatter
	content := `---
Tags: ["#golang", "sqlite"]
Type: "note"
custom_field: "value"
---
# Test Note`

	// Use Notes.Save to parse frontmatter and populate the tags table in DB
	if err := ctx.Notes.Save("notes/test_fm.md", content, nil); err != nil {
		t.Fatalf("Notes.Save: %v", err)
	}

	// Call HandleGetDatabaseData
	req := httptest.NewRequest("GET", "/api/database/data", nil)
	rr := httptest.NewRecorder()
	ctx.HandleGetDatabaseData(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status code: %d", rr.Code)
	}

	var response struct {
		Columns []map[string]interface{} `json:"columns"`
		Data    []map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Verify columns: we should NOT have columns for "Tags", "Type".
	// The built-in tags and type will be present once (lowercase).
	tagsCount := 0
	typeCount := 0
	hasCustomField := false

	for _, col := range response.Columns {
		field := col["field"].(string)
		if field == "Tags" || field == "Type" {
			t.Errorf("unexpected capitalized column: %s", field)
		}
		if field == "tags" {
			tagsCount++
		}
		if field == "type" {
			typeCount++
		}
		if field == "custom_field" {
			hasCustomField = true
		}
	}
	if tagsCount > 1 {
		t.Errorf("duplicate tags column found: %d", tagsCount)
	}
	if typeCount > 1 {
		t.Errorf("duplicate type column found: %d", typeCount)
	}
	if !hasCustomField {
		t.Error("expected custom_field column")
	}

	// Verify data: tags should be normalized without '#' prefix
	if len(response.Data) != 1 {
		t.Fatalf("expected 1 row, got %d", len(response.Data))
	}
	row := response.Data[0]
	// Tags should be joined by comma, and stripped of '#' prefix
	if tagsVal, ok := row["tags"].(string); !ok || tagsVal != "golang, sqlite" {
		t.Errorf("expected tags 'golang, sqlite', got %q", row["tags"])
	}
}

func TestHandleGetDatabaseData_InternalFieldsNotColumns(t *testing.T) {
	ctx := newTestContext(t)

	// Save a note so there's at least one row
	content := `---
tags: [golang]
---
# Test Note`
	if err := ctx.Notes.Save("notes/test_internal.md", content, nil); err != nil {
		t.Fatalf("Notes.Save: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/database/data", nil)
	rr := httptest.NewRecorder()
	ctx.HandleGetDatabaseData(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status code: %d", rr.Code)
	}

	var response struct {
		Columns []map[string]interface{} `json:"columns"`
		Data    []map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Campos internos (_blank, _icon, _url) NÃO podem virar colunas visíveis.
	for _, col := range response.Columns {
		field, _ := col["field"].(string)
		if field == "_blank" || field == "_icon" || field == "_url" {
			t.Errorf("internal helper field exposed as column: %s", field)
		}
	}

	// Mas devem continuar presentes nos dados das linhas (usados pelo formatter abrir_link).
	if len(response.Data) == 0 {
		t.Fatal("expected at least 1 row")
	}
	for i, row := range response.Data {
		if _, ok := row["_blank"]; !ok {
			t.Errorf("row %d: missing _blank in data", i)
		}
		if _, ok := row["_icon"]; !ok {
			t.Errorf("row %d: missing _icon in data", i)
		}
		if _, ok := row["_url"]; !ok {
			t.Errorf("row %d: missing _url in data", i)
		}
	}
}

func TestHandleUpdateNoteProperty_NormalizesTagsAndStripHash(t *testing.T) {
	ctx := newTestContext(t)

	content := `---
tags: [old]
---
# Test Note`
	saveTestNote(t, ctx, "notes/test_fm.md", content, "")

	// Request to update 'tags' (using case-variant key name like 'Tags' or 'tags' with '#' prefix)
	updatePayload := UpdatePropertyRequest{
		File:  "notes/test_fm.md",
		Key:   "tags",
		Value: "#newtag, #other, normal",
	}
	bodyBytes, _ := json.Marshal(updatePayload)
	req := httptest.NewRequest("POST", "/api/notes/update-property", bytes.NewReader(bodyBytes))
	rr := httptest.NewRecorder()
	ctx.HandleUpdateNoteProperty(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status code: %d, body: %s", rr.Code, rr.Body.String())
	}

	// Read note content from DB and verify frontmatter has tags updated and normalized
	updatedContent, err := ctx.Store.GetNote("notes/test_fm.md")
	if err != nil {
		t.Fatalf("get note: %v", err)
	}

	if !strings.Contains(updatedContent, "newtag") || !strings.Contains(updatedContent, "other") {
		t.Errorf("expected tags updated, got content: %s", updatedContent)
	}
	if strings.Contains(updatedContent, "#newtag") {
		t.Error("leading '#' was not stripped from tags")
	}
}
