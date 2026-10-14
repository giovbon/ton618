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

	// Detecta se o backend é KoboldCPP / OpenAI ou Ollama
	client := &http.Client{Timeout: 90 * time.Second}
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

	if isKoboldOrOpenAI {
		// ── Formato OpenAI / KoboldCPP (/v1/chat/completions) ──
		openAIPayload := map[string]interface{}{
			"model": model,
			"messages": []map[string]string{
				{"role": "system", "content": "Você é o assistente inteligente de notas do TON-618. Responda estritamente com base nas notas fornecidas em português brasileiro."},
				{"role": "user", "content": prompt},
			},
			"stream":      true,
			"max_tokens":  250,
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
				fmt.Fprintf(w, "event: done\ndata: {}\n\n")
				if hasFlusher {
					flusher.Flush()
				}
				break
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err == nil && len(chunk.Choices) > 0 {
				token := chunk.Choices[0].Delta.Content
				if token != "" {
					bToken, _ := json.Marshal(token)
					fmt.Fprintf(w, "data: %s\n\n", bToken)
					if hasFlusher {
						flusher.Flush()
					}
				}
			}
		}
		return
	}

	// ── Formato Ollama (/api/generate) ──
	ollamaPayload := map[string]interface{}{
		"model":  model,
		"prompt": prompt,
		"stream": true,
		"options": map[string]interface{}{
			"temperature": 0.3,
			"num_predict": 250,
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
			Response string `json:"response"`
			Done     bool   `json:"done"`
		}
		if err := json.Unmarshal(line, &chunk); err != nil {
			continue
		}
		if chunk.Response != "" {
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
}
