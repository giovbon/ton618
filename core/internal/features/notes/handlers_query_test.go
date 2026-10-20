package notes

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Painel de Consultas — teste com o Store REAL (SQLite + FTS5), porque o que
// está sob teste aqui é a costura: bloco no texto da nota → executor → HTML.
// ---------------------------------------------------------------------------

const notaPainel = "# Painel\n\n```consulta\ntipo: notas\ntags: [livros]\n```\n"

// TestQueryBlocos_NotaComConsulta cobre o caminho feliz: uma nota com bloco,
// duas notas no acervo e só uma com a tag pedida.
func TestQueryBlocos_NotaComConsulta(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/alfa.md", "# Alfa\n", "projeto")
	saveTestNote(t, ctx, "notes/beta.md", "# Beta\n", "livros")

	blocos, excedeu := ctx.queryBlocos("notes/painel.md", notaPainel)
	if excedeu {
		t.Errorf("excedeu deveria ser false com 1 bloco")
	}
	if len(blocos) != 1 {
		t.Fatalf("blocos = %d; quero 1", len(blocos))
	}
	b := blocos[0]
	if b.Erro != "" {
		t.Fatalf("erro no cartão: %s", b.Erro)
	}
	if b.Indice != 1 {
		t.Errorf("Indice = %d; quero 1", b.Indice)
	}
	if len(b.Result.Rows) != 1 || b.Result.Rows[0].Caminho != "notes/beta.md" {
		t.Fatalf("linhas = %+v", b.Result.Rows)
	}
	if b.Result.Rows[0].Titulo != "beta" {
		t.Errorf("Titulo = %q; quero beta", b.Result.Rows[0].Titulo)
	}
	if b.Rotulo == "" || !strings.Contains(b.Rotulo, "notas") {
		t.Errorf("Rotulo = %q", b.Rotulo)
	}
}

// TestQueryBlocos_ErroViraCartao garante o contrato de UX: sintaxe/chave errada
// NÃO viram erro HTTP — viram mensagem em português dentro do bloco.
func TestQueryBlocos_ErroViraCartao(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/alfa.md", "# Alfa\n", "")

	conteudo := "# Quebrado\n\n```consulta\ntipo: notas\ntagz: [x]\n```\n"
	blocos, _ := ctx.queryBlocos("notes/quebrado.md", conteudo)
	if len(blocos) != 1 {
		t.Fatalf("blocos = %d; quero 1", len(blocos))
	}
	if !strings.Contains(blocos[0].Erro, "campo desconhecido") {
		t.Fatalf("Erro = %q", blocos[0].Erro)
	}
}

// TestQueryBlocos_LimiteDeBlocos trava o teto de 10 blocos por nota.
func TestQueryBlocos_LimiteDeBlocos(t *testing.T) {
	ctx := newTestContext(t)
	conteudo := "# Muitos\n\n"
	for i := 0; i < 12; i++ {
		conteudo += "```consulta\ntipo: notas\nmostrar: contagem\n```\n\n"
	}
	blocos, excedeu := ctx.queryBlocos("notes/muitos.md", conteudo)
	if len(blocos) != 10 {
		t.Errorf("blocos = %d; quero 10", len(blocos))
	}
	if !excedeu {
		t.Errorf("excedeu deveria ser true com 12 blocos")
	}
}

// TestQueryBlocos_NotaSemBloco devolve nada (o painel fica invisível).
func TestQueryBlocos_NotaSemBloco(t *testing.T) {
	ctx := newTestContext(t)
	blocos, excedeu := ctx.queryBlocos("notes/simples.md", "# Só texto\n")
	if len(blocos) != 0 || excedeu {
		t.Fatalf("blocos = %d, excedeu = %v", len(blocos), excedeu)
	}
}

// TestHandleQueryPanel_RenderizaCartao exercita o handler HTTP (fragmento HTMX).
func TestHandleQueryPanel_RenderizaCartao(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/alfa.md", "# Alfa\n", "projeto")
	saveTestNote(t, ctx, "notes/beta.md", "# Beta\n", "livros")
	saveTestNote(t, ctx, "notes/painel.md", notaPainel, "")

	req := httptest.NewRequest("GET", "/api/query?file=notes/painel.md", nil)
	rec := httptest.NewRecorder()
	ctx.HandleQueryPanel(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	corpo := rec.Body.String()
	for _, esperado := range []string{"Consultas", "notes/beta.md", "beta", "atualizar"} {
		if !strings.Contains(corpo, esperado) {
			t.Errorf("resposta não contém %q", esperado)
		}
	}
	if strings.Contains(corpo, "notes/alfa.md") {
		t.Errorf("a nota sem a tag não deveria aparecer na resposta")
	}
}

// TestHandleQueryPanel_NaoEscreve é a guarda do §6.29 no nível do comportamento:
// consultar não pode alterar o conteúdo nem o mtime da nota (o motor é somente
// leitura por tipo, mas o teste protege contra um caminho de escrita futuro).
func TestHandleQueryPanel_NaoEscreve(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/painel.md", notaPainel, "livros")

	antes, err := ctx.Store.GetNote("notes/painel.md")
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/query?file=notes/painel.md", nil)
	ctx.HandleQueryPanel(httptest.NewRecorder(), req)

	depois, _ := ctx.Store.GetNote("notes/painel.md")
	if antes != depois {
		t.Fatalf("a consulta alterou o conteúdo da nota")
	}
	if !strings.Contains(depois, "```consulta") {
		t.Errorf("o texto do bloco deveria permanecer intacto na nota")
	}
}
