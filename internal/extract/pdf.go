package extract

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// pdfTimeout limita uma extração de PDF. Livros grandes levam segundos; o teto
// existe para não deixar um processo travado segurando a requisição HTTP.
const pdfTimeout = 3 * time.Minute

// lookPath é variável para os testes simularem a ausência do binário.
var lookPath = exec.LookPath

// runPDFToText é variável para os testes injetarem saída sem depender do
// poppler instalado na máquina de teste.
var runPDFToText = func(ctx context.Context, path string) (stdout []byte, stderr string, truncated bool, err error) {
	buf := &capBuffer{limit: MaxTextBytes}
	var errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, "pdftotext", "-enc", "UTF-8", "-q", path, "-")
	cmd.Stdout = buf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return buf.buf, strings.TrimSpace(errBuf.String()), buf.truncated, err
}

// PDF extrai o texto de um PDF via `pdftotext` (poppler-utils).
func PDF(ctx context.Context, path string) (*Result, error) {
	if _, err := lookPath("pdftotext"); err != nil {
		return nil, ErrToolMissing
	}

	ctx, cancel := context.WithTimeout(ctx, pdfTimeout)
	defer cancel()

	out, stderr, truncated, err := runPDFToText(ctx, path)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("extração de PDF excedeu o limite de %s", pdfTimeout)
		}
		if stderr != "" {
			return nil, fmt.Errorf("pdftotext: %w: %s", err, stderr)
		}
		return nil, fmt.Errorf("pdftotext: %w", err)
	}

	text := string(out)
	if truncated {
		text = truncateUTF8(text, MaxTextBytes)
	}
	// \f separa páginas no pdftotext; vira uma quebra de linha simples.
	text = strings.ReplaceAll(text, "\f", "\n")
	text = normalizeText(text)
	if strings.TrimSpace(text) == "" {
		return nil, ErrEmpty
	}

	return &Result{
		Chapters:  []Chapter{{Text: text}},
		Truncated: truncated,
	}, nil
}
