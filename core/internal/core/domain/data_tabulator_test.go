package domain

import "testing"

func TestDetectNoteType_TabulatorPorTag(t *testing.T) {
	if got := DetectNoteType([]string{"tabulator"}, "notes/x.md"); got != NoteTypeTabulator {
		t.Errorf("tag tabulator deveria detectar NoteTypeTabulator, got %q", got)
	}
}

func TestDetectNoteTypeFromContent_Tabulator(t *testing.T) {
	content := "---\ntype: tabulator\n---\n{\"columns\":[],\"rows\":[]}"
	if got := DetectNoteTypeFromContent(nil, content, "notes/x.md"); got != NoteTypeTabulator {
		t.Errorf("frontmatter type: tabulator deveria detectar NoteTypeTabulator, got %q", got)
	}
}

func TestTabulator_EditorRouteECanonicalTag(t *testing.T) {
	if got := NoteTypeTabulator.EditorRoute(); got != "/tabulator" {
		t.Errorf("EditorRoute deveria ser /tabulator, got %q", got)
	}
	if got := NoteTypeCanonicalTag(NoteTypeTabulator); got != "tabulator" {
		t.Errorf("tag canônica deveria ser \"tabulator\", got %q", got)
	}
}

func TestTabulator_TagInternaEscondida(t *testing.T) {
	userTags := FilterUserTags([]string{"tabulator", "dados", "projeto"})
	for _, tag := range userTags {
		if tag == "tabulator" {
			t.Error("a tag interna \"tabulator\" não deveria aparecer para o usuário")
		}
	}
	if len(userTags) != 2 {
		t.Errorf("esperado 2 tags visíveis, got %v", userTags)
	}
}

func TestTabulator_NoteOpenTarget(t *testing.T) {
	url, blank := NoteOpenTarget(NoteTypeTabulator, "notes/planilha.md")
	if url != "/tabulator?file=notes/planilha.md" {
		t.Errorf("URL inesperada: %q", url)
	}
	if blank {
		t.Error("tabulator não deveria abrir em nova aba")
	}
}
