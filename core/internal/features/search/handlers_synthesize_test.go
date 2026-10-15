package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// synthSources monta n fontes de teste com trechos identificáveis.
func synthSources(n int) []askSource {
	out := make([]askSource, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, askSource{
			Filename: fmt.Sprintf("notes/nota-%d.md", i),
			Name:     fmt.Sprintf("nota-%d", i),
			Excerpt:  fmt.Sprintf("trecho-%d sobre o assunto", i),
		})
	}
	return out
}

func runSynthesize(t *testing.T, ctx *HandlerContext, sources []askSource) string {
	t.Helper()

	body, err := json.Marshal(synthesizeRequest{Query: "o que é X?", Sources: sources})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/ask/synthesize", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	ctx.HandleAskSynthesize(rec, req)
	return rec.Body.String()
}

// TestHandleAskSynthesize_OllamaUsaContextoEParametrosCorretos cobre as correções
// de geração: o prompt leva o contexto montado (até askMaxSources trechos), o
// system prompt define o comportamento, num_ctx cobre o prompt inteiro (o default
// do Ollama é 2048 e truncaria) e num_predict deixa espaço para a resposta.
func TestHandleAskSynthesize_OllamaUsaContextoEParametrosCorretos(t *testing.T) {
	ctx := newTestContext(t)

	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/model", "/v1/models":
			http.NotFound(w, r) // força o caminho nativo do Ollama
		case "/api/generate":
			var err error
			captured, err = io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("ler body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
			flusher, _ := w.(http.Flusher)
			for _, line := range []string{
				`{"response":"A resposta","done":false}`,
				`{"response":" completa.","done":true}`,
			} {
				fmt.Fprintln(w, line)
				if flusher != nil {
					flusher.Flush()
				}
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	if err := ctx.Store.SetSetting("ollama_url", srv.URL); err != nil {
		t.Fatalf("SetSetting url: %v", err)
	}
	if err := ctx.Store.SetSetting("ollama_model", "llama3.2"); err != nil {
		t.Fatalf("SetSetting model: %v", err)
	}

	// 8 fontes: o prompt deve conter no máximo askMaxSources (as primeiras).
	out := runSynthesize(t, ctx, synthSources(8))

	// ── o stream SSE chega completo ──
	if !strings.Contains(out, "event: meta") {
		t.Errorf("stream sem evento meta (backend/modelo): %q", out)
	}
	if !strings.Contains(out, `"A resposta"`) || !strings.Contains(out, `" completa."`) {
		t.Errorf("tokens da IA não chegaram no stream: %q", out)
	}
	if !strings.Contains(out, "event: done") {
		t.Errorf("stream sem evento done: %q", out)
	}

	// ── o payload enviado ao backend ──
	var payload struct {
		Model   string `json:"model"`
		System  string `json:"system"`
		Prompt  string `json:"prompt"`
		Stream  bool   `json:"stream"`
		Options struct {
			Temperature float64 `json:"temperature"`
			NumPredict  int     `json:"num_predict"`
			NumCtx      int     `json:"num_ctx"`
		} `json:"options"`
	}
	if err := json.Unmarshal(captured, &payload); err != nil {
		t.Fatalf("payload não é JSON válido: %v (%s)", err, captured)
	}

	if payload.Model != "llama3.2" {
		t.Errorf("modelo enviado = %q, esperado llama3.2", payload.Model)
	}
	if payload.Options.NumCtx < 4096 {
		t.Errorf("num_ctx = %d; o default (2048) trunca o prompt e o modelo responde sem ver as primeiras notas", payload.Options.NumCtx)
	}
	if payload.Options.NumPredict < 400 {
		t.Errorf("num_predict = %d; era 250 e cortava respostas longas no meio", payload.Options.NumPredict)
	}
	if !strings.Contains(payload.System, "SOMENTE as notas") {
		t.Errorf("system prompt sem a instrução de usar apenas as notas: %q", payload.System)
	}
	if !strings.Contains(payload.Prompt, "o que é X?") {
		t.Errorf("prompt sem a pergunta: %q", payload.Prompt)
	}
	if !strings.Contains(payload.Prompt, "trecho-1") || !strings.Contains(payload.Prompt, "trecho-6") {
		t.Errorf("prompt não levou as %d fontes esperadas: %q", askMaxSources, payload.Prompt)
	}
	if strings.Contains(payload.Prompt, "trecho-7") {
		t.Errorf("prompt excedeu askMaxSources (%d) — levou fonte demais: %q", askMaxSources, payload.Prompt)
	}
}

// TestHandleAskSynthesize_ErroDoModeloVaiParaOStream é a regressão do sintoma
// "Gerando..." eterno: o Ollama devolve erro com HTTP 200 no meio do stream.
func TestHandleAskSynthesize_ErroDoModeloVaiParaOStream(t *testing.T) {
	ctx := newTestContext(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/model", "/v1/models":
			http.NotFound(w, r)
		case "/api/generate":
			w.WriteHeader(http.StatusOK)
			flusher, _ := w.(http.Flusher)
			fmt.Fprintln(w, `{"error":"model \"llama9\" not found, try pulling it first"}`)
			if flusher != nil {
				flusher.Flush()
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	ctx.Store.SetSetting("ollama_url", srv.URL)
	ctx.Store.SetSetting("ollama_model", "llama9")

	out := runSynthesize(t, ctx, synthSources(2))

	if !strings.Contains(out, "event: error") {
		t.Errorf("erro do modelo não foi sinalizado no stream: %q", out)
	}
	if !strings.Contains(out, "not found") {
		t.Errorf("mensagem do modelo não chegou ao stream: %q", out)
	}
	if !strings.Contains(out, "event: done") {
		t.Errorf("stream sem done após o erro (a UI ficaria gerando): %q", out)
	}
}

// TestHandleAskSynthesize_KoboldUsaChatCompletions garante o caminho OpenAI/KoboldCPP:
// system separado, sem instrução duplicada na mensagem do usuário e teto de tokens
// compatível com o prompt.
func TestHandleAskSynthesize_KoboldUsaChatCompletions(t *testing.T) {
	ctx := newTestContext(t)

	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/model":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"result":"llama3.2"}`)
		case "/v1/chat/completions":
			var err error
			captured, err = io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("ler body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
			flusher, _ := w.(http.Flusher)
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			if flusher != nil {
				flusher.Flush()
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	ctx.Store.SetSetting("ollama_url", srv.URL)
	ctx.Store.SetSetting("ollama_model", "llama3.2")

	out := runSynthesize(t, ctx, synthSources(2))
	if !strings.Contains(out, `"ok"`) {
		t.Errorf("token do backend OpenAI não chegou no stream: %q", out)
	}
	if !strings.Contains(out, "OpenAI compatível") {
		t.Errorf("evento meta deveria identificar o backend detectado: %q", out)
	}

	var payload struct {
		MaxTokens int `json:"max_tokens"`
		Messages  []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(captured, &payload); err != nil {
		t.Fatalf("payload não é JSON válido: %v (%s)", err, captured)
	}
	if payload.MaxTokens < 400 {
		t.Errorf("max_tokens = %d, esperado >= 400", payload.MaxTokens)
	}
	if len(payload.Messages) != 2 {
		t.Fatalf("esperado system + user, got %d mensagens", len(payload.Messages))
	}
	if payload.Messages[0].Role != "system" || !strings.Contains(payload.Messages[0].Content, "SOMENTE as notas") {
		t.Errorf("system prompt ausente/incorreto: %+v", payload.Messages[0])
	}
	if payload.Messages[1].Role != "user" || !strings.Contains(payload.Messages[1].Content, "trecho-1") {
		t.Errorf("mensagem do usuário sem o contexto: %+v", payload.Messages[1])
	}
	// A instrução de comportamento vive só no system (antes era duplicada com textos
	// diferentes em cada caminho).
	if strings.Contains(payload.Messages[1].Content, "Você é o assistente") {
		t.Errorf("instruções duplicadas na mensagem do usuário: %q", payload.Messages[1].Content)
	}
}
