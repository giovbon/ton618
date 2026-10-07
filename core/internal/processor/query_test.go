package processor

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Parser do bloco ```consulta (DECISIONS §6.29).
//
// Estes testes são "golden": o texto do bloco entra, o Spec (ou a mensagem em
// português) sai. O último teste garante a PARIDADE entre as listas do parser e
// docs/linguagem-de-consulta.md — se alguém renomear uma chave só num dos dois
// lugares, ele quebra.
// ---------------------------------------------------------------------------

func TestParseQuery_BlocoCompleto(t *testing.T) {
	src := strings.Join([]string{
		"tipo: notas",
		"tags: [projeto, ativo]",
		"sem-tags: [rascunho]",
		"texto: \"foco profundo\"",
		"pasta: notes",
		"onde: n.citada_por >= 3",
		"ordenar: mtime desc",
		"limite: 15",
		"mostrar: tabela",
		"colunas: [titulo, tags, citada_por]",
	}, "\n")

	spec, err := ParseQuery(src)
	if err != nil {
		t.Fatalf("ParseQuery: %v", err)
	}

	if spec.Tipo != QueryTipoNotas {
		t.Errorf("Tipo = %q; quero %q", spec.Tipo, QueryTipoNotas)
	}
	if spec.Onde != "n.citada_por >= 3" {
		t.Errorf("Onde = %q", spec.Onde)
	}
	if spec.Ordenar != "mtime" || spec.Ordem != "desc" {
		t.Errorf("Ordenar = %q/%q; quero mtime/desc", spec.Ordenar, spec.Ordem)
	}
	if spec.Limite != 15 {
		t.Errorf("Limite = %d; quero 15", spec.Limite)
	}
	if spec.Mostrar != QueryMostrarTabela {
		t.Errorf("Mostrar = %q", spec.Mostrar)
	}
	if got := strings.Join(spec.Colunas, ","); got != "titulo,tags,citada_por" {
		t.Errorf("Colunas = %q", got)
	}
	if got := strings.Join(spec.Tags, ","); got != "projeto,ativo" {
		t.Errorf("Tags = %q", got)
	}
	if got := strings.Join(spec.SemTags, ","); got != "rascunho" {
		t.Errorf("SemTags = %q", got)
	}
	if spec.Texto != "foco profundo" {
		t.Errorf("Texto = %q", spec.Texto)
	}
	if spec.Pasta != "notes" {
		t.Errorf("Pasta = %q", spec.Pasta)
	}
}

func TestParseQuery_Defaults(t *testing.T) {
	spec, err := ParseQuery("tipo: notas")
	if err != nil {
		t.Fatalf("ParseQuery: %v", err)
	}
	if spec.Limite != DefaultQueryLimit {
		t.Errorf("Limite = %d; quero o padrão %d", spec.Limite, DefaultQueryLimit)
	}
	if spec.Mostrar != QueryMostrarLista {
		t.Errorf("Mostrar = %q; quero %q", spec.Mostrar, QueryMostrarLista)
	}
	if spec.Ordenar != "" || spec.Ordem != "" {
		t.Errorf("Ordenar = %q/%q; quero vazio (direção natural)", spec.Ordenar, spec.Ordem)
	}
}

func TestParseQuery_Erros(t *testing.T) {
	casos := []struct {
		nome   string
		src    string
		trecho string // trecho que a mensagem precisa conter
	}{
		{"vazio", "", "consulta vazia"},
		{"sem tipo", "onde: n.citada_por > 0", "campo obrigatório ausente: tipo"},
		{"tipo inválido", "tipo: nota", "valor inválido"},
		{"tipo agenda", "tipo: agenda", "v2"},
		{"chave desconhecida", "tipo: notas\ntagz: [x]", "campo desconhecido \"tagz\""},
		{"chave desconhecida lista chaves", "tipo: notas\nfoo: 1", "chaves válidas"},
		{"yaml inválido", "tipo: notas\n\tonde: x", "YAML inválido"},
		{"limite zero", "tipo: notas\nlimite: 0", "entre 1 e 200"},
		{"limite grande", "tipo: notas\nlimite: 500", "entre 1 e 200"},
		{"mostrar inválido", "tipo: notas\nmostrar: quadro", "valor inválido"},
		{"arvore fora de hierarquia", "tipo: notas\nmostrar: arvore", "só existe para \"tipo: hierarquia\""},
		{"colunas sem tabela", "tipo: notas\ncolunas: [titulo]", "só faz sentido com \"mostrar: tabela\""},
		{"coluna inválida", "tipo: notas\nmostrar: tabela\ncolunas: [Titulo!]", "nome de coluna inválido"},
		{"coluna desconhecida", "tipo: notas\nmostrar: tabela\ncolunas: [tamanho]", "coluna desconhecida \"tamanho\""},
		{"ordenar formato", "tipo: notas\nordenar: mtime desc asc", "formato inválido"},
		{"ordenar direção", "tipo: notas\nordenar: mtime up", "direção inválida"},
		{"ordenar campo", "tipo: notas\nordenar: velocidade", "campo desconhecido \"velocidade\""},
		{"pasta inválida", "tipo: notas\npasta: docs", "valor inválido"},
		{"estado inválido", "tipo: tarefas\nestado: aberta", "valor inválido"},
		{"tags em tarefas", "tipo: tarefas\ntags: [x]", "só existe para \"tipo: notas\""},
		{"marcador em notas", "tipo: notas\nmarcador: [TODO]", "só existe para \"tipo: tarefas\""},
		{"de em notas", "tipo: notas\nde: Outra", "só existe para \"tipo: hierarquia\""},
		{"janela", "tipo: notas\njanela: hoje", "v2"},
		{"onde longa", "tipo: notas\nonde: \"" + strings.Repeat("x", MaxQueryOndeBytes+1) + "\"", "longa demais"},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			_, err := ParseQuery(c.src)
			if err == nil {
				t.Fatalf("esperava erro, veio nil")
			}
			if !strings.Contains(err.Error(), c.trecho) {
				t.Fatalf("mensagem %q não contém %q", err.Error(), c.trecho)
			}
		})
	}
}

func TestParseQuery_AceitaCadaChaveDocumentada(t *testing.T) {
	// Cada chave da lista canônica precisa funcionar no tipo em que se aplica —
	// é o contrato que o doc promete.
	casos := map[string]string{
		"tipo":     "tipo: notas",
		"onde":     "tipo: notas\nonde: n.titulo != \"\"",
		"ordenar":  "tipo: notas\nordenar: titulo asc",
		"limite":   "tipo: notas\nlimite: 5",
		"mostrar":  "tipo: notas\nmostrar: contagem",
		"colunas":  "tipo: notas\nmostrar: tabela\ncolunas: [titulo]",
		"tags":     "tipo: notas\ntags: [a]",
		"sem-tags": "tipo: notas\nsem-tags: [b]",
		"texto":    "tipo: notas\ntexto: foco",
		"pasta":    "tipo: notas\npasta: notes",
		"marcador": "tipo: tarefas\nmarcador: [TODO]",
		"estado":   "tipo: tarefas\nestado: pendente",
		"de":       "tipo: hierarquia\nde: Projeto",
		"janela":   "tipo: notas\njanela: hoje", // reservado: erro dedicado de v2
	}

	for chave, src := range casos {
		t.Run(chave, func(t *testing.T) {
			_, err := ParseQuery(src)
			if chave == "janela" {
				if err == nil || !strings.Contains(err.Error(), "v2") {
					t.Fatalf("janela deveria ser recusada como reservada; err = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("chave documentada %q recusada: %v", chave, err)
			}
		})
	}
}

func TestExtractQueryBlocks(t *testing.T) {
	md := strings.Join([]string{
		"# Nota",       // 1
		"",             // 2
		"```go",        // 3  (código comum: ignora)
		"tipo: notas",  // 4
		"```",          // 5
		"",             // 6
		"```consulta",  // 7  → bloco 1 (conteúdo na linha 8)
		"tipo: notas",  // 8
		"limite: 5",    // 9
		"```",          // 10
		"",             // 11
		"````consulta", // 12 → bloco 2 (cerca de 4 crases)
		"tipo: notas",  // 13
		"```",          // 14 (fecha o de dentro do texto? não: fecha o bloco 2 pelo tamanho)
		"````",         // 15
		"",             // 16
		"```consulta",  // 17 aberto e nunca fechado → ignorado
		"tipo: notas",  // 18
	}, "\n")

	blocks := ExtractQueryBlocks(md)
	if len(blocks) != 2 {
		t.Fatalf("encontrei %d blocos; quero 2", len(blocks))
	}
	if blocks[0].Index != 1 || blocks[1].Index != 2 {
		t.Errorf("índices = %d/%d; quero 1/2", blocks[0].Index, blocks[1].Index)
	}
	if blocks[0].Source != "tipo: notas\nlimite: 5" {
		t.Errorf("bloco 1 = %q", blocks[0].Source)
	}
	if blocks[0].Line != 8 {
		t.Errorf("linha do bloco 1 = %d; quero 8", blocks[0].Line)
	}
	if blocks[0].Index == 1 {
		if _, err := ParseQuery(blocks[0].Source); err != nil {
			t.Errorf("bloco extraído não parseia: %v", err)
		}
	}
}

func TestExtractQueryBlocks_CercaTildeEInfoCaseInsensitive(t *testing.T) {
	md := "~~~Consulta\ntipo: notas\n~~~\n"
	blocks := ExtractQueryBlocks(md)
	if len(blocks) != 1 {
		t.Fatalf("encontrei %d blocos; quero 1", len(blocks))
	}
	if _, err := ParseQuery(blocks[0].Source); err != nil {
		t.Fatalf("ParseQuery: %v", err)
	}
}

func TestExtractQueryBlocks_SemBlocos(t *testing.T) {
	if got := ExtractQueryBlocks("# Só texto\n\nnada aqui\n"); len(got) != 0 {
		t.Fatalf("encontrei %d blocos; quero 0", len(got))
	}
}

// TestQueryLists_DocParity trava a paridade parser ↔ documentação: as chaves do
// bloco e os campos de `n`/`t` aceitos têm que ser EXATAMENTE os documentados em
// docs/linguagem-de-consulta.md (é o "SSOT" da linguagem — ver DECISIONS §6.29).
func TestQueryLists_DocParity(t *testing.T) {
	caminho := filepath.Join("..", "..", "..", "docs", "linguagem-de-consulta.md")
	raw, err := os.ReadFile(caminho)
	if err != nil {
		t.Skipf("documentação não encontrada (%v)", err)
	}
	doc := string(raw)

	// --- chaves: só a seção 3 (a tabela de atalhos da seção 13 citaria outras) ---
	inicio := strings.Index(doc, "## 3.")
	fim := strings.Index(doc, "## 4.")
	if inicio < 0 || fim <= inicio {
		t.Fatalf("não achei a seção 3 do doc")
	}
	sec3 := doc[inicio:fim]

	reChave := regexp.MustCompile("(?m)^\\| `([a-z][a-z-]*)` \\|")
	docKeys := map[string]bool{}
	for _, m := range reChave.FindAllStringSubmatch(sec3, -1) {
		docKeys[m[1]] = true
	}

	reCampo := regexp.MustCompile("(?m)^\\| `n\\.([a-z_]+)` \\|")
	docNote := map[string]bool{}
	for _, m := range reCampo.FindAllStringSubmatch(doc, -1) {
		docNote[m[1]] = true
	}
	reCampoT := regexp.MustCompile("(?m)^\\| `t\\.([a-z_]+)` \\|")
	docTask := map[string]bool{}
	for _, m := range reCampoT.FindAllStringSubmatch(doc, -1) {
		docTask[m[1]] = true
	}

	comparar := func(nome string, doParser []string, doDoc map[string]bool) {
		for _, k := range doParser {
			if !doDoc[k] {
				t.Errorf("%s: %q existe no parser mas não está documentado", nome, k)
			}
		}
		parser := map[string]bool{}
		for _, k := range doParser {
			parser[k] = true
		}
		var faltando []string
		for k := range doDoc {
			if !parser[k] {
				faltando = append(faltando, k)
			}
		}
		sort.Strings(faltando)
		if len(faltando) > 0 {
			t.Errorf("%s: documentado mas não existe no parser: %s", nome, strings.Join(faltando, ", "))
		}
	}

	comparar("chaves", QueryKeys(), docKeys)
	comparar("campos de n", QueryNoteFields(), docNote)
	comparar("campos de t", QueryTaskFields(), docTask)

	// Colunas de `mostrar: tabela` — a lista vive no §9 do doc.
	linhaColunas := regexp.MustCompile(`(?m)^\* Colunas válidas:.*$`).FindString(doc)
	if linhaColunas == "" {
		t.Fatalf("doc §9 não tem a linha 'Colunas válidas: …'")
	}
	docColunas := map[string]bool{}
	for _, m := range regexp.MustCompile("`([a-z_]+)`").FindAllStringSubmatch(linhaColunas, -1) {
		docColunas[m[1]] = true
	}
	comparar("colunas", QueryColunas(), docColunas)
	// Campos ordenáveis precisam ser um subconjunto dos campos de n/t.
	for _, c := range QueryOrderFields(QueryTipoNotas) {
		if !containsString(QueryNoteFields(), c) {
			t.Errorf("ordenar: campo %q não é campo de n", c)
		}
	}
	for _, c := range QueryOrderFields(QueryTipoTarefas) {
		if !containsString(QueryTaskFields(), c) {
			t.Errorf("ordenar: campo %q não é campo de t", c)
		}
	}
}
