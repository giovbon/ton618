package embeddings

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ton618/core/internal/core/db"
)

// modelRepoBase é a origem remota dos arquivos do modelo.
const modelRepoBase = "https://huggingface.co"

// modelFiles é a lista de arquivos do repositório exigidos pelo Transformers.js
// no browser (config + tokenizer + ONNX quantizado q8). Mantida em sincronia com
// o web/download_model.js — mesma semântica (download + uso local).
var modelFiles = []string{
	"config.json",
	"special_tokens_map.json",
	"tokenizer.json",
	"tokenizer_config.json",
	"onnx/model_quantized.onnx",
}

// LocalModel baixa e serve localmente o modelo de embeddings usado pelo browser
// (Transformers.js no Web Worker). O download acontece uma única vez, no boot,
// gravando no volume persistente (STATE_DIR/models). Depois de pronto, o browser
// carrega o modelo do próprio servidor (GET /models/...) em vez do CDN do
// HuggingFace — o CDN continua apenas como fallback (allowRemoteModels=true no worker).
//
// Estrutura no disco (espelha a URL /models/<repo>/<arquivo>):
//
//	<ModelDir>/
//	└── Xenova/
//	    └── paraphrase-multilingual-MiniLM-L12-v2/
//	        ├── config.json
//	        ├── tokenizer.json
//	        └── onnx/model_quantized.onnx
type LocalModel struct {
	dir     string // diretório raiz (STATE_DIR/models)
	baseURL string // base remota (huggingface.co) — sobreponível em testes

	mu      sync.RWMutex
	ready   bool
	readyN  int // arquivos presentes localmente
	lastErr error
}

// NewLocalModel cria o gerenciador que grava em dir (ex.: <StateDir>/models).
func NewLocalModel(dir string) *LocalModel {
	return &LocalModel{
		dir:     dir,
		baseURL: modelRepoBase,
	}
}

// Repo retorna o repositório HuggingFace do modelo. Fonte única no backend:
// db.EmbeddingModelName (deve permanecer em sincronia com o semantic-worker.js).
func (m *LocalModel) Repo() string { return db.EmbeddingModelName }

// repoDir retorna o diretório que guarda os arquivos do modelo (raiz + repositório).
func (m *LocalModel) repoDir() string {
	return filepath.Join(m.dir, filepath.FromSlash(m.Repo()))
}

// localFile retorna o caminho absoluto de um arquivo do modelo.
func (m *LocalModel) localFile(file string) string {
	return filepath.Join(m.repoDir(), filepath.FromSlash(file))
}

// IsReady indica se todos os arquivos do modelo já estão disponíveis localmente.
func (m *LocalModel) IsReady() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ready
}

// Status expõe o estado do download local (útil para logs e debug).
func (m *LocalModel) Status() (ready bool, readyCount int, total int, lastErr error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ready, m.readyN, len(modelFiles), m.lastErr
}

// Ensure garante que todos os arquivos do modelo existam localmente, baixando os
// que faltam da base remota (HuggingFace). É bloqueante: roda em goroutine no boot.
// Arquivos já presentes (não vazios) são preservados — permite retomar download
// interrompido sem re-baixar tudo.
func (m *LocalModel) Ensure(ctx context.Context) error {
	if err := os.MkdirAll(m.repoDir(), 0o755); err != nil {
		m.setStatus(false, 0, err)
		return err
	}

	done := 0
	var firstErr error
	for _, file := range modelFiles {
		if ctx.Err() != nil {
			firstErr = ctx.Err()
			break
		}

		dest := m.localFile(file)
		if info, err := os.Stat(dest); err == nil && info.Size() > 0 {
			done++
			continue
		}

		if err := m.download(ctx, file, dest); err != nil {
			slog.Warn("modelo de embeddings: falha ao baixar", "file", file, "error", err)
			if firstErr == nil {
				firstErr = err
			}
			continue // tenta os demais; ready só é marcado com todos presentes
		}
		done++
	}

	m.setStatus(firstErr == nil && done == len(modelFiles), done, firstErr)
	return firstErr
}

// download baixa um arquivo do repositório e grava de forma atômica
// (grava em <arquivo>.part e renomeia ao final) — nunca expõe arquivo parcial.
func (m *LocalModel) download(ctx context.Context, file, dest string) error {
	url := fmt.Sprintf("%s/%s/resolve/main/%s", m.baseURL, m.Repo(), file)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "ton618/1.0 (model-bootstrap)")

	client := &http.Client{Timeout: 15 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d ao baixar %s", resp.StatusCode, file)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}

	n, copyErr := io.Copy(f, resp.Body)
	if cerr := f.Close(); copyErr == nil {
		copyErr = cerr
	}
	if copyErr != nil {
		os.Remove(tmp)
		return copyErr
	}
	if n == 0 {
		os.Remove(tmp)
		return fmt.Errorf("arquivo vazio: %s", file)
	}

	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return err
	}

	slog.Info("modelo de embeddings: arquivo baixado", "file", file, "bytes", n)
	return nil
}

func (m *LocalModel) setStatus(ready bool, readyN int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ready = ready
	m.readyN = readyN
	m.lastErr = err
}

// ── Servir os arquivos do modelo (GET /models/*, público) ────────────

// ServeHTTP entrega os arquivos do modelo a partir do disco local. A URL esperada é
// /models/<repo>/<arquivo> (o mesmo caminho que o Transformers.js monta a partir de
// env.localModelPath="/models/"). Ex.: /models/Xenova/.../onnx/model_quantized.onnx.
func (m *LocalModel) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rel := strings.TrimPrefix(r.URL.Path, "/")
	cleaned := filepath.Clean(filepath.FromSlash(rel))
	// Bloqueia tentativa de path traversal (nunca deve sair de m.dir)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") ||
		filepath.IsAbs(cleaned) {
		http.NotFound(w, r)
		return
	}

	full := filepath.Join(m.dir, cleaned)
	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	// Modelo é imutável por versão (o repo id faz parte do path). Cache longo no
	// browser + CacheStorage do Transformers.js evitam re-download a cada uso.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, full)
}
