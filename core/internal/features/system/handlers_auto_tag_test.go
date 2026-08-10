package system

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandleApplyAutoTag_SavesRulesAndApplies(t *testing.T) {
	ctx := newTestContext(t)

	// Nota com mtime antigo (45d), sem registro em popularity → fallback p/ mtime
	oldMtime := time.Now().Add(-45 * 24 * time.Hour).Format(time.RFC3339)
	path := "notes/old.md"
	content := "---\ntitle: Old\n---\nConteudo"
	if err := ctx.Store.SaveNote(path, content, oldMtime); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	if err := ctx.Store.SetFileMod(path, oldMtime); err != nil {
		t.Fatalf("SetFileMod: %v", err)
	}

	// Aplica enviando as regras no corpo (como o botão faz)
	body := `[{"days":30,"tag":"stale"}]`
	req := httptest.NewRequest("POST", "/api/settings/auto-tag/apply", bytes.NewReader([]byte(body)))
	rr := httptest.NewRecorder()
	ctx.HandleApplyAutoTag(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Status   string `json:"status"`
		Modified int    `json:"modified"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "success" || resp.Modified != 1 {
		t.Errorf("esperava success/1, got %+v", resp)
	}

	// A regra enviada foi salva na configuração
	val, _ := ctx.Store.GetSetting("auto_tag_decay_config")
	if !strings.Contains(val, "stale") {
		t.Errorf("esperava regra salva com 'stale', got %q", val)
	}

	// A tag foi aplicada na nota
	tags, _ := ctx.Store.GetFileTags(path)
	if len(tags) != 1 || tags[0] != "stale" {
		t.Errorf("esperava tag 'stale' na nota, got %v", tags)
	}
}

func TestHandleApplyAutoTag_WithoutBodyUsesSavedRules(t *testing.T) {
	ctx := newTestContext(t)

	// Configura regra salva diretamente
	if err := ctx.Store.SetSetting("auto_tag_decay_config", `[{"days":30,"tag":"stale"}]`); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}

	oldMtime := time.Now().Add(-45 * 24 * time.Hour).Format(time.RFC3339)
	path := "notes/old2.md"
	content := "---\ntitle: Old2\n---\nConteudo"
	if err := ctx.Store.SaveNote(path, content, oldMtime); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	if err := ctx.Store.SetFileMod(path, oldMtime); err != nil {
		t.Fatalf("SetFileMod: %v", err)
	}

	// Sem corpo → usa as regras salvas
	req := httptest.NewRequest("POST", "/api/settings/auto-tag/apply", nil)
	rr := httptest.NewRecorder()
	ctx.HandleApplyAutoTag(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Status   string `json:"status"`
		Modified int    `json:"modified"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Modified != 1 {
		t.Errorf("esperava 1 nota modificada com regras salvas, got %+v", resp)
	}

	tags, _ := ctx.Store.GetFileTags(path)
	if len(tags) != 1 || tags[0] != "stale" {
		t.Errorf("esperava tag 'stale' na nota, got %v", tags)
	}
}
