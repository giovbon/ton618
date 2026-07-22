package embeddings

import (
	"context"
	"io"
	"log/slog"
	"net/http"

	"ton618/core/internal/core/staticver"
	"ton618/core/internal/httputil"
	"ton618/core/web/layout"
)

// HandleSemanticMap retorna todos os pontos 2D do mapa semântico.
// GET /api/embeddings/map
func (ctx *HandlerContext) HandleSemanticMap(w http.ResponseWriter, r *http.Request) {
	points, err := ctx.Store.GetSemanticMapPoints()
	if err != nil {
		slog.Error("GetSemanticMapPoints", "error", err)
		httputil.WriteJSON(w, map[string]interface{}{
			"error":  err.Error(),
			"points": []interface{}{},
			"count":  0,
		})
		return
	}

	httputil.WriteJSON(w, map[string]interface{}{
		"points": points,
		"count":  len(points),
	})
}

// HandleSemanticMapPage renderiza a página HTML do mapa semântico.
// GET /mapa-semantico
func (hctx *HandlerContext) HandleSemanticMapPage(w http.ResponseWriter, r *http.Request) {
	// Marca como full-width no contexto para o Layout não aplicar max-w-4xl
	fullCtx := context.WithValue(r.Context(), "is_full_width", true)
	layout.Layout("Mapa Semântico — TON-618", false, mapPageComponent{}).Render(fullCtx, w)
}

// mapPage é o componente templ que contém apenas o mapa full-page.
// Usamos Raw para evitar criar um arquivo .templ separado.
func mapPage() mapPageComponent {
	return mapPageComponent{}
}

type mapPageComponent struct{}

func (mapPageComponent) Render(ctx context.Context, w io.Writer) error {
	_, err := io.WriteString(w, mapPageHTML())
	return err
}

// mapPageHTML retorna o conteúdo da página do mapa (full-width, sem bordas, sem legenda de clusters).
func mapPageHTML() string {
	return `<main class="w-full px-0">
    <div x-data="semanticMapState()" class="relative">
        <div id="semantic-map-container"
            class="relative w-full h-[calc(100vh-57px)] bg-zinc-950 overflow-hidden select-none"
            @mousedown="startPan"
            @mousemove="doPan"
            @mouseup="endPan"
            @mouseleave="endPan"
            @wheel.prevent="onWheel">
            <svg id="semantic-map-svg"
                class="w-full h-full"
                :style="viewportStyle">
            </svg>
            <div x-show="tooltip.show"
                :style="'left: ' + tooltip.x + 'px; top: ' + tooltip.y + 'px'"
                class="absolute pointer-events-none bg-zinc-900/80 border border-zinc-700/80 rounded-lg px-3 py-1.5 text-xs font-medium text-zinc-200 shadow-xl z-10 backdrop-blur-md -translate-x-1/2 -translate-y-full -mt-3"
                x-transition:enter="transition ease-out duration-100"
                x-transition:enter-start="opacity-0 scale-95"
                x-transition:enter-end="opacity-100 scale-100"
                x-transition:leave="transition ease-in duration-75"
                x-transition:leave-start="opacity-100 scale-100"
                x-transition:leave-end="opacity-0 scale-95">
                <span x-text="tooltip.text"></span>
            </div>
            <div class="absolute top-4 right-4 flex items-center gap-2 z-10">
                <span class="text-xs text-zinc-500 bg-zinc-950/80 backdrop-blur-sm px-3 py-1.5 rounded-lg border border-zinc-800/50" x-text="count + ' notas'"></span>
                <button @click="reload()" class="px-3 py-1.5 text-xs font-bold tracking-wide rounded-lg bg-zinc-800/40 border border-zinc-700/30 text-zinc-400 hover:text-white hover:border-zinc-600 transition-all cursor-pointer backdrop-blur-sm">↻</button>
                <span class="w-px h-6 bg-zinc-800/50"></span>
                <button @click="zoomIn()" class="w-8 h-8 bg-zinc-900/80 border border-zinc-700/50 rounded-lg text-zinc-400 hover:text-white hover:bg-zinc-800 transition-all cursor-pointer flex items-center justify-center text-lg font-bold leading-none">+</button>
                <button @click="zoomOut()" class="w-8 h-8 bg-zinc-900/80 border border-zinc-700/50 rounded-lg text-zinc-400 hover:text-white hover:bg-zinc-800 transition-all cursor-pointer flex items-center justify-center text-lg font-bold leading-none">−</button>
                <button @click="resetView()" class="w-8 h-8 bg-zinc-900/80 border border-zinc-700/50 rounded-lg text-zinc-400 hover:text-white hover:bg-zinc-800 transition-all cursor-pointer flex items-center justify-center text-xs font-bold">⟲</button>
            </div>
            <div x-show="loading" class="absolute inset-0 flex items-center justify-center bg-zinc-950/70 backdrop-blur-sm z-20">
                <div class="text-sm text-zinc-400 animate-pulse">Carregando mapa...</div>
            </div>
        </div>
    </div>
</main>
<script src="` + staticver.URL("/static/semantic-map.js") + `"></script>`
}
