package system

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── HandleTodosPage ────────────────────────────────────────────────────

func TestHandleTodosPage_Retorna200(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/todos", nil)

	ctx.HandleTodosPage(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200, got %d", rec.Code)
	}
}

func TestHandleTodosPage_ContemTituloCorreto(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/todos", nil)

	ctx.HandleTodosPage(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "TON-618") {
		t.Errorf("esperado 'TON-618' no corpo, got %d chars", len(body))
	}
}

// ── HandleTodoSettingsPage ────────────────────────────────────────────

func TestHandleTodoSettingsPage_RedirecionalParaRaiz(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/settings/todos", nil)

	ctx.HandleTodoSettingsPage(rec, req)

	if rec.Code != 303 {
		t.Errorf("esperado 303 SeeOther, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc != "/" {
		t.Errorf("esperado redirect para /, got %q", loc)
	}
}

// ── HandleGetNtfySettings ─────────────────────────────────────────────

func TestHandleGetNtfySettings_Retorna200(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/settings/ntfy", nil)

	ctx.HandleGetNtfySettings(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200, got %d", rec.Code)
	}
}

func TestHandleGetNtfySettings_ValoresConfiguradorAparecemNaResposta(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Store.SetSetting("ntfy_url", "https://ntfy.sh")
	ctx.Store.SetSetting("ntfy_topic", "meu-topico")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/settings/ntfy", nil)

	ctx.HandleGetNtfySettings(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "ntfy.sh") {
		t.Errorf("esperado URL do ntfy no corpo, got %q", body)
	}
	if !strings.Contains(body, "meu-topico") {
		t.Errorf("esperado topic do ntfy no corpo, got %q", body)
	}
}

// ── HandlePostNtfySettings ────────────────────────────────────────────

func TestHandlePostNtfySettings_SalvaConfigurações(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader("ntfy_url=https%3A%2F%2Fntfy.sh&ntfy_topic=meu-topico&ntfy_user=user&ntfy_pass=pass")
	req := httptest.NewRequest("POST", "/api/settings/ntfy", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandlePostNtfySettings(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200 ao salvar ntfy settings, got %d", rec.Code)
	}

	// Verifica que os valores foram persistidos
	url, _ := ctx.Store.GetSetting("ntfy_url")
	if url != "https://ntfy.sh" {
		t.Errorf("ntfy_url não foi salvo corretamente: %q", url)
	}
	topic, _ := ctx.Store.GetSetting("ntfy_topic")
	if topic != "meu-topico" {
		t.Errorf("ntfy_topic não foi salvo corretamente: %q", topic)
	}
}

func TestHandlePostNtfySettings_ValoresAparecemNaResposta(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader("ntfy_url=https%3A%2F%2Fntfy.example.com&ntfy_topic=notificacoes&ntfy_user=&ntfy_pass=")
	req := httptest.NewRequest("POST", "/api/settings/ntfy", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandlePostNtfySettings(rec, req)

	respBody := rec.Body.String()
	if !strings.Contains(respBody, "ntfy.example.com") {
		t.Errorf("esperado URL no corpo da resposta, got %q", respBody)
	}
}

// ── HandleGetAgendaNotifyHours ─────────────────────────────────────────

func TestHandleGetAgendaNotifyHours_PadraoEh24(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/settings/agenda-notify", nil)

	ctx.HandleGetAgendaNotifyHours(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("erro ao decodificar JSON: %v", err)
	}
	hours, ok := resp["hours"]
	if !ok {
		t.Error("esperado campo 'hours' na resposta")
	}
	if hours != "24" {
		t.Errorf("esperado hours='24' por padrão, got %v", hours)
	}
}

func TestHandleGetAgendaNotifyHours_RetornaValorConfigurado(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Store.SetSetting("agenda_notify_hours", "48")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/settings/agenda-notify", nil)

	ctx.HandleGetAgendaNotifyHours(rec, req)

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["hours"] != "48" {
		t.Errorf("esperado hours='48', got %v", resp["hours"])
	}
}

// ── HandlePostAgendaNotifyHours ────────────────────────────────────────

func TestHandlePostAgendaNotifyHours_SalvaValorValido(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader("hours=12")
	req := httptest.NewRequest("POST", "/api/settings/agenda-notify", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandlePostAgendaNotifyHours(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if ok, _ := resp["ok"].(bool); !ok {
		t.Errorf("esperado ok=true, got %v", resp)
	}

	saved, _ := ctx.Store.GetSetting("agenda_notify_hours")
	if saved != "12" {
		t.Errorf("esperado agenda_notify_hours='12', got %q", saved)
	}
}

func TestHandlePostAgendaNotifyHours_ValorInvalido_RetornaErro(t *testing.T) {
	ctx := newTestContext(t)

	cases := []struct {
		name  string
		hours string
	}{
		{"zero", "0"},
		{"negativo", "-5"},
		{"acima do limite", "721"},
		{"nao numerico", "abc"},
		{"vazio", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			body := strings.NewReader("hours=" + tc.hours)
			req := httptest.NewRequest("POST", "/api/settings/agenda-notify", body)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			ctx.HandlePostAgendaNotifyHours(rec, req)

			var resp map[string]interface{}
			json.NewDecoder(rec.Body).Decode(&resp)
			if ok, _ := resp["ok"].(bool); ok {
				t.Errorf("esperado ok=false para horas=%q, got ok=true", tc.hours)
			}
		})
	}
}

// ── HandlePostSemanticThresholds ──────────────────────────────────────

func TestHandlePostSemanticThresholds_SalvaComSucesso(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"search_threshold": 50, "rrf_k": 30, "hybrid_threshold": 60}`)
	req := httptest.NewRequest("POST", "/api/settings/semantic-thresholds", body)
	req.Header.Set("Content-Type", "application/json")

	ctx.HandlePostSemanticThresholds(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200, got %d — body: %s", rec.Code, rec.Body.String())
	}

	// Verifica persitência
	v, _ := ctx.Store.GetSetting("semantic_search_threshold")
	if v != "50" {
		t.Errorf("esperado semantic_search_threshold=50, got %q", v)
	}
	v2, _ := ctx.Store.GetSetting("rrf_k")
	if v2 != "30" {
		t.Errorf("esperado rrf_k=30, got %q", v2)
	}
	v3, _ := ctx.Store.GetSetting("hybrid_semantic_threshold")
	if v3 != "60" {
		t.Errorf("esperado hybrid_semantic_threshold=60, got %q", v3)
	}
}

func TestHandlePostSemanticThresholds_JSONInvalido_Retorna400(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/settings/semantic-thresholds", strings.NewReader("nao e json"))
	req.Header.Set("Content-Type", "application/json")

	ctx.HandlePostSemanticThresholds(rec, req)

	if rec.Code != 400 {
		t.Errorf("esperado 400 para JSON invalido, got %d", rec.Code)
	}
}

func TestHandlePostSemanticThresholds_SearchThresholdForaDoRange_Retorna400(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"search_threshold": 150}`)
	req := httptest.NewRequest("POST", "/api/settings/semantic-thresholds", body)
	req.Header.Set("Content-Type", "application/json")

	ctx.HandlePostSemanticThresholds(rec, req)

	if rec.Code != 400 {
		t.Errorf("esperado 400 para threshold > 100, got %d", rec.Code)
	}
}

func TestHandlePostSemanticThresholds_RrfKForaDoRange_Retorna400(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"rrf_k": 5}`)
	req := httptest.NewRequest("POST", "/api/settings/semantic-thresholds", body)
	req.Header.Set("Content-Type", "application/json")

	ctx.HandlePostSemanticThresholds(rec, req)

	if rec.Code != 400 {
		t.Errorf("esperado 400 para rrf_k < 10, got %d", rec.Code)
	}
}

func TestHandlePostSemanticThresholds_MetodoInvalido_Retorna405(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/settings/semantic-thresholds", nil)

	ctx.HandlePostSemanticThresholds(rec, req)

	if rec.Code != 405 {
		t.Errorf("esperado 405 para GET, got %d", rec.Code)
	}
}

// ── HandleGetSemanticThresholds ────────────────────────────────────────

func TestHandleGetSemanticThresholds_RetornaDefaults(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/settings/semantic-thresholds", nil)

	ctx.HandleGetSemanticThresholds(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200, got %d", rec.Code)
	}

	var resp map[string]int
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("erro ao decodificar JSON: %v", err)
	}
	if resp["search_threshold"] != 35 {
		t.Errorf("esperado search_threshold=35, got %d", resp["search_threshold"])
	}
	if resp["rrf_k"] != 60 {
		t.Errorf("esperado rrf_k=60, got %d", resp["rrf_k"])
	}
	if resp["hybrid_threshold"] != 55 {
		t.Errorf("esperado hybrid_threshold=55, got %d", resp["hybrid_threshold"])
	}
}

func TestHandleGetSemanticThresholds_RetornaValoresSalvos(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Store.SetSetting("semantic_search_threshold", "70")
	ctx.Store.SetSetting("rrf_k", "25")
	ctx.Store.SetSetting("hybrid_semantic_threshold", "80")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/settings/semantic-thresholds", nil)

	ctx.HandleGetSemanticThresholds(rec, req)

	var resp map[string]int
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["search_threshold"] != 70 {
		t.Errorf("esperado search_threshold=70, got %d", resp["search_threshold"])
	}
	if resp["rrf_k"] != 25 {
		t.Errorf("esperado rrf_k=25, got %d", resp["rrf_k"])
	}
	if resp["hybrid_threshold"] != 80 {
		t.Errorf("esperado hybrid_threshold=80, got %d", resp["hybrid_threshold"])
	}
}

// ── HandleGetAutoTagSettings ──────────────────────────────────────────

func TestHandleGetAutoTagSettings_SemConfig_RetornaArrayVazio(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/settings/auto-tag", nil)

	ctx.HandleGetAutoTagSettings(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200, got %d", rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	if body != "[]" {
		t.Errorf("esperado '[]' quando sem config, got %q", body)
	}
}

func TestHandleGetAutoTagSettings_RetornaConfigSalva(t *testing.T) {
	ctx := newTestContext(t)
	config := `[{"tag":"inativo","days":30}]`
	ctx.Store.SetSetting("auto_tag_decay_config", config)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/settings/auto-tag", nil)

	ctx.HandleGetAutoTagSettings(rec, req)

	body := strings.TrimSpace(rec.Body.String())
	if body != config {
		t.Errorf("esperado config salva, got %q", body)
	}
}

// ── HandlePostAutoTagSettings ─────────────────────────────────────────

func TestHandlePostAutoTagSettings_SalvaRegrasValidas(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader(`[{"tag":"inativo","days":30},{"tag":"antigo","days":90}]`)
	req := httptest.NewRequest("POST", "/api/settings/auto-tag", body)
	req.Header.Set("Content-Type", "application/json")

	ctx.HandlePostAutoTagSettings(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200, got %d — body: %s", rec.Code, rec.Body.String())
	}

	saved, _ := ctx.Store.GetSetting("auto_tag_decay_config")
	if !strings.Contains(saved, "inativo") {
		t.Errorf("esperado regra 'inativo' salva, got %q", saved)
	}
}

func TestHandlePostAutoTagSettings_JSONInvalido_Retorna400(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/settings/auto-tag", strings.NewReader("nao e json"))
	req.Header.Set("Content-Type", "application/json")

	ctx.HandlePostAutoTagSettings(rec, req)

	if rec.Code != 400 {
		t.Errorf("esperado 400, got %d", rec.Code)
	}
}

func TestHandlePostAutoTagSettings_DiasMenorQueUm_Retorna400(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader(`[{"tag":"inativo","days":0}]`)
	req := httptest.NewRequest("POST", "/api/settings/auto-tag", body)
	req.Header.Set("Content-Type", "application/json")

	ctx.HandlePostAutoTagSettings(rec, req)

	if rec.Code != 400 {
		t.Errorf("esperado 400 para days=0, got %d", rec.Code)
	}
}

func TestHandlePostAutoTagSettings_TagVazia_Retorna400(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader(`[{"tag":"","days":30}]`)
	req := httptest.NewRequest("POST", "/api/settings/auto-tag", body)
	req.Header.Set("Content-Type", "application/json")

	ctx.HandlePostAutoTagSettings(rec, req)

	if rec.Code != 400 {
		t.Errorf("esperado 400 para tag vazia, got %d", rec.Code)
	}
}

func TestHandlePostAutoTagSettings_RemoveHashDaTag(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader(`[{"tag":"#inativo","days":30}]`)
	req := httptest.NewRequest("POST", "/api/settings/auto-tag", body)
	req.Header.Set("Content-Type", "application/json")

	ctx.HandlePostAutoTagSettings(rec, req)

	if rec.Code != 200 {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}

	saved, _ := ctx.Store.GetSetting("auto_tag_decay_config")
	// A tag deve ter o # removido
	if strings.Contains(saved, "#inativo") {
		t.Errorf("esperado tag sem #, got config: %q", saved)
	}
	if !strings.Contains(saved, `"inativo"`) {
		t.Errorf("esperado tag 'inativo' (sem #) na config salva, got %q", saved)
	}
}

func TestHandlePostAutoTagSettings_MetodoInvalido_Retorna405(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/settings/auto-tag", nil)

	ctx.HandlePostAutoTagSettings(rec, req)

	if rec.Code != 405 {
		t.Errorf("esperado 405 para GET, got %d", rec.Code)
	}
}

// ── HandleGetSemanticDevice / HandlePostSemanticDevice ────────────────

func TestHandleGetSemanticDevice_PadraoEhWasm(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/settings/semantic-device", nil)

	ctx.HandleGetSemanticDevice(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"wasm"`) {
		t.Errorf("esperado device='wasm' por padrão, got %q", body)
	}
}

func TestHandleGetSemanticDevice_RetornaValorSalvo(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Store.SetSetting("semantic_device", "auto")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/settings/semantic-device", nil)

	ctx.HandleGetSemanticDevice(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `"auto"`) {
		t.Errorf("esperado device='auto', got %q", body)
	}
}

func TestHandlePostSemanticDevice_SalvaWasm(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"device":"wasm"}`)
	req := httptest.NewRequest("POST", "/api/settings/semantic-device", body)
	req.Header.Set("Content-Type", "application/json")

	ctx.HandlePostSemanticDevice(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200, got %d", rec.Code)
	}
	saved, _ := ctx.Store.GetSetting("semantic_device")
	if saved != "wasm" {
		t.Errorf("esperado device='wasm' salvo, got %q", saved)
	}
}

func TestHandlePostSemanticDevice_SalvaAuto(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"device":"auto"}`)
	req := httptest.NewRequest("POST", "/api/settings/semantic-device", body)
	req.Header.Set("Content-Type", "application/json")

	ctx.HandlePostSemanticDevice(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200, got %d", rec.Code)
	}
}

func TestHandlePostSemanticDevice_DeviceInvalido_Retorna400(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"device":"gpu"}`)
	req := httptest.NewRequest("POST", "/api/settings/semantic-device", body)
	req.Header.Set("Content-Type", "application/json")

	ctx.HandlePostSemanticDevice(rec, req)

	if rec.Code != 400 {
		t.Errorf("esperado 400 para device inválido, got %d", rec.Code)
	}
}

func TestHandlePostSemanticDevice_JSONInvalido_Retorna400(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/settings/semantic-device", strings.NewReader("nao e json"))

	ctx.HandlePostSemanticDevice(rec, req)

	if rec.Code != 400 {
		t.Errorf("esperado 400, got %d", rec.Code)
	}
}

func TestHandlePostSemanticDevice_MetodoInvalido_Retorna405(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/settings/semantic-device", nil)

	ctx.HandlePostSemanticDevice(rec, req)

	if rec.Code != 405 {
		t.Errorf("esperado 405, got %d", rec.Code)
	}
}
