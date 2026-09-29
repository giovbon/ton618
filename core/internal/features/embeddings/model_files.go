package embeddings

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"ton618/core/internal/core/db"
)

// modelRepoBase é a origem remota dos arquivos do modelo.
const modelRepoBase = "https://huggingface.co"

// downloadChunkSize é o bloco usado na cópia manual — permite atualizar o
// progresso do download sem depender de io.Copy.
const downloadChunkSize = 256 << 10 // 256 KiB

// errRangeInvalid indica que o arquivo .part não pôde ser retomado (servidor
// não aceitou o Range pedido). O chamador descarta o .part e recomeça.
var errRangeInvalid = errors.New("range de retomada inválido")

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

	// Progresso do download em andamento (arquivo atual) — exposto por Progress
	// e pelo endpoint /api/embeddings/model-status.
	curFile  string
	curBytes int64
	curTotal int64 // -1 quando o servidor não informa o tamanho
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

// ModelProgress descreve o andamento do download do modelo. É o payload do
// endpoint /api/embeddings/model-status, consumido pela UI para dar feedback
// granular ao usuário (arquivo atual + bytes + percentual).
type ModelProgress struct {
	Ready      bool   `json:"ready"`
	FilesDone  int    `json:"files_done"`
	FilesTotal int    `json:"files_total"`
	File       string `json:"file,omitempty"` // arquivo em download agora ("" ocioso)
	Bytes      int64  `json:"bytes"`          // bytes do arquivo atual
	TotalBytes int64  `json:"total_bytes"`    // tamanho do arquivo atual (-1 = desconhecido)
	Percent    int    `json:"percent"`        // 0-100 do arquivo atual
	LastError  string `json:"last_error,omitempty"`
}

// Progress devolve um snapshot do andamento (seguro para concorrência).
func (m *LocalModel) Progress() ModelProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p := ModelProgress{
		Ready:      m.ready,
		FilesDone:  m.readyN,
		FilesTotal: len(modelFiles),
		File:       m.curFile,
		Bytes:      m.curBytes,
		TotalBytes: m.curTotal,
	}
	if m.lastErr != nil {
		p.LastError = m.lastErr.Error()
	}
	if m.curTotal > 0 {
		pct := int(m.curBytes * 100 / m.curTotal)
		if pct < 0 {
			pct = 0
		}
		if pct > 100 {
			pct = 100
		}
		p.Percent = pct
	}
	return p
}

// setCurrent registra o arquivo/bytes em download no momento.
func (m *LocalModel) setCurrent(file string, bytes, total int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.curFile = file
	m.curBytes = bytes
	m.curTotal = total
}

// setProgressBytes atualiza só o contador de bytes (chamado a cada bloco).
func (m *LocalModel) setProgressBytes(n int64) {
	m.mu.Lock()
	m.curBytes = n
	m.mu.Unlock()
}

// clearCurrent marca o download como ocioso.
func (m *LocalModel) clearCurrent() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.curFile = ""
	m.curBytes = 0
	m.curTotal = 0
}

// Ensure garante que todos os arquivos do modelo existam localmente, baixando os
// que faltam da base remota (HuggingFace). É bloqueante: roda em goroutine no boot.
// Arquivos já presentes (não vazios) são preservados.
//
// RETOMADA: um download interrompido deixa <arquivo>.part no disco e o próximo
// boot continua de onde parou via HTTP Range (ver download). O .part nunca é
// promovido a definitivo sem o tamanho completo conferido.
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
			// Limpa um .part órfão de uma tentativa anterior (o arquivo definitivo
			// já está íntegro; manter o parcial só ocuparia espaço).
			os.Remove(dest + ".part")
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

	m.clearCurrent()
	m.setStatus(firstErr == nil && done == len(modelFiles), done, firstErr)
	return firstErr
}

// download baixa um arquivo e grava de forma atômica (grava em <arquivo>.part e
// renomeia ao final) — nunca expõe arquivo parcial.
func (m *LocalModel) download(ctx context.Context, file, dest string) error {
	err := m.downloadAttempt(ctx, file, dest)
	if errors.Is(err, errRangeInvalid) {
		// O .part não serve para retomar: descarta e baixa desde o início.
		os.Remove(dest + ".part")
		return m.downloadAttempt(ctx, file, dest)
	}
	return err
}

// downloadAttempt faz UMA tentativa de download, retomando o .part existente
// quando possível:
//
//   - .part vazio/inexistente → GET normal (200);
//   - .part com bytes → GET com "Range: bytes=<n>-" e exige 206 + Content-Range
//     começando exatamente em <n>; respostas 200 (servidor sem Range) reiniciam
//     do zero e 416 invalidam o parcial (errRangeInvalid);
//   - falha de rede no meio PRESERVA o .part, para retomar no próximo boot;
//   - o arquivo só é promovido quando o total confere com o Content-Length
//     (download truncado é descartado como definitivo, mas mantém o parcial).
func (m *LocalModel) downloadAttempt(ctx context.Context, file, dest string) error {
	tmp := dest + ".part"
	var offset int64
	if info, statErr := os.Stat(tmp); statErr == nil && info.Size() > 0 {
		offset = info.Size()
	}

	url := fmt.Sprintf("%s/%s/resolve/main/%s", m.baseURL, m.Repo(), file)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "ton618/1.0 (model-bootstrap)")
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	client := &http.Client{Timeout: 15 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err // .part preservado: a retomada continua possível
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusPartialContent && offset > 0:
		start, ok := contentRangeStart(resp.Header.Get("Content-Range"))
		if !ok || start != offset {
			return errRangeInvalid
		}
	case resp.StatusCode == http.StatusOK:
		offset = 0 // servidor ignorou o Range → recomeça do zero
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && offset > 0:
		return errRangeInvalid
	default:
		return fmt.Errorf("HTTP %d ao baixar %s", resp.StatusCode, file)
	}

	// total = bytes já no .part + o que o servidor ainda vai mandar.
	total := int64(-1)
	if resp.ContentLength >= 0 {
		total = offset + resp.ContentLength
	}
	m.setCurrent(file, offset, total)
	if offset > 0 {
		slog.Info("modelo de embeddings: retomando download", "file", file, "bytes", offset, "total", total)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	flags := os.O_CREATE | os.O_WRONLY
	if offset > 0 {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(tmp, flags, 0o644)
	if err != nil {
		return err
	}

	n, copyErr := m.copyWithProgress(f, resp.Body)
	if cerr := f.Close(); copyErr == nil {
		copyErr = cerr
	}
	if copyErr != nil {
		// .part preservado de propósito: permite retomar via Range.
		return copyErr
	}
	if n == 0 && offset == 0 {
		os.Remove(tmp)
		return fmt.Errorf("arquivo vazio: %s", file)
	}
	if total > 0 && offset+n != total {
		return fmt.Errorf("download incompleto de %s: %d de %d bytes (retomável)", file, offset+n, total)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return err
	}

	slog.Info("modelo de embeddings: arquivo baixado", "file", file, "bytes", offset+n)
	return nil
}

// copyWithProgress copia em blocos de downloadChunkSize, atualizando o contador
// de progresso a cada bloco (io.Copy não dá visibilidade ao andamento).
func (m *LocalModel) copyWithProgress(dst io.Writer, src io.Reader) (int64, error) {
	buf := make([]byte, downloadChunkSize)
	var written int64
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			if _, writeErr := dst.Write(buf[:n]); writeErr != nil {
				return written, writeErr
			}
			written += int64(n)
			m.setProgressBytes(written)
		}
		if readErr == io.EOF {
			return written, nil
		}
		if readErr != nil {
			return written, readErr
		}
	}
}

// contentRangeStart extrai o byte inicial de um header Content-Range
// (ex: "bytes 100-499/1234" → 100).
func contentRangeStart(v string) (int64, bool) {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "bytes ") {
		return 0, false
	}
	v = strings.TrimSpace(strings.TrimPrefix(v, "bytes "))
	if i := strings.IndexByte(v, '-'); i > 0 {
		v = v[:i]
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
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
