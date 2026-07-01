package notes

import (
	"ton618/internal/core/domain"

	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"ton618/internal/processor"
		)

// HandleTypst renderiza a página do editor split-pane do Typst.
func (ctx *HandlerContext) HandleTypst(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		filename = "notes/" + processor.GenerateCUID2() + ".md"
	}

	sanitized := NoteFilename(filename)
	if sanitized != filename {
		canonical := "/typst?file=" + url.QueryEscape(sanitized)
		http.Redirect(w, r, canonical, http.StatusFound)
		return
	}

	var content string
	var tags []string

	if data, err := ctx.Store.GetNote(filename); err == nil && data != "" {
		content = data
		ctx.Store.IncrementPopularity(filename)
	} else {
		// Conteúdo default para uma nova nota Typst com frontmatter
		content = `---
type: typst
---
#let titulo = "Manual de Direito Defensivo"
#set page(
  paper: "a4",
  fill: rgb("#1e1e2e"),
  margin: (
    x: 2cm,
    y: 2.5cm,
  ),
  header: titulo,
  footer: context {
    let atual = counter(page).get().first()
    let total = counter(page).final().first()
    align(center)[#atual / #total]
  }
)

#set text(
  size: 11pt,                 // Tamanho da fonte
  fill: rgb("#cdd6f4"),       // Cor do texto (Ex: claro #cdd6f4 ou black)
)

#outline()

= Minha Nota Typst

Comece a digitar aqui...`
	}

	fileTags, err := ctx.Store.GetFileTags(filename)
	if err == nil {
		tags = fileTags
	}

	// Redireciona planilhas, desenhos, mermaid, markmap ou editor para seus respectivos editores se esta nota mudou de tipo
	if content != "" {
		isTypst := false
		isDrawing := false
		isSpreadsheet := false
		isMermaid := false
		isMarkmap := false
		for _, t := range fileTags {
			lowerT := strings.ToLower(t)
			if lowerT == "typst" {
				isTypst = true
			} else if lowerT == "drawing" {
				isDrawing = true
			} else if lowerT == "spreadsheet" {
				isSpreadsheet = true
			} else if lowerT == "mermaid" {
				isMermaid = true
			} else if lowerT == "mindmap" || lowerT == "markmap" {
				isMarkmap = true
			}
		}
		if isSpreadsheet || strings.Contains(content, "type: spreadsheet") {
			http.Redirect(w, r, "/spreadsheet?file="+url.QueryEscape(filename), http.StatusFound)
			return
		}
		if isDrawing || strings.Contains(content, "type: drawing") {
			http.Redirect(w, r, "/drawing?file="+url.QueryEscape(filename), http.StatusFound)
			return
		}
		if isMermaid || strings.Contains(content, "type: mermaid") {
			http.Redirect(w, r, "/mermaid?file="+url.QueryEscape(filename), http.StatusFound)
			return
		}
		if isMarkmap || strings.Contains(content, "type: mindmap") || strings.Contains(content, "type: markmap") {
			http.Redirect(w, r, "/mindmap?file="+url.QueryEscape(filename), http.StatusFound)
			return
		}
		if !isTypst && !strings.Contains(content, "type: typst") {
			http.Redirect(w, r, "/editor?file="+url.QueryEscape(filename), http.StatusFound)
			return
		}
	}

	// Filtra as tags internas para não mostrar na UI do editor
	var userTags []string
	for _, t := range fileTags {
		lt := strings.ToLower(t)
		if lt != "spreadsheet" && lt != "drawing" && lt != "typst" && lt != "mermaid" && lt != "mindmap" && lt != "markmap" {
			userTags = append(userTags, t)
		}
	}
	tags = userTags



	allTags, err := ctx.Store.GetAllTags()
	if err != nil {
		allTags = nil
	}

	backlinks, err := ctx.Notes.GetBacklinks(filename)
	if err != nil {
		backlinks = &domain.BacklinksResult{}
	}

	data := domain.EditorData{
		Title:       "Typst - " + filename,
		Filename:    filename,
		DisplayName: domain.DisplayName(filename),
		Content:     content,
		Tags:        tags,
		AllTags:     allTags,
		Backlinks:   backlinks,
	}

	Typst(data).Render(r.Context(), w)
}

// HandleTypstRender compila o conteúdo Typst para SVG usando o CLI local do sistema.
func (ctx *HandlerContext) HandleTypstRender(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	content := r.FormValue("content")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// 1. Verifica se o executável 'typst' existe no PATH
	_, err := exec.LookPath("typst")
	if err != nil {
		w.Write([]byte(`<div class="w-full bg-red-950/85 border border-red-800/60 text-red-300 px-4 py-3 rounded-lg font-mono text-xs whitespace-pre-wrap shrink-0 overflow-x-auto">O compilador 'typst' não está instalado no servidor. Por favor, execute 'winget install Typst.Typst' (Windows) ou 'brew install typst' (Mac/Linux), ou verifique se ele está no PATH do sistema.</div>`))
		return
	}

	// 2. Cria diretório temporário para a compilação
	tmpDir, err := os.MkdirTemp("", "ton618-typst-*")
	if err != nil {
		slog.Error("erro ao criar diretório temporário para o typst", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`<div class="w-full bg-red-950/85 border border-red-800/60 text-red-300 px-4 py-3 rounded-lg font-mono text-xs whitespace-pre-wrap shrink-0 overflow-x-auto">Erro interno ao criar espaço de compilação: ` + err.Error() + `</div>`))
		return
	}
	defer os.RemoveAll(tmpDir)

	// Limpa o frontmatter para compilar apenas o código Typst puro
	compileBody := content
	if _, body, err := ParseFrontmatter(content); err == nil {
		compileBody = body
	}
	// Garante A4 como padrão no compilador sem sujar o código da nota com regras #set
	compileBody = "#set page(paper: \"a4\")\n" + compileBody

	// Pre-processa as imagens do documento que sao URLs remotas
	compileBody = preprocessTypstImages(compileBody, tmpDir)

	// 3. Grava o código Typst em um arquivo temporário
	tmpFile := filepath.Join(tmpDir, "main.typ")
	if err := os.WriteFile(tmpFile, []byte(compileBody), 0644); err != nil {
		slog.Error("erro ao escrever arquivo main.typ", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`<div class="w-full bg-red-950/85 border border-red-800/60 text-red-300 px-4 py-3 rounded-lg font-mono text-xs whitespace-pre-wrap shrink-0 overflow-x-auto">Erro interno ao salvar fonte do Typst: ` + err.Error() + `</div>`))
		return
	}

	// 4. Executa a compilação do Typst para SVG
	cmd := exec.Command("typst", "compile", "main.typ", "page-{p}.svg")
	cmd.Dir = tmpDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Retorna erro amigável de compilação
		w.Write([]byte(`<div class="w-full bg-red-950/85 border border-red-800/60 text-red-300 px-4 py-3 rounded-lg font-mono text-xs whitespace-pre-wrap shrink-0 overflow-x-auto">` + stderr.String() + `</div>`))
		return
	}

	// 5. Coleta todas as páginas geradas
	var pages []string
	for i := 1; ; i++ {
		pagePath := filepath.Join(tmpDir, fmt.Sprintf("page-%d.svg", i))
		data, err := os.ReadFile(pagePath)
		if err != nil {
			break
		}
		pages = append(pages, string(data))
	}

	if len(pages) == 0 {
		w.Write([]byte(`<div class="text-zinc-600 text-xs py-20 font-mono">Nenhuma página SVG foi gerada pelo compilador.</div>`))
		return
	}

	// 6. Sucesso! Renderiza o HTML final
	var finalHTML strings.Builder
	for _, page := range pages {
		finalHTML.WriteString(`<div class="typst-page">`)
		finalHTML.WriteString(page)
		finalHTML.WriteString(`</div>`)
	}
	w.Write([]byte(finalHTML.String()))
}

// HandleTypstPDF compila o conteúdo Typst para um arquivo PDF e o retorna como download.
func (ctx *HandlerContext) HandleTypstPDF(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		http.Error(w, "parâmetro 'file' é obrigatório", http.StatusBadRequest)
		return
	}

	sanitized := NoteFilename(filename)
	content, err := ctx.Store.GetNote(sanitized)
	if err != nil || content == "" {
		http.Error(w, "nota não encontrada", http.StatusNotFound)
		return
	}

	// 1. Verifica se o executável 'typst' existe no PATH
	_, err = exec.LookPath("typst")
	if err != nil {
		http.Error(w, "O compilador 'typst' não está instalado no servidor.", http.StatusInternalServerError)
		return
	}

	// 2. Limpa o frontmatter para compilar apenas o código Typst puro
	compileBody := content
	if _, body, err := ParseFrontmatter(content); err == nil {
		compileBody = body
	}
	// Garante A4 como padrão no compilador sem sujar o código da nota com regras #set
	compileBody = "#set page(paper: \"a4\")\n" + compileBody

	// 3. Cria diretório temporário para a compilação
	tmpDir, err := os.MkdirTemp("", "ton618-typst-pdf-*")
	if err != nil {
		slog.Error("erro ao criar diretório temporário para o typst pdf", "error", err)
		http.Error(w, fmt.Sprintf("Erro interno ao criar espaço de compilação: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)

	// Pre-processa as imagens do documento que sao URLs remotas
	compileBody = preprocessTypstImages(compileBody, tmpDir)

	// 4. Grava o código Typst em um arquivo temporário
	tmpFile := filepath.Join(tmpDir, "main.typ")
	if err := os.WriteFile(tmpFile, []byte(compileBody), 0644); err != nil {
		slog.Error("erro ao escrever arquivo main.typ", "error", err)
		http.Error(w, fmt.Sprintf("Erro interno ao salvar fonte do Typst: %v", err), http.StatusInternalServerError)
		return
	}

	// 5. Executa a compilação do Typst para PDF
	pdfPath := filepath.Join(tmpDir, "output.pdf")
	cmd := exec.Command("typst", "compile", "main.typ", "output.pdf")
	cmd.Dir = tmpDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		slog.Error("erro ao compilar PDF", "error", err, "stderr", stderr.String())
		http.Error(w, fmt.Sprintf("Erro de compilação do Typst:\n%s", stderr.String()), http.StatusBadRequest)
		return
	}

	// 6. Lê o PDF gerado
	pdfBytes, err := os.ReadFile(pdfPath)
	if err != nil {
		http.Error(w, "Erro ao ler o PDF gerado", http.StatusInternalServerError)
		return
	}

	// 7. Define headers e retorna o arquivo
	displayName := domain.DisplayName(sanitized)
	pdfName := strings.TrimSuffix(displayName, ".md") + ".pdf"

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", strings.ReplaceAll(pdfName, "\"", "\\\"")))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
	w.Write(pdfBytes)
}

// preprocessTypstImages localiza URLs de imagens no conteúdo e baixa-as no diretório temporário,
// substituindo as URLs pelos nomes dos arquivos locais gerados.
func preprocessTypstImages(content string, tmpDir string) string {
	re := regexp.MustCompile(`image\(\s*["'](https?://[^"']+)["']`)
	matches := re.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return content
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	downloaded := make(map[string]string)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		urlStr := match[1]
		if _, exists := downloaded[urlStr]; exists {
			continue
		}

		hash := sha256.Sum256([]byte(urlStr))
		hashHex := fmt.Sprintf("%x", hash)

		resp, err := client.Get(urlStr)
		if err != nil {
			slog.Warn("falha ao baixar imagem remota para o typst", "url", urlStr, "error", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			slog.Warn("imagem remota retornou status nao-OK para o typst", "url", urlStr, "status", resp.Status)
			continue
		}

		ext := ".png"
		contentType := resp.Header.Get("Content-Type")
		switch {
		case strings.Contains(contentType, "image/jpeg") || strings.Contains(contentType, "image/jpg"):
			ext = ".jpg"
		case strings.Contains(contentType, "image/png"):
			ext = ".png"
		case strings.Contains(contentType, "image/gif"):
			ext = ".gif"
		case strings.Contains(contentType, "image/svg+xml"):
			ext = ".svg"
		default:
			if urlExt := filepath.Ext(urlStr); urlExt != "" {
				if idx := strings.Index(urlExt, "?"); idx != -1 {
					urlExt = urlExt[:idx]
				}
				if idx := strings.Index(urlExt, "#"); idx != -1 {
					urlExt = urlExt[:idx]
				}
				if len(urlExt) <= 5 {
					ext = urlExt
				}
			}
		}

		localName := hashHex + ext
		localPath := filepath.Join(tmpDir, localName)

		out, err := os.Create(localPath)
		if err != nil {
			slog.Error("falha ao criar arquivo local para imagem do typst", "path", localPath, "error", err)
			continue
		}

		_, err = io.Copy(out, resp.Body)
		out.Close()
		if err != nil {
			slog.Error("falha ao gravar imagem baixada para o typst", "url", urlStr, "error", err)
			os.Remove(localPath)
			continue
		}

		downloaded[urlStr] = localName
	}

	newContent := content
	for urlStr, localName := range downloaded {
		newContent = strings.ReplaceAll(newContent, urlStr, localName)
	}

	return newContent
}
