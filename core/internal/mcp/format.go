package mcp

import (
	"strings"

	"ton618/core/internal/core/domain"
	"ton618/core/internal/features/notes"
)

// normalizeNoteName aplica as mesmas regras dos handlers (extensão .md e
// prefixo notes/) e rejeita traversal. Devolve ("", false) para nomes
// inválidos — as tools transformam isso em mensagem de erro.
func normalizeNoteName(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.Contains(raw, "..") || strings.Contains(raw, "\\") {
		return "", false
	}
	name := notes.NoteFilename(raw)
	if !strings.HasPrefix(name, domain.NotePrefix) {
		return "", false
	}
	return name, true
}

// noteType devolve o tipo no vocabulário do frontmatter (mapa_mental, nota, …).
func noteType(tags []string, content, file string) string {
	return domain.FrontmatterTipo(domain.DetectNoteTypeFromContent(tags, content, file))
}

// displayTags remove as tags internas de editor (markmap, tabulator, ...).
func displayTags(tags []string) []string {
	return domain.FilterUserTags(tags)
}

// excerpt achata espaços/quebras e corta em max runas, para um trecho de busca
// legível no contexto do modelo.
func excerpt(texto string, max int) string {
	texto = strings.Join(strings.Fields(texto), " ")
	if max <= 0 {
		return texto
	}
	r := []rune(texto)
	if len(r) <= max {
		return texto
	}
	return strings.TrimSpace(string(r[:max])) + "…"
}

// dedupStrings preserva a ordem e remove vazios/repetidos.
func dedupStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
