package notes

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"ton618/core/internal/core/domain"
)

// childrenCacheTTL é o tempo máximo que o mapa de contagens de filhas fica em
// cache. A invalidação explícita acontece em toda gravação feita pelo
// NoteService (processAndSave/Delete); o TTL cobre escritas fora dele (ex:
// watcher), para a sidebar nunca ficar desatualizada além disso.
const childrenCacheTTL = 5 * time.Second

// ── Hierarquia de notas (propriedade "pai" do frontmatter) ──
//
// Este arquivo concentra a LEITURA da hierarquia. A escrita/validação vive em
// system/note_tree.go (Tabulator) e em NoteService.UpdateParentOnRename (rename).
// A normalização de referências (NormalizeParentRef/ParentCompareKey) é única e
// usada pelos dois lados — ver DECISIONS §6.17/§6.19.

// NormalizeParentRef normaliza a referência do frontmatter "pai": remove
// espaços e aspas, desembrulha wikilinks ("[[x]]", "[[x|alias]]", "[[x#secao]]")
// e baixa a caixa. O caminho é preservado, quando informado.
//
// Movida de system/note_tree.go: os dois pacotes precisam exatamente da mesma
// regra (o frontmatter é lido e escrito por ambos).
func NormalizeParentRef(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.Trim(s, "\"'")
	if strings.HasPrefix(s, "[[") {
		s = strings.TrimPrefix(s, "[[")
		s = strings.TrimSuffix(s, "]]")
		if i := strings.IndexAny(s, "|#"); i >= 0 {
			s = s[:i]
		}
	}
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "./")
	s = strings.Trim(s, "/")
	// Colchetes remanescentes: em YAML sem aspas, `parent: [[nota]]` vira uma
	// lista aninhada e `parent: [nota]` uma lista de um item. O valor chega aqui
	// já convertido para texto (fmt.Sprintf), então os colchetes precisam sair.
	s = strings.Trim(s, "[]")
	return strings.ToLower(s)
}

// ParentCompareKey devolve a chave de comparação canônica de uma referência de
// pai: o nome-base em minúsculas, sem pasta e sem extensão. É a forma GRAVADA em
// `pai:` e, portanto, a chave usada para casar filha↔pai.
func ParentCompareKey(raw string) string {
	s := NormalizeParentRef(raw)
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	return strings.TrimSuffix(s, ".md")
}

// noteShortName devolve o nome-base em minúsculas de um arquivo (ex:
// "notes/Projeto.md" → "projeto").
func noteShortName(file string) string {
	base := file
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	return strings.ToLower(strings.TrimSuffix(base, ".md"))
}

// ParentRefOfContent extrai o valor bruto de `pai:`/`parent:` do frontmatter de
// uma nota (a chave canônica tem precedência; a legada é aceita). Devolve "" se
// não houver a propriedade.
func ParentRefOfContent(content string) string {
	fm, _, err := ParseFrontmatter(content)
	if err != nil || fm == nil {
		return ""
	}
	for _, key := range []string{"pai", "parent"} {
		for k, v := range fm {
			if !strings.EqualFold(k, key) || v == nil {
				continue
			}
			if str, ok := v.(string); ok {
				return str
			}
			return fmt.Sprintf("%v", v)
		}
	}
	return ""
}

// GetHierarchy devolve o pai declarado e as filhas de filename. Lê o conteúdo de
// todas as notas em UMA query (GetAllNotesContent) — usado ao abrir o editor
// para renderizar a barra de hierarquia.
//
// A resolução é por nome-base (o formato canônico gravado em `pai:`); nomes
// duplicados entre pastas são raros e, na árvore do Tabulator, o desempate
// prefere notes/ — aqui a lista de filhas é apenas navegação.
func (s *NoteService) GetHierarchy(filename string) (parent string, children []domain.NoteRef, err error) {
	contents, err := s.notes.GetAllNotesContent()
	if err != nil {
		return "", nil, fmt.Errorf("GetHierarchy: %w", err)
	}

	self := noteShortName(filename)
	parent = ParentCompareKey(ParentRefOfContent(contents[filename]))

	for file, content := range contents {
		if file == filename || content == "" {
			continue
		}
		if ParentCompareKey(ParentRefOfContent(content)) == self {
			children = append(children, domain.NoteRef{
				Filename:    file,
				DisplayName: domain.DisplayName(file),
			})
		}
	}

	sort.Slice(children, func(i, j int) bool {
		return children[i].DisplayName < children[j].DisplayName
	})
	return parent, children, nil
}

// GetChildrenCounts devolve, por nome-base (minúsculo), quantas notas o declaram
// como pai. Usado pela sidebar para exibir o badge de filhas.
//
// CUSTO: uma leitura de conteúdo de todas as notas + um parse de frontmatter por
// nota. Como a sidebar é paginada com scroll infinito (uma requisição por
// página), o resultado é memoizado por childrenCacheTTL e invalidado a cada
// gravação do NoteService — ver invalidateChildrenCounts.
//
// O mapa devolvido NÃO deve ser mutado pelo chamador (pode ser o do cache).
func (s *NoteService) GetChildrenCounts() (map[string]int, error) {
	s.childrenMu.Lock()
	if s.childrenMap != nil && time.Since(s.childrenAt) < childrenCacheTTL {
		cached := s.childrenMap
		s.childrenMu.Unlock()
		return cached, nil
	}
	s.childrenMu.Unlock()

	contents, err := s.notes.GetAllNotesContent()
	if err != nil {
		return nil, fmt.Errorf("GetChildrenCounts: %w", err)
	}

	counts := make(map[string]int)
	for _, content := range contents {
		ref := ParentCompareKey(ParentRefOfContent(content))
		if ref != "" {
			counts[ref]++
		}
	}

	s.childrenMu.Lock()
	s.childrenMap = counts
	s.childrenAt = time.Now()
	s.childrenMu.Unlock()

	return counts, nil
}

// invalidateChildrenCounts descarta o cache de contagens de filhas. Chamado após
// qualquer gravação de nota (o `pai:` de uma nota pode ter mudado).
func (s *NoteService) invalidateChildrenCounts() {
	s.childrenMu.Lock()
	s.childrenMap = nil
	s.childrenMu.Unlock()
}
