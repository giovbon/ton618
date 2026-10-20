package notes

import (
	"testing"

	"ton618/core/internal/core/domain"
)

// ── Hierarquia com ARQUIVOS como filhas (DECISIONS §6.24) ──
//
// Arquivos (PDF/EPUB/ZIP/...) não têm frontmatter: o `pai` vive na tabela
// file_metadata. Eles podem ser FILHAS, nunca PAI.

// newMockServiceWithFiles cria um NoteService em memória com notas (frontmatter)
// E metadados de arquivo, permitindo observar as gravações no metadata.
func newMockServiceWithFiles(t *testing.T, notas map[string]string, fileMeta map[string]map[string]string) (*NoteService, *map[string]string, *map[string]map[string]string) {
	t.Helper()
	meta := fileMeta
	if meta == nil {
		meta = make(map[string]map[string]string)
	}

	svc, saved := newMockServiceForParent(t, notas)
	svc.store = &mockFileOps{
		getAllFileMetadataFn: func() (map[string]map[string]string, error) {
			return meta, nil
		},
		setFileMetadataFn: func(arquivo, key, value string) error {
			if value == "" {
				if m, ok := meta[arquivo]; ok {
					delete(m, key)
					if len(m) == 0 {
						delete(meta, arquivo)
					}
				}
				return nil
			}
			if meta[arquivo] == nil {
				meta[arquivo] = make(map[string]string)
			}
			meta[arquivo][key] = value
			return nil
		},
	}
	return svc, saved, &meta
}

func TestGetChildrenCounts_IncluiFilhasArquivo(t *testing.T) {
	notas := map[string]string{
		"notes/projeto.md": "# Projeto\n",
	}
	meta := map[string]map[string]string{
		"pdfs/manual.pdf":     {domain.ParentKey: "projeto"},
		"attachments/x.zip":   {domain.ParentKey: "Projeto"}, // caixa diferente → mesma chave
		"epubs/livro.epub":    {domain.ParentKey: "outro"},   // pai inexistente: conta mesmo assim
		"attachments/nao.tgz": {},                            // sem pai: não conta
	}
	svc, _, _ := newMockServiceWithFiles(t, notas, meta)

	counts, err := svc.GetChildrenCounts()
	if err != nil {
		t.Fatalf("GetChildrenCounts: %v", err)
	}
	if counts["projeto"] != 2 {
		t.Errorf("esperado 2 filhas de \"projeto\" (2 arquivos), got %d (%+v)", counts["projeto"], counts)
	}
	if counts["outro"] != 1 {
		t.Errorf("referência a pai inexistente ainda é contada, got %d", counts["outro"])
	}
}

func TestGetHierarchy_IncluiFilhasArquivo(t *testing.T) {
	notas := map[string]string{
		"notes/projeto.md": "# Projeto\n",
		"notes/filha.md":   "---\npai: projeto\n---\nConteúdo.\n",
	}
	meta := map[string]map[string]string{
		"pdfs/manual.pdf": {domain.ParentKey: "projeto"},
	}
	svc, _, _ := newMockServiceWithFiles(t, notas, meta)

	parent, children, err := svc.GetHierarchy("notes/projeto.md")
	if err != nil {
		t.Fatalf("GetHierarchy: %v", err)
	}
	if parent != "" {
		t.Errorf("nota raiz deveria ter pai vazio, got %q", parent)
	}
	if len(children) != 2 {
		t.Fatalf("esperava 2 filhas (nota + arquivo), got %d: %+v", len(children), children)
	}
	// Ordem por nome de exibição: "filha" < "manual.pdf".
	if children[0].Filename != "notes/filha.md" || children[1].Filename != "pdfs/manual.pdf" {
		t.Errorf("filhas/ordem inesperadas: %+v", children)
	}
	if children[1].DisplayName != "manual.pdf" {
		t.Errorf("DisplayName do arquivo deveria manter a extensão, got %q", children[1].DisplayName)
	}
}

func TestUpdateParentOnRename_PropagaParaArquivo(t *testing.T) {
	notas := map[string]string{
		"notes/projeto.md": "# Projeto\n",
	}
	meta := map[string]map[string]string{
		"pdfs/manual.pdf":   {domain.ParentKey: "projeto"},
		"attachments/x.zip": {domain.ParentKey: "outro"}, // não deve mudar
	}
	svc, _, saved := newMockServiceWithFiles(t, notas, meta)

	if err := svc.UpdateParentOnRename("notes/projeto.md", "notes/projeto-renomeado.md"); err != nil {
		t.Fatalf("UpdateParentOnRename: %v", err)
	}

	if (*saved)["pdfs/manual.pdf"][domain.ParentKey] != "projeto-renomeado" {
		t.Errorf("o `pai` do arquivo deveria ter sido repontado, got %v", (*saved)["pdfs/manual.pdf"])
	}
	if (*saved)["attachments/x.zip"][domain.ParentKey] != "outro" {
		t.Errorf("arquivo com outro pai não deveria ser tocado, got %v", (*saved)["attachments/x.zip"])
	}
}

// SetFileParent grava e remove (`ref` vazio) o vínculo do arquivo, e o faz
// passando pela chave canônica do domain.
func TestSetFileParent_GravaERemove(t *testing.T) {
	svc, _, saved := newMockServiceWithFiles(t, nil, nil)

	if err := svc.SetFileParent("pdfs/x.pdf", "projeto"); err != nil {
		t.Fatalf("SetFileParent: %v", err)
	}
	if (*saved)["pdfs/x.pdf"][domain.ParentKey] != "projeto" {
		t.Fatalf("gravação inesperada: %v", (*saved)["pdfs/x.pdf"])
	}

	if err := svc.SetFileParent("pdfs/x.pdf", ""); err != nil {
		t.Fatalf("SetFileParent (remoção): %v", err)
	}
	if _, ok := (*saved)["pdfs/x.pdf"]; ok {
		t.Errorf("a remoção deveria apagar a entrada do arquivo, got %v", (*saved)["pdfs/x.pdf"])
	}
}
