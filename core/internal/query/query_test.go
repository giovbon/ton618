package query

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"ton618/core/internal/core/db"
	"ton618/core/internal/processor"
)

var errFake = errors.New("falha de leitura")

// ---------------------------------------------------------------------------
// Motor + executor da linguagem de consulta (DECISIONS §6.29).
//
// Os testes usam um Reader FALSO: a interface é a costura do motor, e o que
// está sob teste aqui é filtro/ordenação/limite/erro — a leitura do SQLite já
// tem os testes dela em internal/core/db.
// ---------------------------------------------------------------------------

var agoraFixo = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

type fakeReader struct {
	files    []db.FileModTag
	contents map[string]string
	links    map[string][]string
	pop      map[string]int
	last     map[string]string
	todos    []processor.TodoItem
	fts      map[string][]db.FTSResult
	hier     Hierarchy
	erro     error
}

func (f *fakeReader) Files() ([]db.FileModTag, error) {
	if f.erro != nil {
		return nil, f.erro
	}
	return f.files, nil
}

func (f *fakeReader) Contents() (map[string]string, error) {
	if f.contents == nil {
		return map[string]string{}, nil
	}
	return f.contents, nil
}

func (f *fakeReader) Links() (map[string][]string, error) {
	if f.links == nil {
		return map[string][]string{}, nil
	}
	return f.links, nil
}

func (f *fakeReader) Popularity() (map[string]int, error) {
	if f.pop == nil {
		return map[string]int{}, nil
	}
	return f.pop, nil
}

func (f *fakeReader) LastInteracted() (map[string]string, error) {
	if f.last == nil {
		return map[string]string{}, nil
	}
	return f.last, nil
}

func (f *fakeReader) Todos(_ map[string]bool, status string) ([]processor.TodoItem, error) {
	if status == "" || status == "all" {
		return f.todos, nil
	}
	var out []processor.TodoItem
	for _, t := range f.todos {
		if t.Status == status {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeReader) SearchText(term string, _ int) ([]db.FTSResult, error) {
	return f.fts[term], nil
}

func (f *fakeReader) Hierarchy() (Hierarchy, error) { return f.hier, nil }

// novoLeitor devolve um acervo pequeno e determinístico.
func novoLeitor() *fakeReader {
	return &fakeReader{
		files: []db.FileModTag{
			{Arquivo: "notes/alfa.md", Mtime: "2026-10-01T10:00:00Z", Tags: "projeto,ativo"},
			{Arquivo: "notes/beta.md", Mtime: "2026-09-01T10:00:00Z", Tags: "livros"},
			{Arquivo: "notes/gama.md", Mtime: "2026-08-01T10:00:00Z", Tags: ""},
			{Arquivo: "pdfs/livro.pdf", Mtime: "2026-07-01T10:00:00Z", Tags: "livros"},
		},
		contents: map[string]string{
			"notes/alfa.md": "# Alfa\n\npai: notes/gama.md\n",
			"notes/beta.md": "# Beta\n",
			"notes/gama.md": "# Gama\n",
		},
		links: map[string][]string{
			"notes/alfa.md": {"notes/beta.md"},
			"notes/beta.md": {"notes/beta.md"},
		},
		pop:  map[string]int{"notes/alfa.md": 7},
		last: map[string]string{"notes/alfa.md": "2026-09-01T10:00:00Z"},
		todos: []processor.TodoItem{
			{ID: "1", File: "notes/alfa.md", Type: "TODO", Status: "pending", Text: "publicar nota", Section: "Geral", Line: 3, Created: agoraFixo.AddDate(0, 0, -2)},
			{ID: "2", File: "notes/alfa.md", Type: "FIXME", Status: "pending", Text: "revisar tabela", Section: "Geral", Line: 9, Created: agoraFixo.AddDate(0, 0, -30)},
			{ID: "3", File: "notes/beta.md", Type: "TODO", Status: "completed", Text: "já feito", Section: "Geral", Line: 2, Created: agoraFixo.AddDate(0, 0, -5)},
		},
		fts: map[string][]db.FTSResult{
			"projeto": {{Arquivo: "notes/alfa.md", Snippet: "…<b>projeto</b>…"}},
		},
		hier: Hierarchy{
			Pai:       map[string]string{"notes/alfa.md": "notes/gama.md"},
			Resolvido: map[string]string{"notes/alfa.md": "notes/gama.md"},
			Filhas: map[string][]string{
				"notes/gama.md": {"notes/alfa.md"},
			},
		},
	}
}

func executar(t *testing.T, src, nota string, r Reader) Result {
	t.Helper()
	spec, err := processor.ParseQuery(src)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", src, err)
	}
	return Run(context.Background(), NewEngine(), spec, r, nota, agoraFixo)
}

func caminhos(rows []Row) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Caminho)
	}
	return out
}

// ---------------------------------------------------------------------------
// tipo: notas
// ---------------------------------------------------------------------------

func TestRun_Notas_TagsEOrdenacao(t *testing.T) {
	res := executar(t, "tipo: notas\ntags: [livros]\nordenar: mtime desc", "notes/x.md", novoLeitor())
	if res.Erro != "" {
		t.Fatalf("erro inesperado: %s", res.Erro)
	}
	if got := strings.Join(caminhos(res.Rows), ","); got != "notes/beta.md,pdfs/livro.pdf" {
		t.Errorf("linhas = %q", got)
	}
	if res.Total != 2 {
		t.Errorf("Total = %d; quero 2", res.Total)
	}
}

func TestRun_Notas_SemTagsEPasta(t *testing.T) {
	res := executar(t, "tipo: notas\nsem-tags: [livros, projeto]\npasta: notes", "notes/x.md", novoLeitor())
	if res.Erro != "" {
		t.Fatalf("erro inesperado: %s", res.Erro)
	}
	if got := strings.Join(caminhos(res.Rows), ","); got != "notes/gama.md" {
		t.Errorf("linhas = %q", got)
	}
}

func TestRun_Notas_TextoUsaFTS(t *testing.T) {
	res := executar(t, "tipo: notas\ntexto: projeto", "notes/x.md", novoLeitor())
	if res.Erro != "" {
		t.Fatalf("erro inesperado: %s", res.Erro)
	}
	if got := strings.Join(caminhos(res.Rows), ","); got != "notes/alfa.md" {
		t.Errorf("linhas = %q", got)
	}
	if res.Rows[0].Snippet == "" {
		t.Errorf("snippet não propagado para mostrar: cartoes")
	}
}

func TestRun_Notas_CamposCalculados(t *testing.T) {
	leitor := novoLeitor()
	res := executar(t, "tipo: notas\nordenar: citada_por desc", "notes/x.md", leitor)
	if res.Erro != "" {
		t.Fatalf("erro inesperado: %s", res.Erro)
	}
	porCaminho := map[string]Row{}
	for _, r := range res.Rows {
		porCaminho[r.Caminho] = r
	}
	beta := porCaminho["notes/beta.md"]
	if beta.CitadaPor != 2 {
		t.Errorf("beta.CitadaPor = %d; quero 2 (alfa e ela mesma)", beta.CitadaPor)
	}
	alfa := porCaminho["notes/alfa.md"]
	if alfa.TarefasAbertas != 2 {
		t.Errorf("alfa.TarefasAbertas = %d; quero 2", alfa.TarefasAbertas)
	}
	if alfa.Popularidade != 7 {
		t.Errorf("alfa.Popularidade = %d; quero 7", alfa.Popularidade)
	}
	if !alfa.PaiResolvido || alfa.Pai != "notes/gama.md" {
		t.Errorf("alfa.Pai/PaiResolvido = %q/%v", alfa.Pai, alfa.PaiResolvido)
	}
	gama := porCaminho["notes/gama.md"]
	if len(gama.Filhas) != 1 || gama.Filhas[0] != "alfa" {
		t.Errorf("gama.Filhas = %v; quero [alfa]", gama.Filhas)
	}
}

func TestRun_Notas_OndeCEL(t *testing.T) {
	casos := []struct {
		nome  string
		onde  string
		quero string
	}{
		{"tag in", `onde: '"projeto" in n.tags'`, "notes/alfa.md"},
		{"macros", `n.tags.exists(t, t.startsWith("liv"))`, "notes/beta.md,pdfs/livro.pdf"},
		{"negacao", `onde: '!("lido" in n.tags) && n.pasta == "notes"'`, "notes/alfa.md,notes/beta.md,notes/gama.md"},
		{"data", `dias(n.mtime) > 30`, "notes/beta.md,notes/gama.md,pdfs/livro.pdf"},
		{"null", `n.interacao == null`, "notes/beta.md,notes/gama.md,pdfs/livro.pdf"},
		{"tamanho lista", `n.filhas.size() > 0`, "notes/gama.md"},
		{"regex", `n.caminho.matches("^pdfs/")`, "pdfs/livro.pdf"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			onde := c.onde
			if !strings.HasPrefix(onde, "onde:") {
				onde = "onde: " + onde
			}
			res := executar(t, "tipo: notas\nordenar: caminho asc\n"+onde, "notes/x.md", novoLeitor())
			if res.Erro != "" {
				t.Fatalf("erro inesperado: %s", res.Erro)
			}
			if got := strings.Join(caminhos(res.Rows), ","); got != c.quero {
				t.Errorf("linhas = %q; quero %q", got, c.quero)
			}
		})
	}
}

func TestRun_LimiteETruncamento(t *testing.T) {
	res := executar(t, "tipo: notas\nlimite: 2", "notes/x.md", novoLeitor())
	if res.Erro != "" {
		t.Fatalf("erro inesperado: %s", res.Erro)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("linhas = %d; quero 2", len(res.Rows))
	}
	if res.Total != 4 || !res.Truncado {
		t.Errorf("Total = %d, Truncado = %v; quero 4/true", res.Total, res.Truncado)
	}
}

func TestRun_TetoDeCandidatos(t *testing.T) {
	leitor := novoLeitor()
	leitor.files = nil
	for i := 0; i < MaxCandidatos+80; i++ {
		leitor.files = append(leitor.files, db.FileModTag{
			Arquivo: "notes/n" + strconv.Itoa(i) + ".md",
			Mtime:   "2026-10-01T10:00:00Z",
		})
	}
	res := executar(t, "tipo: notas\nlimite: 5", "notes/x.md", leitor)
	if res.Erro != "" {
		t.Fatalf("erro inesperado: %s", res.Erro)
	}
	if res.Candidatos != MaxCandidatos || !res.Capado {
		t.Errorf("Candidatos = %d, Capado = %v; quero %d/true", res.Candidatos, res.Capado, MaxCandidatos)
	}
}

func TestRun_ContagemUsaTotal(t *testing.T) {
	res := executar(t, "tipo: notas\nmostrar: contagem\nlimite: 1", "notes/x.md", novoLeitor())
	if res.Erro != "" {
		t.Fatalf("erro inesperado: %s", res.Erro)
	}
	if res.Total != 4 {
		t.Errorf("Total = %d; quero 4 (o Total é antes do limite)", res.Total)
	}
}

// ---------------------------------------------------------------------------
// Erros — sempre conteúdo, nunca erro Go
// ---------------------------------------------------------------------------

func TestRun_ErrosDeOnde(t *testing.T) {
	casos := []struct {
		nome   string
		onde   string
		trecho string
	}{
		{"campo desconhecido", "n.velocidade > 1", `campo desconhecido "velocidade"`},
		{"campo desconhecido lista", "n.velocidade > 1", "campos de n:"},
		{"sintaxe", "n.tags ==", "onde:"},
		{"tipo incompatível", "n.tags + 3 > 1", "tipo incompatível"},
		{"não booleano", "n.titulo", "verdadeiro ou falso"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			res := executar(t, "tipo: notas\nonde: "+c.onde, "notes/x.md", novoLeitor())
			if res.Erro == "" {
				t.Fatalf("esperava mensagem no cartão")
			}
			if !strings.Contains(res.Erro, c.trecho) {
				t.Fatalf("mensagem %q não contém %q", res.Erro, c.trecho)
			}
			if len(res.Rows) != 0 {
				t.Errorf("erro deve zerar as linhas; veio %d", len(res.Rows))
			}
		})
	}
}

func TestRun_ErroDeLeitura(t *testing.T) {
	leitor := novoLeitor()
	leitor.erro = errFake
	res := executar(t, "tipo: notas", "notes/x.md", leitor)
	if !strings.Contains(res.Erro, "não foi possível ler o acervo") {
		t.Fatalf("mensagem = %q", res.Erro)
	}
}

func TestRun_DeterminismoComAgoraInjetado(t *testing.T) {
	src := "tipo: notas\nonde: dias(n.mtime) > 30\nordenar: caminho asc"
	spec, err := processor.ParseQuery(src)
	if err != nil {
		t.Fatalf("ParseQuery: %v", err)
	}
	eng := NewEngine()

	primeiro := Run(context.Background(), eng, spec, novoLeitor(), "notes/x.md", agoraFixo)
	segundo := Run(context.Background(), eng, spec, novoLeitor(), "notes/x.md", agoraFixo)
	if strings.Join(caminhos(primeiro.Rows), ",") != strings.Join(caminhos(segundo.Rows), ",") {
		t.Fatalf("mesma entrada e mesmo agora deram resultados diferentes")
	}
	if len(primeiro.Rows) == 0 {
		t.Fatalf("a consulta de referência deveria trazer linhas")
	}

	// 60 dias antes, quase nada passou do corte de 30 dias: o filtro segue `agora`.
	passado := agoraFixo.AddDate(0, 0, -60)
	terceiro := Run(context.Background(), eng, spec, novoLeitor(), "notes/x.md", passado)
	if len(terceiro.Rows) >= len(primeiro.Rows) {
		t.Errorf("com agora mais antigo o filtro deveria cortar linhas: %d → %d",
			len(primeiro.Rows), len(terceiro.Rows))
	}
}

func TestRun_LimiteDeCusto(t *testing.T) {
	tags := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		tags = append(tags, "t"+strconv.Itoa(i))
	}
	leitor := novoLeitor()
	leitor.files[0].Tags = strings.Join(tags, ",")

	// 40³ = 64.000 passos: acima do teto de custo de propósito.
	onde := `n.tags.map(a, n.tags.map(b, n.tags.map(c, a + b + c))).size() > 0`
	res := executar(t, "tipo: notas\nonde: "+onde, "notes/x.md", leitor)
	if res.Erro == "" {
		t.Fatalf("esperava estouro de custo/tempo")
	}
	if !strings.Contains(res.Erro, "custo") && !strings.Contains(res.Erro, "tempo esgotado") {
		t.Fatalf("mensagem = %q", res.Erro)
	}
}

// TestColunas_CobremAListaCanonica garante que TODA coluna aceita no `colunas:`
// tem renderização no motor. Uma coluna "válida" que sai vazia é erro silencioso
// (o usuário vê a tabela com uma célula em branco e acha que o dado não existe).
func TestColunas_CobremAListaCanonica(t *testing.T) {
	row := Row{
		Caminho: "notes/x.md", Titulo: "x", Pasta: "notes", Classe: "nota",
		Tags: []string{"a"}, Mtime: agoraFixo, Pai: "y", PaiResolvido: true,
		Filhas: []string{"z"}, CitadaPor: 1, TarefasAbertas: 2, Popularidade: 3,
		Interacao: agoraFixo, Texto: "t", Marcador: "TODO", Estado: "pendente",
		Secao: "Geral", Linha: 7, Criado: agoraFixo,
	}
	for _, c := range processor.QueryColunas() {
		if v := row.Coluna(c); v == "" {
			t.Errorf("coluna %q não tem renderização (sai vazia)", c)
		}
	}
}

// ---------------------------------------------------------------------------
// tipo: tarefas
// ---------------------------------------------------------------------------

func TestRun_Tarefas(t *testing.T) {
	res := executar(t, "tipo: tarefas\nmarcador: [TODO]\nestado: pendente\nordenar: linha asc", "notes/x.md", novoLeitor())
	if res.Erro != "" {
		t.Fatalf("erro inesperado: %s", res.Erro)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("linhas = %d; quero 1", len(res.Rows))
	}
	row := res.Rows[0]
	if row.Texto != "publicar nota" || row.Marcador != "TODO" || row.Estado != "pendente" {
		t.Errorf("linha = %+v", row)
	}
	if row.Titulo != "alfa" {
		t.Errorf("nota_titulo = %q; quero alfa", row.Titulo)
	}
}

func TestRun_Tarefas_OndeCEL(t *testing.T) {
	res := executar(t, `tipo: tarefas
estado: todas
onde: t.texto.contains("tabela") || t.linha > 5
`, "notes/x.md", novoLeitor())
	if res.Erro != "" {
		t.Fatalf("erro inesperado: %s", res.Erro)
	}
	if len(res.Rows) != 1 || res.Rows[0].Texto != "revisar tabela" {
		t.Fatalf("linhas = %+v", res.Rows)
	}
}

func TestRun_Tarefas_Concluidas(t *testing.T) {
	res := executar(t, "tipo: tarefas\nestado: concluida", "notes/x.md", novoLeitor())
	if res.Erro != "" {
		t.Fatalf("erro inesperado: %s", res.Erro)
	}
	if len(res.Rows) != 1 || res.Rows[0].Texto != "já feito" {
		t.Fatalf("linhas = %+v", res.Rows)
	}
}

// ---------------------------------------------------------------------------
// tipo: hierarquia
// ---------------------------------------------------------------------------

func TestRun_Hierarquia_DaPropriaNota(t *testing.T) {
	res := executar(t, "tipo: hierarquia", "notes/gama.md", novoLeitor())
	if res.Erro != "" {
		t.Fatalf("erro inesperado: %s", res.Erro)
	}
	if got := strings.Join(caminhos(res.Rows), ","); got != "notes/alfa.md" {
		t.Errorf("linhas = %q; quero as filhas de gama", got)
	}
	if res.Rows[0].Nivel != 1 {
		t.Errorf("Nivel = %d; quero 1", res.Rows[0].Nivel)
	}
}

func TestRun_Hierarquia_DeComNome(t *testing.T) {
	res := executar(t, "tipo: hierarquia\nde: gama", "notes/alfa.md", novoLeitor())
	if res.Erro != "" {
		t.Fatalf("erro inesperado: %s", res.Erro)
	}
	if got := strings.Join(caminhos(res.Rows), ","); got != "notes/alfa.md" {
		t.Errorf("linhas = %q", got)
	}
}

func TestRun_Hierarquia_DeInexistente(t *testing.T) {
	res := executar(t, "tipo: hierarquia\nde: não existe", "notes/alfa.md", novoLeitor())
	if !strings.Contains(res.Erro, "nota não encontrada") {
		t.Fatalf("mensagem = %q", res.Erro)
	}
}

func TestRun_Hierarquia_PreservaEstruturaDeArvore(t *testing.T) {
	leitor := novoLeitor()
	leitor.files = append(leitor.files,
		db.FileModTag{Arquivo: "notes/z_filha.md", Mtime: "2026-10-01T10:00:00Z"},
		db.FileModTag{Arquivo: "notes/a_neta.md", Mtime: "2026-10-01T10:00:00Z"},
	)
	leitor.contents["notes/z_filha.md"] = "# Z Filha\npai: notes/gama.md\n"
	leitor.contents["notes/a_neta.md"] = "# A Neta\npai: notes/z_filha.md\n"
	leitor.hier.Filhas["notes/gama.md"] = []string{"notes/z_filha.md"}
	leitor.hier.Filhas["notes/z_filha.md"] = []string{"notes/a_neta.md"}

	res := executar(t, "tipo: hierarquia", "notes/gama.md", leitor)
	if res.Erro != "" {
		t.Fatalf("erro inesperado: %s", res.Erro)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("esperava 2 linhas, vieram %d", len(res.Rows))
	}
	if res.Rows[0].Caminho != "notes/z_filha.md" || res.Rows[0].Nivel != 1 {
		t.Errorf("linha 0 = %s (Nível %d); esperava z_filha Nível 1", res.Rows[0].Caminho, res.Rows[0].Nivel)
	}
	if res.Rows[1].Caminho != "notes/a_neta.md" || res.Rows[1].Nivel != 2 {
		t.Errorf("linha 1 = %s (Nível %d); esperava a_neta Nível 2", res.Rows[1].Caminho, res.Rows[1].Nivel)
	}
}

func TestRun_Tarefas_OrdenarNotaTitulo(t *testing.T) {
	res := executar(t, "tipo: tarefas\nestado: todas\nordenar: nota_titulo asc", "notes/x.md", novoLeitor())
	if res.Erro != "" {
		t.Fatalf("erro inesperado: %s", res.Erro)
	}
	if len(res.Rows) < 2 {
		t.Fatalf("esperava ao menos 2 tarefas")
	}
	if res.Rows[0].Titulo != "alfa" {
		t.Errorf("primeira tarefa esperada de alfa; veio %s", res.Rows[0].Titulo)
	}
}
