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

// ── Configuração IA Local (Ollama / KoboldCPP / LM Studio / OpenAI) ───────────

// normalizeURL força IPv4 em substituição a localhost para evitar que conexões
// caiam em [::1] (IPv6) onde servidores locais como KoboldCPP normalmente não escutam.
func normalizeURL(u string) string {
	u = strings.TrimRight(strings.TrimSpace(u), "/")
	if strings.Contains(u, "://localhost:") {
		u = strings.Replace(u, "://localhost:", "://127.0.0.1:", 1)
	} else if strings.HasSuffix(u, "://localhost") {
		u = strings.Replace(u, "://localhost", "://127.0.0.1", 1)
	}
	return u
}

// HandleGetOllamaSettings retorna a URL e o modelo configurados.
// GET /api/settings/ollama
func (ctx *HandlerContext) HandleGetOllamaSettings(w http.ResponseWriter, r *http.Request) {
	url, _ := ctx.Store.GetSetting("ollama_url")
	model, _ := ctx.Store.GetSetting("ollama_model")
	if url == "" {
		url = "http://127.0.0.1:11434"
	}
	if model == "" {
		model = "llama3.2"
	}
	httputil.WriteJSON(w, map[string]string{
		"url":   url,
		"model": model,
	})
}

// HandlePostOllamaSettings salva URL e modelo e testa a conexão (suporta Ollama e KoboldCPP).
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
	body.URL = normalizeURL(body.URL)
	body.Model = strings.TrimSpace(body.Model)
	if body.URL == "" {
		http.Error(w, "url é obrigatória", http.StatusBadRequest)
		return
	}

	// Testa a conexão (detecta automaticamente Ollama, KoboldCPP ou OpenAI-compatible)
	ok, detectedModel, errMsg := pingAI(body.URL, body.Model)
	if ok && body.Model == "" && detectedModel != "" {
		body.Model = detectedModel
	}
	if body.Model == "" {
		body.Model = "default"
	}

	// Persiste as configurações
	ctx.Store.SetSetting("ollama_url", body.URL)
	ctx.Store.SetSetting("ollama_model", body.Model)

	httputil.WriteJSON(w, map[string]interface{}{
		"ok":    ok,
		"error": errMsg,
		"url":   body.URL,
		"model": body.Model,
	})
}

// pingAI verifica se o servidor local está acessível (suporta KoboldCPP, Ollama e LM Studio/OpenAI).
func pingAI(baseURL, model string) (bool, string, string) {
	client := &http.Client{Timeout: 6 * time.Second}

	// 1. Tenta API nativa do KoboldCPP (GET /api/v1/model)
	if resp, err := client.Get(baseURL + "/api/v1/model"); err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			var kModel struct {
				Result string `json:"result"`
			}
			if json.NewDecoder(resp.Body).Decode(&kModel) == nil && kModel.Result != "" {
				return true, kModel.Result, ""
			}
			return true, model, ""
		}
	}

	// 2. Tenta API do Ollama (GET /api/tags)
	if resp, err := client.Get(baseURL + "/api/tags"); err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return true, model, ""
		}
	}

	// 3. Tenta API compatível OpenAI (GET /v1/models)
	if resp, err := client.Get(baseURL + "/v1/models"); err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return true, model, ""
		}
	}

	// 4. Fallback: chamada mínima de teste ao Ollama /api/generate
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
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return true, model, ""
		}
	}

	return false, "", fmt.Sprintf("Não foi possível conectar ao servidor em %s (verifique se está rodando e se a porta está correta)", baseURL)
}

// ── Síntese via IA Local (SSE streaming) ───────────────────────────────────────

type synthesizeRequest struct {
	Query   string      `json:"query"`
	Sources []askSource `json:"sources"`
}

// Parâmetros de geração da síntese.
//
// synthesizeNumCtx importa: o default do Ollama é 2048 tokens e, quando o prompt
// não cabe, ele descarta o começo SILENCIOSAMENTE — o modelo responderia sem ver
// as primeiras notas. 4096 cobre o contexto montado em handlers_ask.go
// (askMaxSources × askExcerptChars ≈ 1.500 tokens) sem exagerar no KV cache.
//
// synthesizeNumPredict era 250: o modelo recebia instrução de responder em
// "2 a 4 frases" e tinha teto de ~250 tokens, então respostas mais longas eram
// cortadas no meio.
const (
	synthesizeNumPredict = 800
	synthesizeNumCtx     = 4096
	synthesizeTimeout    = 180 * time.Second
)

// HandleAskSynthesize chama o modelo local (Ollama ou KoboldCPP/OpenAI) e faz stream
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

	baseURL, _ := ctx.Store.GetSetting("ollama_url")
	model, _ := ctx.Store.GetSetting("ollama_model")
	if baseURL == "" {
		http.Error(w, "Servidor de IA não configurado. Configure em Configurações → IA.", http.StatusServiceUnavailable)
		return
	}
	baseURL = normalizeURL(baseURL)

	// Monta o contexto com os trechos das notas
	var contextParts []string
	for i, src := range req.Sources {
		if i >= askMaxSources {
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

	// Uma única definição de comportamento, usada pelos dois backends: o formato de
	// chat (KoboldCPP/OpenAI) recebe isto como mensagem de sistema e o Ollama no
	// campo `system` (que passa pela template do modelo).
	systemPrompt := `Você é o assistente de notas do TON-618.
Responda em português brasileiro, de forma direta e organizada.
Use SOMENTE as notas fornecidas; se a resposta não estiver nelas, diga que não encontrou nas notas.
Não invente fatos, nomes ou números. Prefira de 4 a 8 frases (ou uma lista curta quando fizer sentido).`

	userContent := fmt.Sprintf("Notas:\n%s\n\nPergunta: %s", contextText, req.Query)

	// Detecta se o backend é KoboldCPP / OpenAI ou Ollama
	client := &http.Client{Timeout: synthesizeTimeout}
	isKoboldOrOpenAI := false

	// Checa se o endpoint OpenAI /v1/chat/completions ou Kobold existe
	if checkResp, err := client.Get(baseURL + "/api/v1/model"); err == nil && checkResp.StatusCode == http.StatusOK {
		checkResp.Body.Close()
		isKoboldOrOpenAI = true
	} else if checkResp, err := client.Get(baseURL + "/v1/models"); err == nil && checkResp.StatusCode == http.StatusOK {
		checkResp.Body.Close()
		isKoboldOrOpenAI = true
	}

	// Configura Headers para Server-Sent Events (SSE)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Connection", "keep-alive")

	flusher, hasFlusher := w.(http.Flusher)

	// Primeiro frame: a UI mostra qual backend/modelo está respondendo (o usuário
	// pode ter os dois configurados e não tem como saber qual foi usado).
	provider := "Ollama"
	if isKoboldOrOpenAI {
		provider = "OpenAI compatível"
	}
	sseWrite(w, hasFlusher, flusher, "meta", mustJSON(map[string]string{
		"provider": provider,
		"model":    model,
		"sources":  fmt.Sprintf("%d", len(contextParts)),
	}))

	sentDone := false
	emitDone := func() {
		if sentDone {
			return
		}
		sentDone = true
		sseWrite(w, hasFlusher, flusher, "done", "{}")
	}

	if isKoboldOrOpenAI {
		// ── Formato OpenAI / KoboldCPP (/v1/chat/completions) ──
		openAIPayload := map[string]interface{}{
			"model": model,
			"messages": []map[string]string{
				{"role": "system", "content": systemPrompt},
				{"role": "user", "content": userContent},
			},
			"stream":      true,
			"max_tokens":  synthesizeNumPredict,
			"temperature": 0.3,
		}
		payloadBytes, _ := json.Marshal(openAIPayload)
		resp, err := client.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewReader(payloadBytes))
		if err != nil {
			slog.Error("ia: erro ao conectar ao kobold/openai", "error", err)
			http.Error(w, "Erro ao conectar à IA: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			http.Error(w, fmt.Sprintf("Servidor retornou HTTP %d: %s", resp.StatusCode, string(body)), http.StatusBadGateway)
			return
		}

		w.WriteHeader(http.StatusOK)
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				emitDone()
				break
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
				Error json.RawMessage `json:"error"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if msg := jsonErrorMessage(chunk.Error); msg != "" {
				slog.Warn("ia: erro no stream", "error", msg)
				sseWrite(w, hasFlusher, flusher, "error", mustJSON(map[string]string{"message": msg}))
				break
			}
			if len(chunk.Choices) > 0 {
				if token := chunk.Choices[0].Delta.Content; token != "" {
					sseToken(w, hasFlusher, flusher, token)
				}
			}
		}
		emitDone()
		return
	}

	// ── Formato Ollama (/api/generate) ──
	// `system` passa pela template do modelo (llama3.2 tem chat template própria);
	// `num_ctx` precisa cobrir o prompt inteiro (ver constantes no topo).
	ollamaPayload := map[string]interface{}{
		"model":  model,
		"system": systemPrompt,
		"prompt": userContent,
		"stream": true,
		"options": map[string]interface{}{
			"temperature": 0.3,
			"num_predict": synthesizeNumPredict,
			"num_ctx":     synthesizeNumCtx,
		},
	}
	payloadBytes, _ := json.Marshal(ollamaPayload)

	ollamaResp, err := client.Post(baseURL+"/api/generate", "application/json", bytes.NewReader(payloadBytes))
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

	w.WriteHeader(http.StatusOK)

	scanner := bufio.NewScanner(ollamaResp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var chunk struct {
			Response string          `json:"response"`
			Error    json.RawMessage `json:"error"`
			Done     bool            `json:"done"`
		}
		if err := json.Unmarshal(line, &chunk); err != nil {
			continue
		}
		// O Ollama devolve erros (modelo inexistente, falta de memória…) com HTTP 200
		// no meio do stream; sem tratar isso a UI ficava em "Gerando..." para sempre.
		if msg := jsonErrorMessage(chunk.Error); msg != "" {
			slog.Warn("ollama: erro no stream", "error", msg)
			sseWrite(w, hasFlusher, flusher, "error", mustJSON(map[string]string{"message": msg}))
			break
		}
		if chunk.Response != "" {
			sseToken(w, hasFlusher, flusher, chunk.Response)
		}
		if chunk.Done {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		slog.Warn("ollama: stream interrompido", "error", err)
	}
	emitDone()
}

// ── Utilidades de Server-Sent Events ──

// sseWrite escreve um frame de SSE (evento opcional + data) e libera o buffer.
func sseWrite(w io.Writer, hasFlusher bool, flusher http.Flusher, event, data string) {
	if event != "" {
		fmt.Fprintf(w, "event: %s\n", event)
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	if hasFlusher {
		flusher.Flush()
	}
}

// sseToken escreve um pedaço do texto gerado. O JSON evita que aspas ou quebras de
// linha no token quebrem o frame.
func sseToken(w io.Writer, hasFlusher bool, flusher http.Flusher, token string) {
	b, err := json.Marshal(token)
	if err != nil {
		return
	}
	sseWrite(w, hasFlusher, flusher, "", string(b))
}

// mustJSON serializa para JSON (usado nos frames de controle).
func mustJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// jsonErrorMessage extrai uma mensagem legível do campo `error` de uma resposta,
// que aparece como string, {"message": "..."} ou {"error": {"message": "..."}}.
func jsonErrorMessage(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		for _, key := range []string{"message", "error", "detail"} {
			if v, ok := obj[key]; ok {
				if msg := jsonErrorMessage(v); msg != "" {
					return msg
				}
			}
		}
	}
	return strings.TrimSpace(string(raw))
}
