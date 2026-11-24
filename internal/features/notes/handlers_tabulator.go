package notes

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"ton618/core/internal/core/domain"
	"ton618/core/internal/processor"
)

// HandleTabulator renderiza o editor em tela cheia de uma nota Tabulator
// (planilha de inserção de dados). É o análogo do /drawing e /mindmap.
func (ctx *HandlerContext) HandleTabulator(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	filename, redirected := ensureNoteFilename(w, r, "/tabulator")
	if redirected {
		return
	}

	nd, _ := ctx.loadNoteData(filename)

	if !nd.Exists {
		// Nota nova: nasce com o frontmatter de tipo e uma tabela mínima pronta
		// para digitar.
		nd.Content = "---\ntype: tabulator\n---\n" + processor.TabulatorDefaultBody + "\n"
	} else {
		noteType := domain.DetectNoteTypeFromContent(nd.FileTags, nd.Content, filename)
		if redirectIfWrongEditor(w, r, noteType, "/tabulator", filename) {
			return
		}
	}

	data := buildEditorData("Tabulator - "+domain.DisplayName(filename), filename, nd)
	Tabulator(data).Render(r.Context(), w)
}

// HandleTabulatorExport converte uma nota Tabulator em CSV e faz o download.
// O CSV inclui BOM UTF-8 para o Excel abrir os acentos corretamente.
func (ctx *HandlerContext) HandleTabulatorExport(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("file")
	if strings.TrimSpace(raw) == "" {
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}
	filename := NoteFilename(raw)

	content, err := ctx.Store.GetNote(filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if strings.TrimSpace(content) == "" {
		http.Error(w, "nota não encontrada", http.StatusNotFound)
		return
	}

	csvData, ok := processor.TabulatorCSVFromNote(content)
	if !ok {
		http.Error(w, "a nota não é um Tabulator válido", http.StatusUnprocessableEntity)
		return
	}

	base := domain.DisplayName(filename)
	if base == "" {
		base = "tabulator"
	}
	base = strings.Map(func(r rune) rune {
		switch r {
		case '"', '\\', '/', '\r', '\n':
			return '-'
		}
		return r
	}, base)

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+base+`.csv"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := w.Write(csvData); err != nil {
		_ = err
	}
}

// tabulatorMaxUploadBytes é o limite de um arquivo enviado para uma célula da
// coluna "arquivo" (64 MiB).
const tabulatorMaxUploadBytes = 64 << 20

// HandleTabulatorUpload recebe um arquivo (multipart, campo "file") enviado ao
// clicar numa célula de coluna "arquivo" e o grava em docs/datasets/.
//
// Responde JSON: {"path":"datasets/<nome>","name":"<nome>","size":<bytes>} — o
// `path` é o valor persistido na célula e serve para montar o link de download.
//
// Os arquivos ficam em datasets/ (InternalFilePrefixes): são servidos por
// /file/download, mas NÃO são varridos/indexados, então não aparecem na lista
// lateral nem na busca como anexos.
func (ctx *HandlerContext) HandleTabulatorUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, tabulatorMaxUploadBytes)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		http.Error(w, "arquivo grande demais ou formulário inválido", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "campo \"file\" ausente", http.StatusBadRequest)
		return
	}
	defer file.Close()

	dir := filepath.Join(ctx.Cfg.DocsDir, domain.DatasetDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, "não foi possível criar a pasta de datasets", http.StatusInternalServerError)
		return
	}

	// Nome livre no diretório: relatorio.pdf, relatorio-2.pdf, ...
	base := processor.TabulatorUploadFilename(header.Filename)
	name := base
	for i := 2; ; i++ {
		_, statErr := os.Stat(filepath.Join(dir, name))
		if os.IsNotExist(statErr) {
			break
		}
		if statErr != nil {
			http.Error(w, "não foi possível verificar a pasta de datasets", http.StatusInternalServerError)
			return
		}
		name = tabulatorNumberedName(base, i)
	}

	abs := filepath.Join(dir, name)
	dst, err := os.Create(abs)
	if err != nil {
		http.Error(w, "não foi possível gravar o arquivo", http.StatusInternalServerError)
		return
	}

	written, copyErr := io.Copy(dst, file)
	closeErr := dst.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(abs)
		http.Error(w, "falha ao gravar o arquivo", http.StatusInternalServerError)
		return
	}

	relPath := domain.DatasetPrefix + name
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"path": relPath,
		"name": name,
		"size": written,
	})
	slog.Info("Arquivo de célula Tabulator gravado", "path", relPath, "bytes", written)
}

// tabulatorNumberedName acrescenta "-<n>" antes da extensão (relatorio-2.pdf).
func tabulatorNumberedName(name string, n int) string {
	ext := filepath.Ext(name)
	return fmt.Sprintf("%s-%d%s", strings.TrimSuffix(name, ext), n, ext)
}
