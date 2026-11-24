package system

import (
	"log/slog"
	"net/http"
	"sort"
	"strings"
)

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
	// valor vazio seria confundido com "nunca configurado", então gravamos o
	// marcador "-" (ver readAllowedColumns).
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
