package notes

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"ton618/core/internal/core/db"
	"ton618/core/internal/processor"
	"ton618/core/internal/query"
)

// ---------------------------------------------------------------------------
// Guardas do painel de Consultas (DECISIONS §6.29).
//
// Dois pontos que já foram problema e que devem continuar travados:
//   1. o leitor é um SNAPSHOT por requisição (sem isso, cada bloco refazia
//      todas as leituras — medido: 22 leituras para 3 blocos);
//   2. cada `mostrar:` tem que renderizar de verdade (a lista de colunas já
//      chegou a aceitar nomes que saíam em branco na tela).
// ---------------------------------------------------------------------------

// TestQueryReader_MemoizaLeituras prova o snapshot: chamadas repetidas devolvem
// a MESMA fatia/mapa (e um leitor novo = requisição nova = snapshot novo).
func TestQueryReader_MemoizaLeituras(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/a.md", "# A\n", "livros")
	saveTestNote(t, ctx, "notes/b.md", "# B\n", "livros")

	r := newQueryReader(ctx.Store)

	f1, err := r.Files()
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	f2, _ := r.Files()
	if len(f1) == 0 || &f1[0] != &f2[0] {
		t.Errorf("Files() não está memoizado (fatias diferentes)")
	}

	c1, _ := r.Contents()
	c2, _ := r.Contents()
	if len(c1) == 0 {
		t.Fatalf("Contents() vazio")
	}
	c1["__marca__"] = "x"
	if c2["__marca__"] != "x" {
		t.Errorf("Contents() não está memoizado (mapas diferentes)")
	}

	// Hierarchy usa Files()/Contents() memoizados: chamar depois não troca a fatia.
	if _, err := r.Hierarchy(); err != nil {
		t.Fatalf("Hierarchy: %v", err)
	}
	f3, _ := r.Files()
	if &f3[0] != &f1[0] {
		t.Errorf("Hierarchy() refez a leitura de Files()")
	}

	// Requisição nova → leitor novo → snapshot novo (fatias diferentes).
	r2 := newQueryReader(ctx.Store)
	f4, _ := r2.Files()
	if len(f4) > 0 && &f4[0] == &f1[0] {
		t.Errorf("leitores distintos não deveriam compartilhar o snapshot")
	}
}

// TestQueryPanel_RenderizaCadaApresentacao passa pelo caminho real (bloco no
// texto → parser → motor → template) para cada `mostrar:`.
func TestQueryPanel_RenderizaCadaApresentacao(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/alfa.md", "# Alfa\n\nconteudo alfa\n", "projeto")
	saveTestNote(t, ctx, "notes/beta.md", "---\npai: alfa\n---\n\n# Beta\n\nconteudo beta\n", "livros")

	// FTS: em produção o watcher/processor chama IndexFTS ao reindexar a nota —
	// aqui entra pela mesma porta que os testes de busca usam.
	carimbo := time.Now().Format(time.RFC3339)
	ctx.Store.InsertDocument(db.Document{
		ID: "doc-alfa", Tipo: "nota", Arquivo: "notes/alfa.md", Secao: "Geral",
		Texto: "conteudo alfa sobre foco profundo", Timestamp: carimbo,
	})
	ctx.Store.IndexFTS("doc-alfa", "nota", "notes/alfa.md", "Geral", "conteudo alfa sobre foco profundo", "")
	ctx.Store.InsertDocument(db.Document{
		ID: "doc-beta", Tipo: "nota", Arquivo: "notes/beta.md", Secao: "Geral",
		Texto: "conteudo beta sobre leitura", Timestamp: carimbo,
	})
	ctx.Store.IndexFTS("doc-beta", "nota", "notes/beta.md", "Geral", "conteudo beta sobre leitura", "")

	casos := []struct {
		nome  string
		bloco string
		quero []string
	}{
		{"contagem", "tipo: notas\nmostrar: contagem", []string{"Consultas", "notas"}},
		{"lista", "tipo: notas\nordenar: titulo asc\nmostrar: lista", []string{"divide-y", "notes/alfa.md"}},
		{"tabela", "tipo: notas\nmostrar: tabela\ncolunas: [titulo, tags, modif_ha_dias]", []string{"<table", "Dias sem editar"}},
		{"cartoes", "tipo: notas\ntexto: foco\nmostrar: cartoes", []string{"line-clamp-3", "notes/alfa.md"}},
		{"arvore", "tipo: hierarquia\nde: alfa\nmostrar: arvore", []string{"└", "beta"}},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			conteudo := "# Painel\n\n```consulta\n" + c.bloco + "\n```\n"
			blocos, _ := ctx.queryBlocos("notes/painel.md", conteudo)
			if len(blocos) != 1 {
				t.Fatalf("blocos = %d; quero 1", len(blocos))
			}
			if blocos[0].Erro != "" {
				t.Fatalf("erro no cartão: %s", blocos[0].Erro)
			}

			var buf bytes.Buffer
			if err := QueryPanel("notes/painel.md", blocos, false).Render(context.Background(), &buf); err != nil {
				t.Fatalf("Render: %v", err)
			}
			html := buf.String()
			for _, marca := range c.quero {
				if !strings.Contains(html, marca) {
					t.Errorf("HTML de %q não contém %q\n---\n%s", c.nome, marca, html)
				}
			}
		})
	}
}

// TestQueryPanel_ErroNaoVazaHTML garante que a mensagem de erro entra como
// TEXTO (escapada), nunca como markup vindo da expressão do usuário.
func TestQueryPanel_ErroNaoVazaHTML(t *testing.T) {
	ctx := newTestContext(t)

	blocos, _ := ctx.queryBlocos("notes/painel.md", "```consulta\ntipo: notas\nonde: n.<script>\n```\n")
	if len(blocos) != 1 || blocos[0].Erro == "" {
		t.Fatalf("esperava um cartão de erro; veio %+v", blocos)
	}

	var buf bytes.Buffer
	if err := QueryPanel("notes/painel.md", blocos, false).Render(context.Background(), &buf); err != nil {
		t.Fatalf("Render: %v", err)
	}
	html := buf.String()
	if strings.Contains(html, "<script>") {
		t.Errorf("a expressão do usuário vazou como markup:\n%s", html)
	}
	if !strings.Contains(html, "onde:") {
		t.Errorf("a mensagem de erro deveria aparecer no cartão:\n%s", html)
	}
}

// TestQueryBlocos_TodasAsApresentacoesAceitas trava a lista de `mostrar:` — se
// uma apresentação nova entrar no processor sem template, o teste acusa.
func TestQueryBlocos_TodasAsApresentacoesAceitas(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/a.md", "# A\n", "")

	for _, mostrar := range []string{
		processor.QueryMostrarLista,
		processor.QueryMostrarContagem,
		processor.QueryMostrarCartoes,
	} {
		blocos, _ := ctx.queryBlocos("x.md", "```consulta\ntipo: notas\nmostrar: "+mostrar+"\n```\n")
		if len(blocos) != 1 || blocos[0].Erro != "" {
			t.Errorf("mostrar: %s → %+v", mostrar, blocos)
		}
		var buf bytes.Buffer
		if err := QueryPanel("x.md", blocos, false).Render(context.Background(), &buf); err != nil {
			t.Errorf("Render(%s): %v", mostrar, err)
		}
	}
}

var _ = query.Result{}
