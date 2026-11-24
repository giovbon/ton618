package notes

import (
	"strings"
	"testing"
)

// ── GetHierarchy / GetChildrenCounts (hierarquia de notas) ──

func TestGetHierarchy_PaiEFilhas(t *testing.T) {
	pai := "notes/projeto.md"
	filhaA := "notes/filha-a.md"
	filhaB := "notes/filha-b.md"

	notas := map[string]string{
		pai:                "---\ntitle: Projeto\n---\nConteúdo do pai.\n",
		filhaA:             "---\npai: projeto\n---\nConteúdo A.\n",
		filhaB:             "---\npai: \"[[Projeto]]\"\n---\nConteúdo B.\n",
		"notes/neta.md":    "---\npai: filha-a\n---\nConteúdo da neta.\n",
		"notes/orfa.md":    "---\npai: inexistente\n---\nÓrfã.\n",
		"notes/sem-pai.md": "---\ntitle: Sem pai\n---\nSem hierarquia.\n",
	}

	svc, _ := newMockServiceForParent(t, notas)

	parent, children, err := svc.GetHierarchy(pai)
	if err != nil {
		t.Fatalf("GetHierarchy: %v", err)
	}
	if parent != "" {
		t.Errorf("nota raiz deveria ter pai vazio, got %q", parent)
	}
	if len(children) != 2 {
		t.Fatalf("esperado 2 filhas (wikilink e chave simples), got %d: %+v", len(children), children)
	}
	// Ordenação determinística por nome de exibição.
	if children[0].DisplayName != "filha-a" || children[1].DisplayName != "filha-b" {
		t.Errorf("ordem inesperada: %+v", children)
	}
	if children[0].Filename != filhaA || children[1].Filename != filhaB {
		t.Errorf("arquivos inesperados: %+v", children)
	}

	// A filha A tem pai "projeto" e uma filha (a neta).
	parentA, childrenA, err := svc.GetHierarchy(filhaA)
	if err != nil {
		t.Fatalf("GetHierarchy(filha-a): %v", err)
	}
	if parentA != "projeto" {
		t.Errorf("pai da filha-a: esperado \"projeto\", got %q", parentA)
	}
	if len(childrenA) != 1 || childrenA[0].DisplayName != "neta" {
		t.Errorf("filhas da filha-a: esperado [neta], got %+v", childrenA)
	}

	// A neta está a 2 níveis: seu pai é a filha-a.
	parentNeta, childrenNeta, err := svc.GetHierarchy("notes/neta.md")
	if err != nil {
		t.Fatalf("GetHierarchy(neta): %v", err)
	}
	if parentNeta != "filha-a" {
		t.Errorf("pai da neta: esperado \"filha-a\", got %q", parentNeta)
	}
	if len(childrenNeta) != 0 {
		t.Errorf("neta não deveria ter filhas, got %+v", childrenNeta)
	}
}

func TestGetHierarchy_ChaveLegadaParent(t *testing.T) {
	notas := map[string]string{
		"notes/pai.md":   "---\ntitle: Pai\n---\n",
		"notes/filha.md": "---\nparent: PAI\n---\nConteúdo.\n",
	}
	svc, _ := newMockServiceForParent(t, notas)

	_, children, err := svc.GetHierarchy("notes/pai.md")
	if err != nil {
		t.Fatalf("GetHierarchy: %v", err)
	}
	if len(children) != 1 || children[0].Filename != "notes/filha.md" {
		t.Errorf("chave legada `parent` deveria aninhar a filha, got %+v", children)
	}
}

func TestGetHierarchy_CaminhoCompletoNoValor(t *testing.T) {
	notas := map[string]string{
		"notes/pai.md":   "---\ntitle: Pai\n---\n",
		"notes/filha.md": "---\npai: notes/pai.md\n---\nConteúdo.\n",
	}
	svc, _ := newMockServiceForParent(t, notas)

	_, children, err := svc.GetHierarchy("notes/pai.md")
	if err != nil {
		t.Fatalf("GetHierarchy: %v", err)
	}
	if len(children) != 1 {
		t.Errorf("caminho completo deveria resolver para o nome-base, got %+v", children)
	}
}

func TestGetChildrenCounts(t *testing.T) {
	notas := map[string]string{
		"notes/projeto.md": "---\ntitle: Projeto\n---\n",
		"notes/a.md":       "---\npai: projeto\n---\n",
		"notes/b.md":       "---\npai: Projeto\n---\n",    // caixa diferente → mesma chave
		"notes/c.md":       "---\nparent: projeto\n---\n", // chave legada
		"notes/d.md":       "---\npai: outro\n---\n",      // pai inexistente: conta mesmo assim
		"notes/e.md":       "---\ntitle: sem pai\n---\n",
	}
	svc, _ := newMockServiceForParent(t, notas)

	counts, err := svc.GetChildrenCounts()
	if err != nil {
		t.Fatalf("GetChildrenCounts: %v", err)
	}
	if counts["projeto"] != 3 {
		t.Errorf("esperado 3 filhas para \"projeto\", got %d (%+v)", counts["projeto"], counts)
	}
	if counts["outro"] != 1 {
		t.Errorf("referência a pai inexistente ainda é contada, got %d", counts["outro"])
	}
	if len(counts) != 2 {
		t.Errorf("esperado apenas 2 chaves com filhos, got %+v", counts)
	}
	if _, ok := counts["sem-pai"]; ok {
		t.Error("nota sem `pai` não deve aparecer no mapa")
	}
}

// TestGetChildrenCounts_CacheEInvalidação cobre o contrato do cache: leituras
// seguidas usam o mapa memoizado (a sidebar é paginada) e uma gravação do
// NoteService o descarta.
func TestGetChildrenCounts_CacheEInvalidação(t *testing.T) {
	notas := map[string]string{
		"notes/pai.md": "---\ntitle: Pai\n---\n",
		"notes/a.md":   "---\npai: pai\n---\n",
	}
	svc, saved := newMockServiceForParent(t, notas)

	counts, err := svc.GetChildrenCounts()
	if err != nil {
		t.Fatalf("GetChildrenCounts: %v", err)
	}
	if counts["pai"] != 1 {
		t.Fatalf("esperado 1 filha, got %d", counts["pai"])
	}

	// Uma filha nova aparece no "banco": a leitura seguinte ainda vem do cache.
	(*saved)["notes/b.md"] = "---\npai: pai\n---\n"
	cached, err := svc.GetChildrenCounts()
	if err != nil {
		t.Fatalf("GetChildrenCounts (cache): %v", err)
	}
	if cached["pai"] != 1 {
		t.Errorf("esperado valor cacheado (1), got %d", cached["pai"])
	}

	// Após a invalidação (feita pelas gravações), a contagem é recalculada.
	svc.invalidateChildrenCounts()
	fresh, err := svc.GetChildrenCounts()
	if err != nil {
		t.Fatalf("GetChildrenCounts (após invalidar): %v", err)
	}
	if fresh["pai"] != 2 {
		t.Errorf("esperado 2 filhas após invalidar o cache, got %d", fresh["pai"])
	}
}

func TestNormalizeParentRef_EParentCompareKey(t *testing.T) {
	cases := []struct {
		in       string
		wantNorm string
		wantKey  string
	}{
		{"seila", "seila", "seila"},
		{"  Seila.md  ", "seila.md", "seila"},
		{"notes/seila.md", "notes/seila.md", "seila"},
		{"[[seila]]", "seila", "seila"},
		{"[[seila|alias]]", "seila", "seila"},
		{"[[seila#secao]]", "seila", "seila"},
		{`"[[seila]]"`, "seila", "seila"},
		{"[seila]", "seila", "seila"},
		{"./seila", "seila", "seila"},
	}
	for _, tc := range cases {
		if got := NormalizeParentRef(tc.in); got != tc.wantNorm {
			t.Errorf("NormalizeParentRef(%q) = %q, want %q", tc.in, got, tc.wantNorm)
		}
		if got := ParentCompareKey(tc.in); got != tc.wantKey {
			t.Errorf("ParentCompareKey(%q) = %q, want %q", tc.in, got, tc.wantKey)
		}
	}
}

func TestParentRefOfContent(t *testing.T) {
	if got := ParentRefOfContent("---\npai: projeto\n---\nCorpo\n"); got != "projeto" {
		t.Errorf("esperado \"projeto\", got %q", got)
	}
	if got := ParentRefOfContent("---\nParent: projeto\n---\n"); got != "projeto" {
		t.Errorf("chave legada: esperado \"projeto\", got %q", got)
	}
	// `pai` tem precedência sobre `parent`.
	if got := ParentRefOfContent("---\npai: novo\nparent: velho\n---\n"); got != "novo" {
		t.Errorf("canônica deve vencer a legada, got %q", got)
	}
	if got := ParentRefOfContent("---\ntitle: x\n---\n"); got != "" {
		t.Errorf("sem `pai` deveria devolver vazio, got %q", got)
	}
	if got := ParentRefOfContent("sem frontmatter"); got != "" {
		t.Errorf("sem frontmatter deveria devolver vazio, got %q", got)
	}
}

// ── MergeFrontmatterTags ──

func TestMergeFrontmatterTags(t *testing.T) {
	tagsOf := func(t *testing.T, content string) []string {
		t.Helper()
		fm, _, err := ParseFrontmatter(content)
		if err != nil {
			t.Fatalf("ParseFrontmatter: %v", err)
		}
		if fm == nil {
			return nil
		}
		var out []string
		switch v := fm["tags"].(type) {
		case []interface{}:
			for _, it := range v {
				if s, ok := it.(string); ok {
					out = append(out, s)
				}
			}
		case string:
			out = append(out, strings.Split(v, ",")...)
		}
		return out
	}

	t.Run("nota sem frontmatter ganha a chave tags", func(t *testing.T) {
		got, err := MergeFrontmatterTags("Corpo da nota.\n", []string{"tag1", "tag2"})
		if err != nil {
			t.Fatalf("MergeFrontmatterTags: %v", err)
		}
		tags := tagsOf(t, got)
		if len(tags) != 2 {
			t.Fatalf("esperado 2 tags, got %v", tags)
		}
		if !strings.Contains(got, "Corpo da nota.") {
			t.Errorf("corpo perdido: %q", got)
		}
	})

	t.Run("união sem duplicatas e case-insensitive", func(t *testing.T) {
		got, err := MergeFrontmatterTags("---\ntags: [a, b]\n---\nCorpo\n", []string{"b", "c", "#d"})
		if err != nil {
			t.Fatalf("MergeFrontmatterTags: %v", err)
		}
		tags := tagsOf(t, got)
		want := map[string]bool{"a": true, "b": true, "c": true, "d": true}
		if len(tags) != len(want) {
			t.Fatalf("esperado 4 tags, got %v", tags)
		}
		for _, tag := range tags {
			if !want[tag] {
				t.Errorf("tag inesperada %q em %v", tag, tags)
			}
		}
	})

	t.Run("tags em bloco YAML viram lista flow", func(t *testing.T) {
		got, err := MergeFrontmatterTags("---\ntags:\n  - x\n  - y\n---\nCorpo\n", []string{"z"})
		if err != nil {
			t.Fatalf("MergeFrontmatterTags: %v", err)
		}
		if !strings.Contains(got, "tags: [x, y, z]") {
			t.Errorf("esperado lista flow com x, y, z: %q", got)
		}
	})

	t.Run("nada a adicionar devolve conteúdo intacto", func(t *testing.T) {
		original := "---\ntags: [a]\n---\nCorpo\n"
		got, err := MergeFrontmatterTags(original, []string{"a", ""})
		if err != nil {
			t.Fatalf("MergeFrontmatterTags: %v", err)
		}
		if got != original {
			t.Errorf("conteúdo alterado sem necessidade:\nantes: %q\ndepois: %q", original, got)
		}
	})
}
