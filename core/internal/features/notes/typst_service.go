package notes

import (
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
)

// TypstService compila código Typst para SVG e PDF.
type TypstService struct{}

func NewTypstService() *TypstService {
	return &TypstService{}
}

// resolvePath retorna o caminho absoluto para o executável 'typst' se ele existir em locais comuns ou no PATH.
func (s *TypstService) resolvePath() string {
	path, err := exec.LookPath("typst")
	if err == nil {
		return path
	}

	// Fallback para $HOME/.local/bin/typst
	if home, err := os.UserHomeDir(); err == nil {
		localPath := filepath.Join(home, ".local", "bin", "typst")
		if _, err := os.Stat(localPath); err == nil {
			return localPath
		}
	}

	// Fallback para caminhos comuns adicionais
	for _, p := range []string{"/usr/local/bin/typst", "/usr/bin/typst", "/bin/typst"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return "typst"
}

// CheckAvailability verifica se o binário 'typst' está instalado.
func (s *TypstService) CheckAvailability() error {
	resolved := s.resolvePath()
	if filepath.IsAbs(resolved) {
		return nil
	}
	_, err := exec.LookPath(resolved)
	return err
}

// RenderResult contém as páginas SVG compiladas.
type RenderResult struct {
	Pages []string
	Error string
}

// RenderToSVG compila conteúdo Typst para páginas SVG.
func (s *TypstService) RenderToSVG(content string) *RenderResult {
	if err := s.CheckAvailability(); err != nil {
		return &RenderResult{
			Error: "O compilador 'typst' não está instalado no servidor.",
		}
	}

	tmpDir, err := os.MkdirTemp("", "ton618-typst-*")
	if err != nil {
		slog.Error("erro ao criar diretório temporário para o typst", "error", err)
		return &RenderResult{Error: "Erro interno ao criar espaço de compilação: " + err.Error()}
	}
	defer os.RemoveAll(tmpDir)

	compileBody := s.prepareContent(content)
	compileBody = preprocessTypstImages(compileBody, tmpDir)

	tmpFile := filepath.Join(tmpDir, "main.typ")
	if err := os.WriteFile(tmpFile, []byte(compileBody), 0644); err != nil {
		slog.Error("erro ao escrever arquivo main.typ", "error", err)
		return &RenderResult{Error: "Erro interno ao salvar fonte do Typst: " + err.Error()}
	}

	args := []string{"compile", "main.typ", "page-{p}.svg"}
	if os.Getenv("TYPST_IGNORE_SYSTEM_FONTS") != "false" {
		args = append(args, "--ignore-system-fonts")
	}
	cmd := exec.Command(s.resolvePath(), args...)
	cmd.Dir = tmpDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return &RenderResult{Error: stderr.String()}
	}

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
		return &RenderResult{Error: "Nenhuma página SVG foi gerada pelo compilador."}
	}

	return &RenderResult{Pages: pages}
}

// RenderToPDF compila conteúdo Typst para PDF e retorna os bytes.
func (s *TypstService) RenderToPDF(content string) ([]byte, error) {
	if err := s.CheckAvailability(); err != nil {
		return nil, fmt.Errorf("typst não instalado: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "ton618-typst-pdf-*")
	if err != nil {
		return nil, fmt.Errorf("erro ao criar diretório temporário: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	compileBody := s.prepareContent(content)

	tmpFile := filepath.Join(tmpDir, "main.typ")
	if err := os.WriteFile(tmpFile, []byte(compileBody), 0644); err != nil {
		return nil, fmt.Errorf("erro ao escrever fonte: %w", err)
	}

	args := []string{"compile", "main.typ", "output.pdf"}
	if os.Getenv("TYPST_IGNORE_SYSTEM_FONTS") != "false" {
		args = append(args, "--ignore-system-fonts")
	}
	cmd := exec.Command(s.resolvePath(), args...)
	cmd.Dir = tmpDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("erro de compilação: %s", stderr.String())
	}

	pdfPath := filepath.Join(tmpDir, "output.pdf")
	pdfData, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler PDF: %w", err)
	}

	return pdfData, nil
}

// prepareContent limpa frontmatter e garante configuração A4.
func (s *TypstService) prepareContent(content string) string {
	body := content
	if _, parsedBody, err := ParseFrontmatter(content); err == nil {
		body = parsedBody
	}
	return "#set page(paper: \"a4\")\n" + body
}

func preprocessTypstImages(content string, tmpDir string) string {
	re := regexp.MustCompile(`(?:#)?image\((?:"(https?://[^\s"]+)"|'(https?://[^\s']+)')(?:,\s*[^)]*)?\)`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		imageURL := parts[1]
		if imageURL == "" {
			imageURL = parts[2]
		}

		if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
			return match
		}

		parsedURL, err := url.Parse(imageURL)
		if err != nil {
			return match
		}
		hash := sha256.Sum256([]byte(imageURL))
		ext := filepath.Ext(parsedURL.Path)
		if ext == "" {
			ext = ".png"
		}
		fileName := fmt.Sprintf("%x%s", hash, ext)
		localPath := filepath.Join(tmpDir, fileName)

		userCacheDir, err := os.UserCacheDir()
		var cachePath string
		var useCached bool
		if err == nil {
			cacheDir := filepath.Join(userCacheDir, "ton618-typst-cache")
			_ = os.MkdirAll(cacheDir, 0755)
			cachePath = filepath.Join(cacheDir, fileName)
			if info, err := os.Stat(cachePath); err == nil {
				// Re-download if cached file is older than 24 hours
				if time.Since(info.ModTime()) < 24*time.Hour {
					useCached = true
				}
			}
		}

		var data []byte
		if useCached {
			data, err = os.ReadFile(cachePath)
			if err != nil {
				useCached = false
			}
		}

		if !useCached {
			resp, err := http.Get(imageURL)
			if err != nil {
				return match
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return match
			}

			data, err = io.ReadAll(resp.Body)
			if err != nil {
				return match
			}

			if cachePath != "" {
				_ = os.WriteFile(cachePath, data, 0644)
			}
		}

		if err := os.WriteFile(localPath, data, 0644); err != nil {
			return match
		}

		slog.Debug("imagem typst baixada", "url", imageURL, "local", localPath)
		return strings.Replace(match, imageURL, fileName, 1)
	})
}
