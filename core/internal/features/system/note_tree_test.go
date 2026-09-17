package system

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── Helpers ──

// treeRow monta uma linha mínima do Tabulator (arquivo + parent + mtime).
func treeRow(arquivo, parent, mtime string) map[string]interface{} {
	r := map[string]interface{}{"arquivo": arquivo}
	if parent != "" {
		r[parentKey] = parent
	}
	if mtime != "" {
		r["mtime"] = mtime
	}
	return r
}

// findRow procura uma linha pelo arquivo (comparação exata).
func findRow(rows []map[string]interface{}, arquivo string) map[string]interface{} {
	for _, r := range rows {
		if rowFile(r) == arquivo {
			return r
		}
	}
	return nil
}

// findNodeDeep procura um nó em qualquer nível da floresta.
func findNodeDeep(rows []map[string]interface{}, arquivo string) map[string]interface{} {
	for _, r := range rows {
		if rowFile(r) == arquivo {
			return r
		}
		if found := findNodeDeep(rawChildren(r), arquivo); found != nil {
			return found
		}
	}
	return nil
}

// rawChildren devolve os filhos de um nó, aceitando tanto os maps montados em
// memória (buildNoteTree) quanto os decodificados de JSON (que viram
// []interface{}).
func rawChildren(node map[string]interface{}) []map[string]interface{} {
	switch list := node[childrenKey].(type) {
	case []map[string]interface{}:
		return list
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(list))
		for _, item := range list {
			if m, ok := item.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

// childFiles devolve os arquivos dos filhos diretos de um nó.
func childFiles(t *testing.T, node map[string]interface{}) []string {
	t.Helper()
	if raw, ok := node[childrenKey]; ok && raw != nil {
		switch raw.(type) {
		case []map[string]interface{}, []interface{}:
		default:
			t.Fatalf("_children com tipo inesperado: %T", raw)
		}
	}
	kids := rawChildren(node)
	files := make([]string, 0, len(kids))
	for _, kid := range kids {
		files = append(files, rowFile(kid))
	}
	return files
}

// ── buildNoteTree ──

func TestBuildNoteTree_Nested(t *testing.T) {
	rows := []map[string]interface{}{
		treeRow("notes/pai.md", "", "2026-01-01 10:00:00"),
		treeRow("notes/filha.md", "pai", "2026-01-02 10:00:00"),
		treeRow("notes/neta.md", "filha", "2026-01-03 10:00:00"),
		treeRow("notes/solta.md", "", "2026-01-04 10:00:00"),
	}

	roots, hasTree := buildNoteTree(rows)
	if !hasTree {
		t.Fatal("esperava hasTree=true")
	}
	if len(roots) != 2 {
		t.Fatalf("esperava 2 raízes, got %d", len(roots))
	}
	if rowFile(roots[0]) != "notes/pai.md" {
		t.Fatalf("primeira raiz deveria preservar a ordem de entrada, got %s", rowFile(roots[0]))
	}

	kids := childFiles(t, roots[0])
	if len(kids) != 1 || kids[0] != "notes/filha.md" {
		t.Fatalf("filhos do pai: %v", kids)
	}

	// 3 níveis: a neta está dentro da filha.
	filha := findNodeDeep(roots, "notes/filha.md")
	if filha == nil {
		t.Fatal("filha não encontrada na árvore")
	}
	neta := childFiles(t, filha)
	if len(neta) != 1 || neta[0] != "notes/neta.md" {
		t.Fatalf("netos: %v", neta)
	}
}

func TestBuildNoteTree_NoHierarchyReturnsInput(t *testing.T) {
	rows := []map[string]interface{}{
		treeRow("notes/a.md", "", "2026-01-01 10:00:00"),
		treeRow("notes/b.md", "", "2026-01-02 10:00:00"),
	}

	roots, hasTree := buildNoteTree(rows)
	if hasTree {
		t.Fatal("sem parent não deveria haver árvore")
	}
	if len(roots) != 2 {
		t.Fatalf("esperava 2 linhas, got %d", len(roots))
	}
	if _, ok := roots[0][childrenKey]; ok {
		t.Error("nenhuma linha deveria ganhar _children")
	}
}

func TestBuildNoteTree_OrphanSelfAndCycleFallBackToRoot(t *testing.T) {
	rows := []map[string]interface{}{
		// Pai inexistente.
		treeRow("notes/orfa.md", "nao-existe", ""),
		// Auto-pai.
		treeRow("notes/ego.md", "ego", ""),
		// Ciclo A ↔ B.
		treeRow("notes/ciclo-a.md", "ciclo-b", ""),
		treeRow("notes/ciclo-b.md", "ciclo-a", ""),
		// Filha legítima de uma nota que está no ciclo.
		treeRow("notes/valida.md", "ciclo-a", ""),
		// Raiz solta.
		treeRow("notes/raiz.md", "", ""),
	}

	roots, hasTree := buildNoteTree(rows)
	if !hasTree {
		t.Fatal("esperava hasTree=true")
	}

	// Órfã e auto-pai voltam para a raiz, sem filhos.
	for _, want := range []string{"notes/orfa.md", "notes/ego.md", "notes/raiz.md"} {
		node := findRow(roots, want)
		if node == nil {
			t.Fatalf("%s deveria estar na raiz", want)
		}
		if kids := childFiles(t, node); len(kids) > 0 {
			t.Fatalf("%s não deveria ter filhos: %v", want, kids)
		}
	}

	// No ciclo, o vínculo que FECHA o laço é descartado e a dupla vira uma
	// cadeia de 2 níveis. O resultado é determinístico: quem decide é a ordem
	// das linhas (aqui, ciclo-a vem antes de ciclo-b), nunca a ordem de
	// iteração de map.
	if findRow(roots, "notes/ciclo-a.md") != nil {
		t.Fatal("ciclo-a deveria ser filha (o vínculo fechado foi o dela)")
	}
	cicloB := findRow(roots, "notes/ciclo-b.md")
	if cicloB == nil {
		t.Fatal("ciclo-b deveria ser a raiz da dupla")
	}
	kids := childFiles(t, cicloB)
	if len(kids) != 1 || kids[0] != "notes/ciclo-a.md" {
		t.Fatalf("ciclo-b deveria ter ciclo-a como filha: %v", kids)
	}

	// E a nota que dependia do ciclo continua na árvore, abaixo dele.
	valida := findNodeDeep(roots, "notes/valida.md")
	if valida == nil {
		t.Fatal("valida.md não deveria ser perdida ao desfazer o ciclo")
	}
	if kids := childFiles(t, findRow(roots, "notes/ciclo-b.md")); len(kids) != 1 {
		t.Fatalf("ciclo-a deveria continuar filha de ciclo-b: %v", kids)
	}
}

// Regressão de um caso real: em YAML, `parent: [[alvo]]` SEM aspas não é
// string — vira uma lista aninhada ([["alvo"]]), e `parent: [alvo]` uma lista
// de um item. As duas formas apareceram em teste manual e precisam continuar
// resolvendo (o valor cai no fallback de texto do parentRefFromMap).
func TestBuildNoteTree_ParentAsYamlList(t *testing.T) {
	rows := []map[string]interface{}{
		treeRow("notes/alvo.md", "", ""),
		{"arquivo": "notes/lista-aninhada.md", parentKey: []interface{}{[]interface{}{"alvo"}}},
		{"arquivo": "notes/lista-simples.md", parentKey: []interface{}{"alvo"}},
	}

	roots, hasTree := buildNoteTree(rows)
	if !hasTree {
		t.Fatal("esperava hasTree=true")
	}
	alvo := findRow(roots, "notes/alvo.md")
	if alvo == nil {
		t.Fatal("alvo deveria ser raiz")
	}
	if kids := childFiles(t, alvo); len(kids) != 2 {
		t.Fatalf("as duas formas de lista deveriam resolver: %v", kids)
	}
	if len(roots) != 1 {
		t.Fatalf("esperava 1 raiz, got %d", len(roots))
	}
}

func TestBuildNoteTree_ReferenceFormats(t *testing.T) {
	rows := []map[string]interface{}{
		treeRow("notes/alvo.md", "", ""),
		treeRow("notes/wikilink.md", "[[Alvo]]", ""),
		treeRow("notes/com-alias.md", "[[alvo|Apelido]]", ""),
		treeRow("notes/com-ancora.md", "[[alvo#secao]]", ""),
		treeRow("notes/com-caminho.md", "notes/ALVO.md", ""),
		treeRow("notes/com-aspas.md", "'alvo'", ""),
	}

	roots, _ := buildNoteTree(rows)

	alvo := findRow(roots, "notes/alvo.md")
	if alvo == nil {
		t.Fatal("alvo deveria ser raiz")
	}
	kids := childFiles(t, alvo)
	if len(kids) != 5 {
		t.Fatalf("esperava 5 filhos (todas as formas de referência), got %v", kids)
	}
	if len(roots) != 1 {
		t.Fatalf("esperava 1 raiz, got %d", len(roots))
	}
}

func TestBuildNoteTree_DuplicateBaseNamePrefersNotesFolder(t *testing.T) {
	rows := []map[string]interface{}{
		// archives/ é mais recente…
		treeRow("archives/seila.md", "", "2026-05-01 10:00:00"),
		// …mas notes/ tem precedência.
		treeRow("notes/seila.md", "", "2020-01-01 10:00:00"),
		treeRow("notes/filha.md", "seila", ""),
	}

	roots, _ := buildNoteTree(rows)
	if len(roots) != 2 {
		t.Fatalf("esperava 2 raízes, got %d", len(roots))
	}
	if kids := childFiles(t, findRow(roots, "notes/seila.md")); len(kids) != 1 {
		t.Fatalf("notes/ deveria vencer a disputa pelo nome: %v", kids)
	}
	if raw, ok := findRow(roots, "archives/seila.md")[childrenKey]; ok {
		t.Errorf("archives/ não deveria receber a filha (raw=%v)", raw)
	}
}

func TestBuildNoteTree_DuplicateBaseNameTieBreaksByMtime(t *testing.T) {
	rows := []map[string]interface{}{
		treeRow("pdfs/relatorio.md", "", "2026-01-01 10:00:00"),
		treeRow("archives/relatorio.md", "", "2026-06-01 10:00:00"),
		treeRow("notes/filha.md", "relatorio", ""),
	}

	roots, _ := buildNoteTree(rows)
	if kids := childFiles(t, findRow(roots, "archives/relatorio.md")); len(kids) != 1 {
		t.Fatalf("empate de pasta deveria escolher o mtime mais recente: %v", kids)
	}
}

// Regressão: as linhas do cache são guardadas por REFERÊNCIA de map. Se
// buildNoteTree escrevesse "_children" nelas, o segundo request encontraria o
// campo já populado e a tabela mostraria filhos duplicados.
func TestBuildNoteTree_DoesNotMutateInputRows(t *testing.T) {
	rows := []map[string]interface{}{
		treeRow("notes/pai.md", "", ""),
		treeRow("notes/filha.md", "pai", ""),
	}

	first, _ := buildNoteTree(rows)
	if _, ok := rows[0]["_children"]; ok {
		t.Fatal("buildNoteTree não pode escrever _children na linha de entrada")
	}

	second, _ := buildNoteTree(rows)
	if len(second) != 1 {
		t.Fatalf("segunda chamada: esperava 1 raiz, got %d", len(second))
	}
	if kids := childFiles(t, second[0]); len(kids) != 1 {
		t.Fatalf("segunda chamada: filhos duplicados (%v)", kids)
	}
	if len(childFiles(t, first[0])) != 1 {
		t.Error("o resultado da primeira chamada foi alterado pela segunda")
	}
}

// ── Integração com o handler do Tabulator ──

type databaseMeta struct {
	HasTree bool `json:"hasTree"`
	Total   int  `json:"total"`
}

type databaseResponse struct {
	Columns []map[string]interface{} `json:"columns"`
	Data    []map[string]interface{} `json:"data"`
	Meta    databaseMeta             `json:"meta"`
}

func fetchDatabase(t *testing.T, ctx *HandlerContext) databaseResponse {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/notes/database", nil)
	rr := httptest.NewRecorder()
	ctx.HandleGetDatabaseData(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var resp databaseResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp
}

func TestHandleGetDatabaseData_Tree(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/pai.md", "---\ntags: [x]\n---\n# Pai", "")
	// Chave LEGADA de propósito: a linha precisa chegar ao Tabulator já
	// normalizada em "pai" (DECISIONS 6.19).
	saveTestNote(t, ctx, "notes/filha.md", "---\nparent: pai\n---\n# Filha", "")

	resp := fetchDatabase(t, ctx)
	if !resp.Meta.HasTree {
		t.Fatal("esperava meta.hasTree=true")
	}
	if len(resp.Data) != 1 {
		t.Fatalf("esperava 1 raiz, got %d", len(resp.Data))
	}
	if rowFile(resp.Data[0]) != "notes/pai.md" {
		t.Fatalf("raiz inesperada: %s", rowFile(resp.Data[0]))
	}
	kids := childFiles(t, resp.Data[0])
	if len(kids) != 1 || kids[0] != "notes/filha.md" {
		t.Fatalf("filhos: %v", kids)
	}

	// A linha da filha usa a chave canônica (a legada não pode chegar ao
	// frontend, senão o Tabulator mostraria duas colunas de hierarquia).
	filha := findNodeDeep(resp.Data, "notes/filha.md")
	if filha == nil {
		t.Fatal("filha não encontrada na árvore")
	}
	if got, _ := filha[parentKey].(string); got != "pai" {
		t.Fatalf("linha da filha deveria usar %q, got %v", parentKey, filha[parentKey])
	}
	if _, ok := filha[parentKeyLegacy]; ok {
		t.Errorf("chave legada %q não deveria chegar ao payload", parentKeyLegacy)
	}

	// "_children" é campo interno: não pode virar coluna, mas a coluna de
	// hierarquia deve aparecer com o rótulo "Pai" (é o que o usuário edita).
	hasParentCol := false
	for _, col := range resp.Columns {
		field, _ := col["field"].(string)
		if field == childrenKey {
			t.Errorf("campo interno %s exposto como coluna", childrenKey)
		}
		if field != parentKey {
			continue
		}
		hasParentCol = true
		if title, _ := col["title"].(string); title != "Pai" {
			t.Errorf("rótulo da coluna de hierarquia: esperava \"Pai\", got %q", title)
		}
	}
	if !hasParentCol {
		t.Errorf("esperava a coluna %q no Tabulator", parentKey)
	}
	if _, ok := treeColumn(resp.Columns, "abrir_link"); !ok {
		t.Error("a coluna abrir_link deveria continuar existindo (oculta por padrão)")
	}

	// Regressão do cache: o segundo request (cache hit) não pode acumular filhos.
	again := fetchDatabase(t, ctx)
	if len(again.Data) != 1 {
		t.Fatalf("segundo request: esperava 1 raiz, got %d", len(again.Data))
	}
	if kids := childFiles(t, again.Data[0]); len(kids) != 1 {
		t.Fatalf("segundo request: filhos duplicados (%v)", kids)
	}
}

// treeColumn devolve a definição de uma coluna pelo field.
func treeColumn(columns []map[string]interface{}, field string) (map[string]interface{}, bool) {
	for _, col := range columns {
		if f, _ := col["field"].(string); f == field {
			return col, true
		}
	}
	return nil, false
}

// A coluna "Abrir" nasceu como link explícito e virou redundante com o duplo
// clique na linha: ela passa a nascer oculta (mas continua no seletor).
func TestHandleGetDatabaseData_AbrirColumnHiddenByDefault(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/a.md", "# A", "")

	resp := fetchDatabase(t, ctx)
	col, ok := treeColumn(resp.Columns, "abrir_link")
	if !ok {
		t.Fatal("esperava a coluna abrir_link")
	}
	if visible, ok := col["visible"].(bool); !ok || visible {
		t.Errorf("abrir_link deveria nascer oculta, got visible=%v", col["visible"])
	}
}

func TestHandleGetDatabaseData_FlatWhenNoParent(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/a.md", "# A", "")
	saveTestNote(t, ctx, "notes/b.md", "# B", "")

	resp := fetchDatabase(t, ctx)
	if resp.Meta.HasTree {
		t.Error("sem parent, hasTree deveria ser false")
	}
	if len(resp.Data) != 2 {
		t.Fatalf("esperava 2 linhas, got %d", len(resp.Data))
	}
	if resp.Meta.Total != 2 {
		t.Fatalf("meta.total deveria contar todas as notas, got %d", resp.Meta.Total)
	}

	// A coluna de hierarquia existe mesmo sem nenhuma nota aninhada: é por ela
	// que a primeira hierarquia é criada direto na tabela.
	hasParentCol := false
	for _, col := range resp.Columns {
		if field, _ := col["field"].(string); field == parentKey {
			hasParentCol = true
		}
	}
	if !hasParentCol {
		t.Errorf("esperava a coluna %q mesmo sem hierarquia", parentKey)
	}
}

// ── Validação da gravação ──

func postProperty(t *testing.T, ctx *HandlerContext, file, key string, value interface{}) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(map[string]interface{}{"file": file, "key": key, "value": value})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest("POST", "/api/notes/update-property", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	ctx.HandleUpdateNoteProperty(rr, req)
	return rr
}

func TestHandleUpdateNoteProperty_ParentNormalizesValue(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/pai.md", "# Pai", "")
	saveTestNote(t, ctx, "notes/filha.md", "# Filha", "")

	rr := postProperty(t, ctx, "notes/filha.md", parentKey, "[[PAI]]")
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rr.Code, rr.Body.String())
	}

	content, err := ctx.Store.GetNote("notes/filha.md")
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if !strings.Contains(content, parentKey+": pai") {
		t.Fatalf("esperava o valor canônico no frontmatter, got:\n%s", content)
	}

	// E a nota aparece aninhada.
	resp := fetchDatabase(t, ctx)
	if kids := childFiles(t, findRow(resp.Data, "notes/pai.md")); len(kids) != 1 {
		t.Fatalf("filha não foi aninhada: %v", kids)
	}
}

// A chave antiga continua sendo aceita na gravação (clientes/navegadores com o
// bundle antigo em cache), mas o frontmatter é migrado para a canônica.
func TestHandleUpdateNoteProperty_LegacyParentKeyMigratesToPai(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/alvo.md", "# Alvo", "")
	saveTestNote(t, ctx, "notes/filha.md", "---\nparent: outro\n---\n# Filha", "")

	rr := postProperty(t, ctx, "notes/filha.md", parentKeyLegacy, "alvo")
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rr.Code, rr.Body.String())
	}

	content, _ := ctx.Store.GetNote("notes/filha.md")
	if !strings.Contains(content, parentKey+": alvo") {
		t.Fatalf("esperava a chave canônica no frontmatter, got:\n%s", content)
	}
	if strings.Contains(content, parentKeyLegacy+":") {
		t.Fatalf("a chave legada deveria ter sido removida:\n%s", content)
	}

	resp := fetchDatabase(t, ctx)
	if kids := childFiles(t, findRow(resp.Data, "notes/alvo.md")); len(kids) != 1 {
		t.Fatalf("filha não foi aninhada no novo pai: %v", kids)
	}
}

// Remover o vínculo apaga as DUAS chaves (canônica e legada) — sem isso a nota
// continuaria aninhada por causa do valor antigo.
func TestHandleUpdateNoteProperty_RemovalClearsLegacyKey(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/alvo.md", "# Alvo", "")
	saveTestNote(t, ctx, "notes/filha.md", "---\npai: alvo\nparent: alvo\n---\n# Filha", "")

	rr := postProperty(t, ctx, "notes/filha.md", parentKey, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rr.Code, rr.Body.String())
	}

	content, _ := ctx.Store.GetNote("notes/filha.md")
	if strings.Contains(content, parentKey+":") || strings.Contains(content, parentKeyLegacy+":") {
		t.Fatalf("as duas chaves deveriam ter sido removidas:\n%s", content)
	}

	resp := fetchDatabase(t, ctx)
	if resp.Meta.HasTree {
		t.Error("sem vínculo não deveria haver árvore")
	}
}

// normalizeParentKey é o que garante uma única coluna de hierarquia no
// Tabulator quando a nota ainda usa o nome antigo da chave.
func TestNormalizeParentKey(t *testing.T) {
	cases := []struct {
		name string
		row  map[string]interface{}
		want interface{}
	}{
		{"canônica fica como está", map[string]interface{}{"pai": "alvo"}, "alvo"},
		{"legada é movida", map[string]interface{}{"parent": "alvo"}, "alvo"},
		{"caixa alta da canônica", map[string]interface{}{"PAI": "alvo"}, "alvo"},
		{"caixa alta da legada", map[string]interface{}{"Parent": "alvo"}, "alvo"},
		{"canônica vence a legada", map[string]interface{}{"pai": "certa", "parent": "antiga"}, "certa"},
		{"sem chave nenhuma", map[string]interface{}{"titulo": "x"}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			normalizeParentKey(tc.row)
			if got := tc.row[parentKey]; got != tc.want {
				t.Fatalf("pai = %v, esperava %v", got, tc.want)
			}
			for k := range tc.row {
				if strings.EqualFold(k, parentKeyLegacy) {
					t.Fatalf("chave legada %q sobreviveu: %v", k, tc.row)
				}
			}
		})
	}
}

func TestHandleUpdateNoteProperty_ParentRejectsSelfAndCycle(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/a.md", "---\nparent: b\n---\n# A", "")
	saveTestNote(t, ctx, "notes/b.md", "# B", "")

	// Auto-pai.
	rr := postProperty(t, ctx, "notes/a.md", parentKey, "a")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("auto-pai deveria ser recusado, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "pai dela mesma") {
		t.Errorf("mensagem inesperada: %s", rr.Body.String())
	}

	// Ciclo: "a" já está abaixo de "b".
	rr = postProperty(t, ctx, "notes/b.md", parentKey, "a")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("ciclo deveria ser recusado, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "ciclo") {
		t.Errorf("mensagem inesperada: %s", rr.Body.String())
	}

	// E a hierarquia original continua intacta.
	resp := fetchDatabase(t, ctx)
	if kids := childFiles(t, findRow(resp.Data, "notes/b.md")); len(kids) != 1 || kids[0] != "notes/a.md" {
		t.Fatalf("hierarquia deveria continuar b > a, got %v", kids)
	}
}

func TestHandleUpdateNoteProperty_ParentAllowsMissingAndRemoval(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/orfa.md", "# Órfã", "")

	// Pai inexistente é permitido de propósito (a nota fica na raiz).
	rr := postProperty(t, ctx, "notes/orfa.md", parentKey, "ainda-nao-existe")
	if rr.Code != http.StatusOK {
		t.Fatalf("pai inexistente deveria ser aceito, got %d (%s)", rr.Code, rr.Body.String())
	}
	content, _ := ctx.Store.GetNote("notes/orfa.md")
	if !strings.Contains(content, parentKey+": ainda-nao-existe") {
		t.Fatalf("frontmatter inesperado:\n%s", content)
	}
	resp := fetchDatabase(t, ctx)
	if resp.Meta.HasTree {
		t.Error("vínculo sem pai existente não deve gerar árvore")
	}

	// Valor vazio remove a propriedade.
	rr = postProperty(t, ctx, "notes/orfa.md", parentKey, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("remoção deveria ser aceita, got %d (%s)", rr.Code, rr.Body.String())
	}
	content, _ = ctx.Store.GetNote("notes/orfa.md")
	if strings.Contains(content, parentKey) {
		t.Fatalf("propriedade deveria ter sido removida:\n%s", content)
	}
}
