package notes

import (
	"sort"
	"strings"
	"sync"

	"ton618/core/internal/core/db"
	"ton618/core/internal/core/domain"
	"ton618/core/internal/processor"
	"ton618/core/internal/query"
)

// ── Ponte entre a feature de notas e o motor de consultas ────────────────
//
// queryReader implementa query.Reader usando SOMENTE métodos de leitura do
// Store. É o único ponto de contato entre a linguagem de consulta e o acervo
// (DECISIONS §6.29): o motor não conhece *db.Store, então não existe (nem pode
// existir) caminho de escrita a partir dele.
// ⚠️ O leitor é um SNAPSHOT por requisição. Uma nota pode ter até 10 blocos
// `consulta` e todos leem exatamente os mesmos dados — antes, cada bloco refazia
// todas as leituras (medido: 22 leituras para 3 blocos, ~7 por bloco) e cada
// Contents() carrega o conteúdo de TODAS as notas. Com o memo abaixo o painel faz
// uma leitura por tabela, não importa quantos blocos existam — e, de quebra,
// todos os cartões da página mostram a MESMA fotografia do acervo.
//
// Os métodos são de ponteiro por causa dos sync.Once (copiar o struct copiaria
// os locks).
type queryReader struct {
	store *db.Store

	filesOnce sync.Once
	files     []db.FileModTag
	filesErr  error

	contentsOnce sync.Once
	contents     map[string]string
	contentsErr  error

	linksOnce sync.Once
	links     map[string][]string
	linksErr  error

	popOnce sync.Once
	pop     map[string]int
	popErr  error

	lastOnce sync.Once
	last     map[string]string
	lastErr  error

	hierOnce sync.Once
	hier     query.Hierarchy
	hierErr  error
}

func newQueryReader(store *db.Store) *queryReader { return &queryReader{store: store} }

// Files devolve (uma vez por requisição) arquivo + mtime + tags de todo o acervo.
func (q *queryReader) Files() ([]db.FileModTag, error) {
	q.filesOnce.Do(func() {
		q.files, q.filesErr = q.store.GetFilesModsAndTags()
	})
	return q.files, q.filesErr
}

// Contents devolve (uma vez) o conteúdo indexado das notas — a leitura mais cara
// do conjunto, usada só para classe, título e pai.
func (q *queryReader) Contents() (map[string]string, error) {
	q.contentsOnce.Do(func() {
		q.contents, q.contentsErr = q.store.GetAllNotesContent()
		if q.contentsErr != nil {
			q.contents = map[string]string{}
		}
	})
	return q.contents, nil
}

func (q *queryReader) Links() (map[string][]string, error) {
	q.linksOnce.Do(func() {
		q.links, q.linksErr = q.store.GetAllLinks()
		if q.linksErr != nil {
			q.links = map[string][]string{}
		}
	})
	return q.links, nil
}

func (q *queryReader) Popularity() (map[string]int, error) {
	q.popOnce.Do(func() {
		q.pop, q.popErr = q.store.GetAllPopularity()
		if q.popErr != nil {
			q.pop = map[string]int{}
		}
	})
	return q.pop, nil
}

func (q *queryReader) LastInteracted() (map[string]string, error) {
	q.lastOnce.Do(func() {
		q.last, q.lastErr = q.store.GetAllLastInteracted()
		if q.lastErr != nil {
			q.last = map[string]string{}
		}
	})
	return q.last, nil
}

// Todos NÃO é memoizado: o filtro (status) muda a consulta, e é uma query barata
// indexada por `todos.file`.
func (q *queryReader) Todos(filtro map[string]bool, status string) ([]processor.TodoItem, error) {
	return q.store.GetTodosFiltered(filtro, status)
}

// SearchText também não é memoizado — cada bloco pode ter um termo diferente.
func (q *queryReader) SearchText(termo string, limite int) ([]db.FTSResult, error) {
	resultados, _, err := q.store.SearchFTS(termo, 0, limite)
	if err != nil {
		return nil, err
	}
	return resultados, nil
}

// Hierarchy monta a fotografia da árvore `pai:` com as MESMAS regras de
// resolução do resto do app (NormalizeParentRef/ParentCompareKey — §6.17/§6.19):
//
//   - notas (Markdown) declaram o pai no frontmatter (`pai`, com `parent` como
//     legado);
//   - arquivos sem frontmatter (PDF/EPUB/ZIP) guardam o vínculo em
//     file_metadata (mesma chave).
//
// O valor BRUTO (como escrito) alimenta `n.pai`; a resolução por nome-base
// alimenta `n.pai_resolvido` e `n.filhas`.
func (q *queryReader) Hierarchy() (query.Hierarchy, error) {
	q.hierOnce.Do(func() {
		q.hier, q.hierErr = q.buildHierarchy()
	})
	return q.hier, q.hierErr
}

func (q *queryReader) buildHierarchy() (query.Hierarchy, error) {
	files, err := q.Files()
	if err != nil {
		return query.Hierarchy{}, err
	}
	contents, _ := q.Contents()
	meta, err := q.store.GetAllFileMetadata()
	if err != nil {
		meta = map[string]map[string]string{}
	}

	h := query.Hierarchy{
		Pai:       map[string]string{},
		Resolvido: map[string]string{},
		Filhas:    map[string][]string{},
	}

	// Índice nome-base → arquivo (primeiro vence, como em GetHierarchy).
	porNome := map[string]string{}
	for _, f := range files {
		if _, ok := porNome[noteShortName(f.Arquivo)]; !ok {
			porNome[noteShortName(f.Arquivo)] = f.Arquivo
		}
	}

	for _, f := range files {
		ref := ParentRefOfContent(contents[f.Arquivo])
		if strings.TrimSpace(ref) == "" {
			if m := meta[f.Arquivo]; m != nil {
				ref = m[domain.ParentKey]
				if strings.TrimSpace(ref) == "" {
					ref = m[domain.ParentKeyLegacy]
				}
			}
		}
		if strings.TrimSpace(ref) == "" {
			continue
		}
		h.Pai[f.Arquivo] = ref

		alvo, ok := porNome[ParentCompareKey(ref)]
		if !ok || alvo == f.Arquivo {
			continue
		}
		h.Resolvido[f.Arquivo] = alvo
		h.Filhas[alvo] = append(h.Filhas[alvo], f.Arquivo)
	}

	for k := range h.Filhas {
		sort.Strings(h.Filhas[k])
	}

	return h, nil
}
