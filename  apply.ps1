# ============================================================
# apply.ps1 — Aplica todas as alterações da busca semântica
# Execute na raiz do projeto: C:\Users\Giovani\Downloads\ton618
# ============================================================

$root = "C:\Users\Giovani\Downloads\ton618"

# ─── 1. Criar similarity.go ───
@"
package semantic

import "math"

// CosineSimilarity returns the cosine similarity between two float32 vectors.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
"@ | Set-Content "$root\internal\semantic\similarity.go" -Encoding UTF8

# ─── 2. Adicionar rota ───
$routes = Get-Content "$root\internal\api\routes.go" -Raw
$routes = $routes.Replace(
	'mux.HandleFunc("GET /api/graph/data", ctx.HandleGraphData)',
	'mux.HandleFunc("GET /api/graph/data", ctx.HandleGraphData)' + "`n" + '	mux.HandleFunc("POST /api/graph/query", ctx.HandleGraphQuery)'
)
Set-Content "$root\internal\api\routes.go" $routes -Encoding UTF8

# ─── 3. Adicionar handler HandleGraphQuery ───
$handlers = Get-Content "$root\internal\api\handlers.go" -Raw
$handlerCode = @'

func (ctx *HandlerContext) HandleGraphQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Query == "" {
		http.Error(w, "query required", http.StatusBadRequest)
		return
	}
	if ctx.Embed == nil {
		http.Error(w, "embedding not configured", http.StatusServiceUnavailable)
		return
	}
	queryVec, err := ctx.Embed.Embed(r.Context(), body.Query)
	if err != nil {
		http.Error(w, "embedding failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	allEmbeddings, err := ctx.Store.GetAllEmbeddings()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type nearest struct {
		Arquivo    string  `json:"arquivo"`
		Title      string  `json:"title"`
		Similarity float64 `json:"similarity"`
		X          float64 `json:"x"`
		Y          float64 `json:"y"`
	}
	var results []nearest
	for docID, nv := range allEmbeddings {
		if len(nv.Vector) == 0 {
			continue
		}
		doc, _ := ctx.Store.GetDocument(docID)
		if doc == nil || doc.Arquivo == "" {
			continue
		}
		sim := semantic.CosineSimilarity(queryVec, nv.Vector)
		if sim < 0.25 {
			continue
		}
		title := nv.Title
		if title == "" {
			parts := strings.Split(doc.Arquivo, "/")
			title = parts[len(parts)-1]
		}
		results = append(results, nearest{
			Arquivo:    doc.Arquivo,
			Title:      title,
			Similarity: sim,
			X:          nv.X,
			Y:          nv.Y,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})
	if len(results) > 20 {
		results = results[:20]
	}
	var qx, qy, totalWeight float64
	n := 5
	if len(results) < n {
		n = len(results)
	}
	for i := 0; i < n; i++ {
		weight := results[i].Similarity
		qx += results[i].X * weight
		qy += results[i].Y * weight
		totalWeight += weight
	}
	if totalWeight > 0 {
		qx /= totalWeight
		qy /= totalWeight
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"query_x":    qx,
		"query_y":    qy,
		"query_text": body.Query,
		"nearest":    results,
	})
}

'@

# Inserir antes de HandleLogin
$handlers = $handlers.Replace(
	"func (ctx *HandlerContext) HandleLogin",
	$handlerCode + "`n`nfunc (ctx *HandlerContext) HandleLogin"
)
Set-Content "$root\internal\api\handlers.go" $handlers -Encoding UTF8

# ─── 4. graph.html — Substituir header ───
$graph = Get-Content "$root\internal\template\graph.html" -Raw

# 4A — Header com campo de busca
$oldHeader = @'
    <div class="flex items-center justify-between mb-2" style="padding: 0 4px">
        <h1 class="text-sm font-black tracking-tight text-zinc-200">
            Mapa Semântico
        </h1>
        <div class="flex items-center gap-4">
            <span
                id="graph-status"
                class="text-[10px] text-zinc-600 font-medium"
                >—</span
            >
            <span class="text-[10px] text-zinc-700">✦ notas embedadas</span>
        </div>
    </div>
'@
$newHeader = @'
    <div class="flex items-center justify-between mb-2" style="padding: 0 4px">
        <h1 class="text-sm font-black tracking-tight text-zinc-200">
            Mapa Semântico
        </h1>
        <div class="flex items-center gap-3">
            <div class="flex items-center gap-1.5">
                <input
                    id="semantic-search-input"
                    type="text"
                    placeholder="Pergunte algo..."
                    class="w-36 md:w-48 px-3 py-1.5 text-[11px] bg-zinc-900/60 border border-zinc-800 rounded-lg text-zinc-200 placeholder-zinc-500 focus:outline-none focus:border-sky-500/50 focus:w-56 transition-all"
                    spellcheck="false"
                />
                <button
                    id="semantic-search-btn"
                    class="text-[11px] font-bold text-sky-400 hover:text-sky-300 transition-colors"
                >
                    Buscar
                </button>
            </div>
            <span
                id="graph-status"
                class="text-[10px] text-zinc-600 font-medium"
                >—</span
            >
            <span class="text-[10px] text-zinc-700 hidden md:inline">✦ embedadas</span>
        </div>
    </div>
'@
$graph = $graph.Replace($oldHeader, $newHeader)

# 4B — Highlight glow nos nós (após ctx.stroke();)
$graph = $graph.Replace(
	"ctx.stroke();`n                ctx.fillStyle = `"#f8fafc`";",
	"ctx.stroke();`n                // Highlight glow for semantic search results`n                if (n._highlighted) {`n                    ctx.beginPath();`n                    ctx.arc(px, py, r + 6, 0, Math.PI * 2);`n                    ctx.strokeStyle = `"#38bdf8`";`n                    ctx.globalAlpha = 0.5 + 0.3 * Math.sin(Date.now() / 800);`n                    ctx.lineWidth = 2 / scale;`n                    ctx.stroke();`n                    ctx.globalAlpha = 1;`n                }`n                ctx.fillStyle = `"#f8fafc`";"
)

# 4C — Query rendering (antes de ctx.restore();)
$graph = $graph.Replace(
	"            ctx.restore();",
	"            // ── Query point (if set) ──`n            if (queryPoint) {`n                ctx.strokeStyle = `"#38bdf8`";`n                ctx.globalAlpha = 0.3;`n                ctx.lineWidth = 1 / scale;`n                ctx.setLineDash([4 / scale, 4 / scale]);`n                var qn = queryPoint.nearest || [];`n                qn.forEach(function (nr) {`n                    ctx.beginPath();`n                    ctx.moveTo(queryPoint.x, queryPoint.y);`n                    ctx.lineTo(nr.x, nr.y);`n                    ctx.stroke();`n                });`n                ctx.setLineDash([]);`n                ctx.globalAlpha = 1;`n                var pulse = (Math.sin(Date.now() / 600) + 1) / 2;`n                var qr = 8 + pulse * 5;`n                ctx.beginPath();`n                ctx.arc(queryPoint.x, queryPoint.y, qr + 3, 0, Math.PI * 2);`n                ctx.fillStyle = `"rgba(56,189,248,0.15)`";`n                ctx.fill();`n                ctx.beginPath();`n                ctx.arc(queryPoint.x, queryPoint.y, qr, 0, Math.PI * 2);`n                ctx.fillStyle = `"#38bdf8`";`n                ctx.globalAlpha = 0.85;`n                ctx.fill();`n                ctx.globalAlpha = 1;`n                ctx.strokeStyle = `"#fff`";`n                ctx.lineWidth = 1.5 / scale;`n                ctx.stroke();`n                ctx.fillStyle = `"#fff`";`n                ctx.font = `"bold `" + (10 / scale) + `"px Inter,system-ui,sans-serif`";`n                ctx.textAlign = `"center`";`n                ctx.textBaseline = `"middle`";`n                ctx.fillText(`"?`", queryPoint.x, queryPoint.y);`n                if (scale >= 0.5 && queryPoint.text) {`n                    ctx.fillStyle = `"#38bdf8`";`n                    ctx.font = `"500 `" + (10 / scale) + `"px Inter,system-ui,sans-serif`";`n                    var label = queryPoint.text.length > 30 ? queryPoint.text.slice(0, 30) + `"...`" : queryPoint.text;`n                    ctx.fillText(label, queryPoint.x, queryPoint.y + qr + 14 / scale);`n                }`n            }`n            ctx.restore();"
)

# 4D — JS busca semântica (antes do </script>)
$graph = $graph.Replace(
	"        render();`n    });`n</script>",
	"        // ── Semantic search ──`n        var queryPoint = null;`n        var animationFrame = null;`n`n        document.getElementById(`"semantic-search-btn`").addEventListener(`"click`", doSemanticSearch);`n        document.getElementById(`"semantic-search-input`").addEventListener(`"keydown`", function(e) {`n            if (e.key === `"Enter`") doSemanticSearch();`n        });`n`n        async function doSemanticSearch() {`n            var input = document.getElementById(`"semantic-search-input`");`n            var q = input.value.trim();`n            if (!q) return;`n            var btn = document.getElementById(`"semantic-search-btn`");`n            var origText = btn.textContent;`n            btn.textContent = `"...`";`n            btn.style.pointerEvents = `"none`";`n            try {`n                var resp = await fetch(`"/api/graph/query`", {`n                    method: `"POST`",`n                    headers: { `"Content-Type`": `"application/json`" },`n                    body: JSON.stringify({ query: q }),`n                });`n                if (!resp.ok) { var err = await resp.text(); throw new Error(err); }`n                var data = await resp.json();`n                data.nearest = (data.nearest || []).filter(function(nr) {`n                    var found = nodes.find(function(gn) { return gn.id === nr.arquivo; });`n                    if (found) { nr.x = found.x; nr.y = found.y; return true; }`n                    return false;`n                });`n                queryPoint = { x: data.query_x || 0, y: data.query_y || 0, text: data.query_text || q, nearest: data.nearest };`n                nodes.forEach(function(n) { n._highlighted = data.nearest.some(function(nr) { return nr.arquivo === n.id; }); });`n                var r = canvas.getBoundingClientRect();`n                tx = r.width / 2 - queryPoint.x * scale;`n                ty = r.height / 2 - queryPoint.y * scale;`n                if (animationFrame) cancelAnimationFrame(animationFrame);`n                function animatePulse() { render(); animationFrame = requestAnimationFrame(animatePulse); }`n                animatePulse();`n                setTimeout(function() {`n                    if (animationFrame) { cancelAnimationFrame(animationFrame); animationFrame = null; }`n                    queryPoint = null;`n                    nodes.forEach(function(n) { n._highlighted = false; });`n                    render();`n                }, 5000);`n            } catch (err) { alert(`"Erro na busca: `" + err.message); }`n            finally { btn.textContent = origText; btn.style.pointerEvents = `"`"; }`n        }`n        render();`n    });`n</script>"
)

Set-Content "$root\internal\template\graph.html" $graph -Encoding UTF8

Write-Host "✅ Todas as alterações aplicadas!"
Write-Host "▶ Rode: go build -tags sqlite_fts5 ./cmd/server/ para compilar"
