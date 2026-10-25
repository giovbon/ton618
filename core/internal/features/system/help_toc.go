package system

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
)

// ── Índice do /help ──────────────────────────────────────────────────────
//
// O índice NÃO é escrito à mão no HELP.md: ele é derivado dos títulos do
// documento a cada requisição. Assim, adicionar uma seção nova já a faz
// aparecer no índice, sem ninguém lembrar de atualizar uma lista paralela.
//
// Cada título ganha um `id` determinístico (slug ASCII) e o índice aponta
// para `#id`. Os ids são estáveis entre requisições — o mesmo título gera
// sempre o mesmo link — e recebem sufixo numérico quando repetidos.

// HelpTOCEntry é uma entrada do índice gerado em /help.
// `Children` guarda as subseções (h3) da seção (h2).
type HelpTOCEntry struct {
	ID       string
	Text     string
	Children []HelpTOCEntry
}

// helpScrollMargin mantém o título visível abaixo da navbar fixa ao pular
// para uma âncora. Fica em style inline (e não numa classe) para não depender
// do CSS gerado pelo Tailwind.
const helpScrollMargin = "scroll-margin-top:5.5rem"

// helpHeadingIDs percorre o documento, dá um `id` a cada título e devolve as
// entradas do índice (h2 como seção, h3 como subseção).
func helpHeadingIDs(doc ast.Node, src []byte) []HelpTOCEntry {
	var entries []HelpTOCEntry
	usados := make(map[string]int)

	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}

		texto := helpHeadingText(h, src)
		id := helpUniqueID(helpSlug(texto), usados)
		h.SetAttributeString("id", []byte(id))
		h.SetAttributeString("style", []byte(helpScrollMargin))

		switch h.Level {
		case 2:
			entries = append(entries, HelpTOCEntry{ID: id, Text: texto})
		case 3:
			if len(entries) == 0 {
				// Subseção antes de qualquer seção: entra como raiz para não
				// sumir do índice.
				entries = append(entries, HelpTOCEntry{ID: id, Text: texto})
				break
			}
			atual := &entries[len(entries)-1]
			atual.Children = append(atual.Children, HelpTOCEntry{ID: id, Text: texto})
		}
		return ast.WalkContinue, nil
	})
	return entries
}

// helpHeadingText devolve o texto puro do título (sem a marcação markdown).
// Um `### 5.3 O Acervo (`/database`)` vira `5.3 O Acervo (/database)`.
func helpHeadingText(h *ast.Heading, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(h, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Text:
			b.Write(n.Segment.Value(src))
		case *ast.String:
			b.Write(n.Value)
		}
		return ast.WalkContinue, nil
	})
	return strings.Join(strings.Fields(b.String()), " ")
}

// helpSlug transforma o título num id ASCII: minúsculas, sem acentos, com
// hífens no lugar da pontuação. `4.5 Nota semanal` vira `4-5-nota-semanal`.
func helpSlug(texto string) string {
	var b strings.Builder
	ultimoHifen := false
	for _, r := range strings.ToLower(texto) {
		if base, ok := helpAccents[r]; ok {
			r = base
		}
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			ultimoHifen = false
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			// Letras não latinas não entram no id: o href ficaria com
			// percent-encoding e o link deixaria de ser previsível.
			continue
		default:
			if !ultimoHifen && b.Len() > 0 {
				b.WriteByte('-')
				ultimoHifen = true
			}
		}
	}
	id := strings.Trim(b.String(), "-")
	if id == "" {
		return "secao"
	}
	return id
}

// helpUniqueID garante ids distintos quando dois títulos coincidem
// (`2-notas`, `2-notas-2`, …). O contador fica no mapa compartilhado pela
// varredura, então a numeração é determinística.
func helpUniqueID(base string, usados map[string]int) string {
	n := usados[base] + 1
	usados[base] = n
	if n == 1 {
		return base
	}
	return base + "-" + strconv.Itoa(n)
}

// helpAccents mapeia as letras acentuadas do português para a base ASCII.
var helpAccents = map[rune]rune{
	'á': 'a', 'à': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a',
	'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e',
	'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i',
	'ó': 'o', 'ò': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o',
	'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u',
	'ç': 'c', 'ñ': 'n',
}

// helpTOCLabel resume o tamanho do índice ("17 seções · 70 subseções").
func helpTOCLabel(entries []HelpTOCEntry) string {
	secoes, subsecoes := helpTOCSize(entries)
	label := strconv.Itoa(secoes) + " seções"
	if secoes == 1 {
		label = "1 seção"
	}
	if subsecoes > 0 {
		if subsecoes == 1 {
			label += " · 1 subseção"
		} else {
			label += " · " + strconv.Itoa(subsecoes) + " subseções"
		}
	}
	return label
}

// helpTOCSize conta seções e subseções para o cabeçalho do índice.
func helpTOCSize(entries []HelpTOCEntry) (secoes, subsecoes int) {
	for _, e := range entries {
		secoes++
		subsecoes += len(e.Children)
	}
	return secoes, subsecoes
}
