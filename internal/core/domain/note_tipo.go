package domain

import "strings"

// ── Campo `tipo:` do frontmatter ────────────────────────────────────────
//
// O campo `tipo:` persiste o tipo da nota em palavra única snake_case PT
// (ex: anexo, nota). É um espelho determinístico de
// DetectNoteType (tags + caminho + nome): a fonte da verdade do tipo continua
// sendo a detecção — o frontmatter existe para exibir o campo ao usuário e
// viabilizar o filtro `tipo:` na busca global e no Acervo.
//
// Regras:
//   - processAndSave grava/atualiza o campo a cada save (note_service.go);
//   - EnsureNoteTypeField faz o backfill das notas antigas (note_service.go);
//   - uma edição manual do valor é normalizada de volta na próxima detecção;
//   - ARQUIVOS binários (pdf/anexo/arquivo/epub/imagem) não têm frontmatter
//     (DECISIONS §6.24) — o filtro deles é derivado do caminho no indexador.

// FrontmatterTipo devolve o valor canônico do campo `tipo:` para um tipo de
// nota.
func FrontmatterTipo(t NoteType) string {
	return string(t)
}

// NormalizeNoteTipo normaliza um valor de `tipo:` digitado pelo usuário
// (sintaxe `tipo:x` da busca, filtro do Acervo) para o vocabulário canônico:
// minúsculas, espaços/hífens → underscore e aliases históricos (drawing, ...).
// Valores desconhecidos são devolvidos apenas
// normalizados — o filtro simplesmente não casa nada, sem erro.
func NormalizeNoteTipo(s string) string {
	v := strings.ToLower(strings.TrimSpace(s))
	v = strings.ReplaceAll(v, " ", "_")
	v = strings.ReplaceAll(v, "-", "_")
	switch v {
	case "drawing":
		return "desenho"
	case "markdown", "md":
		return "nota"
	case "capture":
		return "captura"
	case "article":
		return "artigo"
	case "calc", "cálculo", "calculation":
		return "calculo"
	case "attachment":
		return "anexo"
	case "archive":
		return "arquivo"
	case "image", "img":
		return "imagem"
	case "pdfs":
		return "pdf"
	case "epubs":
		return "epub"
	case "anexos":
		return "anexo"
	case "arquivos":
		return "arquivo"
	case "imagens":
		return "imagem"
	case "notas":
		return "nota"
	case "artigos":
		return "artigo"
	case "capturas":
		return "captura"
	case "calculos":
		return "calculo"
	}
	return v
}

// NoteTipoLabel devolve o rótulo de exibição (pt-BR) de um valor canônico do
// campo `tipo:`. Usado em chips/dicas onde não há mapeamento do front-end
// (o Acervo usa TYPE_LABELS em core/web/src/database.js — mantenha em sincronia).
func NoteTipoLabel(v string) string {
	switch v {
	case "nota":
		return "Nota"
	case "desenho":
		return "Desenho"
	case "tabulator":
		return "Tabulator"
	case "pdf":
		return "PDF"
	case "epub":
		return "EPUB"
	case "anexo":
		return "Anexo"
	case "arquivo":
		return "Arquivo"
	case "imagem":
		return "Imagem"
	case "youtube":
		return "YouTube"
	case "artigo":
		return "Artigo"
	case "captura":
		return "Captura"
	case "calculo":
		return "Cálculo"
	}
	return v
}
