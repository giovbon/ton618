package system

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"ton618/core/internal/core/domain"
	"ton618/core/internal/features/notes"
	"ton618/core/internal/httputil"
)

// ── Handlers de Configurações de Sistema (Settings) ──

func (ctx *HandlerContext) HandleGetNtfySettings(w http.ResponseWriter, r *http.Request) {
	url, _ := ctx.Store.GetSetting("ntfy_url")
	topic, _ := ctx.Store.GetSetting("ntfy_topic")
	user, _ := ctx.Store.GetSetting("ntfy_user")
	pass, _ := ctx.Store.GetSetting("ntfy_pass")

	NtfySettings(url, topic, user, pass, false).Render(r.Context(), w)
}

func (ctx *HandlerContext) HandlePostNtfySettings(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	url := r.FormValue("ntfy_url")
	topic := r.FormValue("ntfy_topic")
	user := r.FormValue("ntfy_user")
	pass := r.FormValue("ntfy_pass")

	ctx.Store.SetSetting("ntfy_url", url)
	ctx.Store.SetSetting("ntfy_topic", topic)
	ctx.Store.SetSetting("ntfy_user", user)
	ctx.Store.SetSetting("ntfy_pass", pass)

	NtfySettings(url, topic, user, pass, true).Render(r.Context(), w)
}

func (ctx *HandlerContext) HandleGetAgendaNotifyHours(w http.ResponseWriter, r *http.Request) {
	hours := "24"
	if v, err := ctx.Store.GetSetting("agenda_notify_hours"); err == nil && v != "" {
		hours = v
	}
	httputil.WriteJSON(w, map[string]interface{}{"hours": hours})
}

func (ctx *HandlerContext) HandlePostAgendaNotifyHours(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(r.FormValue("hours"))
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 || n > 720 {
		httputil.WriteJSON(w, map[string]interface{}{"ok": false, "error": "horas inválidas (1–720)"})
		return
	}
	ctx.Store.SetSetting("agenda_notify_hours", strconv.Itoa(n))
	httputil.WriteJSON(w, map[string]interface{}{"ok": true, "hours": n})
}

func (ctx *HandlerContext) HandleGetAutoTagSettings(w http.ResponseWriter, r *http.Request) {
	val, err := ctx.Store.GetSetting("auto_tag_decay_config")
	if err != nil || val == "" {
		val = "[]"
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(val))
}

func (ctx *HandlerContext) HandlePostAutoTagSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var rules []domain.AutoTagRule
	if err := json.NewDecoder(r.Body).Decode(&rules); err != nil {
		http.Error(w, "json invalido", http.StatusBadRequest)
		return
	}

	for i, rule := range rules {
		if rule.Days <= 0 {
			http.Error(w, "os dias devem ser maiores que zero", http.StatusBadRequest)
			return
		}
		rules[i].Tag = strings.TrimSpace(rule.Tag)
		rules[i].Tag = strings.TrimPrefix(rules[i].Tag, "#")
		if rules[i].Tag == "" {
			http.Error(w, "a tag não pode ser vazia", http.StatusBadRequest)
			return
		}
	}

	configJSON, err := json.Marshal(rules)
	if err != nil {
		http.Error(w, "erro ao processar dados", http.StatusInternalServerError)
		return
	}

	if err := ctx.Store.SetSetting("auto_tag_decay_config", string(configJSON)); err != nil {
		http.Error(w, "erro ao salvar", http.StatusInternalServerError)
		return
	}

	httputil.WriteJSON(w, map[string]string{"status": "success"})
}

// ── Preferência de colunas (aba "Colunas" no modal de configurações) ──

func (ctx *HandlerContext) HandleGetColumnsSettings(w http.ResponseWriter, r *http.Request) {
	opts, err := ctx.columnFieldOptions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := ColumnsSettings(opts, false).Render(r.Context(), w); err != nil {
		slog.Error("render columns settings", "error", err)
	}
}

func (ctx *HandlerContext) HandlePostColumnsSettings(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	opts, err := ctx.columnFieldOptions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Só aceita campos que são candidatos reais (evita nomes arbitrários).
	selected := make(map[string]bool)
	submitted := r.Form["columns"]
	for _, f := range submitted {
		f = strings.TrimSpace(f)
		if f != "" {
			selected[f] = true
		}
	}
	valid := make(map[string]bool)
	for _, o := range opts {
		if selected[o.Field] {
			valid[o.Field] = true
		}
	}

	keys := make([]string, 0, len(valid))
	for k := range valid {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Desmarcar TODOS os campos é uma escolha explícita (nenhuma coluna). Um
	// valor vazio seria confundido com \"nunca configurado\", então gravamos o
	// marcador \"-\" (ver readAllowedColumns).
	val := strings.Join(keys, ",")
	if val == "" {
		val = "-"
	}
	if err := ctx.Store.SetSetting("db_columns", val); err != nil {
		http.Error(w, "erro ao salvar", http.StatusInternalServerError)
		return
	}

	// Re-renderiza com o estado recalculado e o banner de "salvo".
	opts, err = ctx.columnFieldOptions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := ColumnsSettings(opts, true).Render(r.Context(), w); err != nil {
		slog.Error("render columns settings", "error", err)
	}
}

// ── Sinônimos (aba "Sinônimos" no modal de configurações) ──

// HandleGetSynonymSettings devolve as regras salvas de sinônimos como JSON. O
// mapa embarcado de sinônimos continua como base — aqui só as entradas que o
// usuário adicionou/sobrescreveu (o lookup em internal/search dá prioridade às
// regras do usuário e cai para o embarcado quando não houver entrada).
func (ctx *HandlerContext) HandleGetSynonymSettings(w http.ResponseWriter, r *http.Request) {
	val := "[]"
	if raw, err := ctx.Store.GetSetting("synonyms_config"); err == nil && raw != "" {
		val = raw
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(val))
}

func (ctx *HandlerContext) HandlePostSynonymSettings(w http.ResponseWriter, r *http.Request) {
	var rules []domain.SynonymRule
	if err := json.NewDecoder(r.Body).Decode(&rules); err != nil {
		http.Error(w, "json invalido", http.StatusBadRequest)
		return
	}

	configJSON, err := json.Marshal(rules)
	if err != nil {
		http.Error(w, "erro ao processar dados", http.StatusInternalServerError)
		return
	}
	if err := ctx.Store.SetSetting("synonyms_config", string(configJSON)); err != nil {
		http.Error(w, "erro ao salvar", http.StatusInternalServerError)
		return
	}

	httputil.WriteJSON(w, map[string]string{"status": "success"})
}

func (ctx *HandlerContext) HandleApplyAutoTag(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "erro ao ler corpo da requisição", http.StatusBadRequest)
			return
		}
		if len(bodyBytes) > 0 {
			var incomingRules []domain.AutoTagRule
			if err := json.Unmarshal(bodyBytes, &incomingRules); err != nil {
				http.Error(w, "json invalido", http.StatusBadRequest)
				return
			}
			for i, rule := range incomingRules {
				if rule.Days <= 0 {
					http.Error(w, "os dias devem ser maiores que zero", http.StatusBadRequest)
					return
				}
				incomingRules[i].Tag = strings.TrimSpace(rule.Tag)
				incomingRules[i].Tag = strings.TrimPrefix(incomingRules[i].Tag, "#")
				if incomingRules[i].Tag == "" {
					http.Error(w, "a tag não pode ser vazia", http.StatusBadRequest)
					return
				}
			}
			configJSON, err := json.Marshal(incomingRules)
			if err != nil {
				http.Error(w, "erro ao processar dados", http.StatusInternalServerError)
				return
			}
			if err := ctx.Store.SetSetting("auto_tag_decay_config", string(configJSON)); err != nil {
				http.Error(w, "erro ao salvar", http.StatusInternalServerError)
				return
			}
		}
	}

	modified, err := notes.ApplyDecayTags(ctx.Store, ctx.Notes)
	if err != nil {
		http.Error(w, "erro ao aplicar tags: "+err.Error(), http.StatusInternalServerError)
		return
	}

	httputil.WriteJSON(w, map[string]interface{}{
		"status":   "success",
		"modified": modified,
	})
}
