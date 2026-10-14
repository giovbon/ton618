package search

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"ton618/core/internal/httputil"
)

// ── Configuração Ollama ───────────────────────────────────────────────────────

// HandleGetOllamaSettings retorna a URL e o modelo Ollama configurados.
// GET /api/settings/ollama
func (ctx *HandlerContext) HandleGetOllamaSettings(w http.ResponseWriter, r *http.Request) {
	url, _ := ctx.Store.GetSetting("ollama_url")
	model, _ := ctx.Store.GetSetting("ollama_model")
	if url == "" {
		url = "http://localhost:11434"
	}
	if model == "" {
		model = "llama3.2"
	}
	httputil.WriteJSON(w, map[string]string{
		"url":   url,
		"model": model,
	})
}

// HandlePostOllamaSettings salva URL e modelo Ollama e testa a conexão.
// POST /api/settings/ollama   (JSON: {"url":"...", "model":"..."})
func (ctx *HandlerContext) HandlePostOllamaSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL   string `json:"url"`
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "json inválido", http.StatusBadRequest)
		return
	}
	body.URL = strings.TrimRight(strings.TrimSpace(body.URL), "/")
	body.Model = strings.TrimSpace(body.Model)
	if body.URL == "" || body.Model == "" {
		http.Error(w, "url e model são obrigatórios", http.StatusBadRequest)
		return
	}

	// Persiste imediatamente para que testes subsequentes já leiam
	ctx.Store.SetSetting("ollama_url", body.URL)
	ctx.Store.SetSetting("ollama_model", body.Model)

	// Testa a conexão fazendo uma chamada mínima ao Ollama
	ok, errMsg := pingOllama(body.URL, body.Model)
	httputil.WriteJSON(w, map[string]interface{}{
		"ok":    ok,
		"error": errMsg,
		"url":   body.URL,
		"model": body.Model,
	})
}

// pingOllama verifica se o Ollama está acessível e o modelo está disponível.
func pingOllama(baseURL, model string) (bool, string) {
	client := &http.Client{Timeout: 8 * time.Second}

	payload := map[string]interface{}{
		"model":  model,
		"prompt": "ok",
		"stream": false,
		"options": map[string]interface{}{
			"num_predict": 1,
		},
	}
	b, _ := json.Marshal(payload)
	resp, err := client.Post(baseURL+"/api/generate", "application/json", bytes.NewReader(b))
	if err != nil {
		return false, fmt.Sprintf("Não foi possível conectar ao Ollama em %s: %v", baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Sprintf("Ollama retornou HTTP %d — verifique se o modelo '%s' está instalado (ollama pull %s)", resp.StatusCode, model, model)
	}
	return true, ""
}

// ── Síntese via Ollama (SSE streaming) ───────────────────────────────────────

type synthesizeRequest struct {
	Query   string      `json:"query"`
	Sources []askSource `json:"sources"`
}

// HandleAskSynthesize chama o Ollama com os trechos do /api/ask e faz stream
// da resposta gerada para o browser via Server-Sent Events.
// POST /api/ask/synthesize
func (ctx *HandlerContext) HandleAskSynthesize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req synthesizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "json inválido: "+err.Error(), http.StatusBadRequest)
		return
	}
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		http.Error(w, "query obrigatória", http.StatusBadRequest)
		return
	}

	ollamaURL, _ := ctx.Store.GetSetting("ollama_url")
	model, _ := ctx.Store.GetSetting("ollama_model")
	if ollamaURL == "" || model == "" {
		http.Error(w, "Ollama não configurado. Configure em Configurações → IA.", http.StatusServiceUnavailable)
		return
	}
	ollamaURL = strings.TrimRight(ollamaURL, "/")

	// Monta o contexto com os trechos das notas
	var contextParts []string
	for i, src := range req.Sources {
		if i >= 5 {
			break
		}
		name := src.Name
		if name == "" {
			name = src.Filename
		}
		excerpt := strings.TrimSpace(src.Excerpt)
		if excerpt == "" {
			continue
		}
		contextParts = append(contextParts, fmt.Sprintf("[Nota: %s]\n%s", name, excerpt))
	}
	if len(contextParts) == 0 {
		http.Error(w, "nenhum trecho disponível para síntese", http.StatusBadRequest)
		return
	}
	contextText := strings.Join(contextParts, "\n\n")

	prompt := fmt.Sprintf(`Você é o assistente inteligente de notas do TON-618.
Responda à pergunta do usuário em 2 a 4 frases claras e diretas em português brasileiro, baseando-se estritamente nas notas fornecidas abaixo.
Não invente informações que não estejam nas notas.

Notas:
%s

Pergunta: %s

Resposta:`, contextText, req.Query)

	ollamaPayload := map[string]interface{}{
		"model":  model,
		"prompt": prompt,
		"stream": true,
		"options": map[string]interface{}{
			"temperature": 0.3,
			"num_predict": 200,
		},
	}
	payloadBytes, _ := json.Marshal(ollamaPayload)

	client := &http.Client{Timeout: 60 * time.Second}
	ollamaResp, err := client.Post(ollamaURL+"/api/generate", "application/json", bytes.NewReader(payloadBytes))
	if err != nil {
		slog.Error("ollama: falha na conexão", "error", err)
		http.Error(w, "Erro ao conectar ao Ollama: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer ollamaResp.Body.Close()

	if ollamaResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(ollamaResp.Body, 1024))
		http.Error(w, fmt.Sprintf("Ollama retornou HTTP %d: %s", ollamaResp.StatusCode, string(body)), http.StatusBadGateway)
		return
	}

	// Configura SSE no response
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // desativa buffer do nginx/proxy
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, hasFlusher := w.(http.Flusher)

	// Lê a resposta do Ollama linha por linha e faz stream para o browser
	scanner := bufio.NewScanner(ollamaResp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var chunk struct {
			Response string `json:"response"`
			Done     bool   `json:"done"`
		}
		if err := json.Unmarshal(line, &chunk); err != nil {
			continue
		}
		if chunk.Response != "" {
			// Envia token como SSE data
			token, _ := json.Marshal(chunk.Response)
			fmt.Fprintf(w, "data: %s\n\n", token)
			if hasFlusher {
				flusher.Flush()
			}
		}
		if chunk.Done {
			fmt.Fprintf(w, "event: done\ndata: {}\n\n")
			if hasFlusher {
				flusher.Flush()
			}
			break
		}
	}
	if err := scanner.Err(); err != nil {
		slog.Warn("ollama: erro no scanner da resposta", "error", err)
	}
}
