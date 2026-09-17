package system

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"ton618/core/internal/core/domain"
	"ton618/core/internal/features/notes"
)

// ── Hierarquia de notas (propriedade "pai" do frontmatter) ──
//
// A nota declara a sua "nota-mãe" no frontmatter:
//
//	---
//	pai: projeto-ton618
//	---
//
// A chave canônica é `pai`. O nome antigo `parent` continua sendo LIDO (notas
// gravadas antes da renomeação não perdem a hierarquia), mas toda gravação usa
// `pai` e a chave legada é removida do frontmatter — ver normalizeParentKey e
// o branch de hierarquia em HandleUpdateNoteProperty (DECISIONS 6.19).
//
// O valor canônico é o NOME da nota (sem pasta e sem ".md"). São aceitos, por
// conveniência: o caminho completo ("notes/projeto.md", usado para desambiguar
// notas de mesmo nome), a forma de wikilink ("[[projeto]]", "[[projeto|alias]]",
// "[[projeto#secao]]") e qualquer variação de caixa.
//
// Regras (DECISIONS 6.17):
//   - no máximo 1 pai por nota (é uma árvore, não um grafo);
//   - profundidade ilimitada;
//   - pai inexistente → a nota fica na raiz (sem erro);
//   - ciclo (nota dentro de si mesma, direta ou indiretamente) é bloqueado.
//
// O Tabulator exibe a hierarquia com o módulo Data Tree, que espera os filhos
// no campo "_children" de cada linha — ver buildNoteTree.
const (
	// parentKey é a chave canônica do frontmatter que declara a nota-mãe.
	// Também é o campo/coluna que o Tabulator exibe e edita (rótulo "Pai").
	parentKey = "pai"

	// parentKeyLegacy é o nome antigo da chave. Aceito apenas na LEITURA: existe
	// para que notas gravadas antes da renomeação continuem aninhadas. Toda
	// gravação usa parentKey.
	parentKeyLegacy = "parent"

	// childrenKey é o campo que o módulo Data Tree do Tabulator lê para montar
	// os níveis. O prefixo "_" garante que ele nunca vire uma coluna visível
	// (HandleGetDatabaseData ignora campos internos).
	childrenKey = "_children"

	// parentValidationMaxDepth limita a caminhada de validação, que lê o
	// conteúdo das notas no banco. Não restringe a profundidade da árvore
	// exibida: existe só para impedir I/O patológico numa cadeia corrompida, e
	// erra para o lado seguro (bloqueia a gravação).
	parentValidationMaxDepth = 100
)

// noteEntry é o mínimo necessário para indexar as notas por nome e por pasta.
type noteEntry struct {
	File  string
	Mtime string
}

// noteTreeIndex resolve a referência "parent" para o arquivo canônico da nota.
type noteTreeIndex struct {
	files  map[string]string // lower(arquivo) → arquivo canônico (caixa original)
	byBase map[string]string // lower(nome sem pasta/extensão) → arquivo canônico
}

// newNoteTreeIndex indexa as notas para resolução por nome (caso típico) e por
// caminho (desambiguação).
//
// A escolha do dono de cada nome-base é 100% determinística: a varredura é feita
// na ordem da slice de entrada e a preferência (notes/ primeiro, depois mtime
// mais recente) é decidida por regra explícita — nunca pela ordem de iteração
// de um map.
func newNoteTreeIndex(entries []noteEntry) *noteTreeIndex {
	ix := &noteTreeIndex{
		files:  make(map[string]string, len(entries)),
		byBase: make(map[string]string, len(entries)),
	}

	for _, e := range entries {
		if e.File != "" {
			ix.files[strings.ToLower(e.File)] = e.File
		}
	}

	winners := make(map[string]noteEntry, len(entries))
	for _, e := range entries {
		if e.File == "" {
			continue
		}
		base := baseNameKey(e.File)
		cur, ok := winners[base]
		if !ok || preferAsParent(e, cur) {
			winners[base] = e
		}
	}
	for base, e := range winners {
		ix.byBase[base] = e.File
	}

	return ix
}

// resolve devolve o arquivo canônico da nota referenciada. Retorna "" quando a
// nota não existe (a nota-filha fica na raiz — ver regras no cabeçalho).
func (ix *noteTreeIndex) resolve(raw string) string {
	ref := NormalizeParentRef(raw)
	if ref == "" {
		return ""
	}

	// Caminho completo tem prioridade: é a forma de desambiguar.
	if strings.Contains(ref, "/") {
		cand := ref
		if !strings.HasSuffix(cand, ".md") {
			cand += ".md"
		}
		if f, ok := ix.files[cand]; ok {
			return f
		}
	}

	if f, ok := ix.byBase[baseNameKey(ref)]; ok {
		return f
	}
	return ""
}

// NormalizeParentRef normaliza a referência do frontmatter "pai": remove
// espaços e aspas, desembrulha wikilinks ("[[x]]", "[[x|alias]]", "[[x#secao]]")
// e baixa a caixa. O caminho é preservado, quando informado.
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

// parentRefFromMap extrai o valor da chave de hierarquia de um mapa de
// frontmatter (ou de uma linha do Tabulator). A chave canônica (`pai`) tem
// precedência sobre a antiga (`parent`); a comparação é case-insensitive para
// tolerar "Pai:"/"Parent:".
func parentRefFromMap(m map[string]interface{}) string {
	for _, key := range []string{parentKey, parentKeyLegacy} {
		for k, v := range m {
			if !strings.EqualFold(k, key) || v == nil {
				continue
			}
			if s, ok := v.(string); ok {
				return NormalizeParentRef(s)
			}
			return NormalizeParentRef(fmt.Sprintf("%v", v))
		}
	}
	return ""
}

// isParentKey informa se a chave recebida é a canônica ou a antiga. Usado na
// gravação, que aceita as duas e normaliza para parentKey.
func isParentKey(key string) bool {
	return strings.EqualFold(key, parentKey) || strings.EqualFold(key, parentKeyLegacy)
}

// normalizeParentKey garante UMA única coluna de hierarquia no Tabulator: move
// o valor de qualquer variação da chave (caixa diferente, nome antigo) para
// `pai` e remove a chave legada. Sem isso uma nota que ainda usa `parent`
// apareceria com duas colunas ("Pai" vazia e "Parent" preenchida).
//
// É idempotente e só troca chaves do próprio map recebido — o chamador passa a
// linha já copiada do cache (ver o comentário sobre referência de map em
// buildNoteTree).
func normalizeParentKey(row map[string]interface{}) {
	var canonicalKey, legacyKey string
	var canonicalVal interface{}
	for k, v := range row {
		switch {
		case canonicalKey == "" && strings.EqualFold(k, parentKey):
			canonicalKey, canonicalVal = k, v
		case legacyKey == "" && strings.EqualFold(k, parentKeyLegacy):
			legacyKey = k
		}
	}

	if canonicalKey != "" && canonicalKey != parentKey {
		delete(row, canonicalKey)
		row[parentKey] = canonicalVal
	}
	if legacyKey == "" {
		return
	}

	legacyVal := row[legacyKey]
	delete(row, legacyKey)
	if _, ok := row[parentKey]; !ok {
		row[parentKey] = legacyVal
	}
}

// baseNameKey devolve o nome do arquivo sem pasta e sem extensão, em minúsculas.
func baseNameKey(file string) string {
	base := file
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	return strings.ToLower(strings.TrimSuffix(base, filepath.Ext(base)))
}

// preferAsParent decide qual candidata vence quando duas notas compartilham o
// mesmo nome-base: a da pasta notes/ (notas normais) e, em empate, a modificada
// mais recentemente. Empate total mantém a candidata atual (determinístico).
func preferAsParent(candidate, current noteEntry) bool {
	cNotes := strings.HasPrefix(strings.ToLower(candidate.File), "notes/")
	curNotes := strings.HasPrefix(strings.ToLower(current.File), "notes/")
	if cNotes != curNotes {
		return cNotes
	}
	return mtimeIsNewer(candidate.Mtime, current.Mtime)
}

// mtimeIsNewer aceita tanto RFC3339 (banco) quanto o formato de exibição do
// Tabulator ("2006-01-02 15:04:05"), que também é ordenável como string.
func mtimeIsNewer(a, b string) bool {
	if a == "" {
		return false
	}
	if b == "" {
		return true
	}
	if ta, errA := time.Parse(time.RFC3339, a); errA == nil {
		if tb, errB := time.Parse(time.RFC3339, b); errB == nil {
			return ta.After(tb)
		}
	}
	return a > b
}

// hasParentCycle informa se a cadeia de pais de start volta a si mesma.
// Cada arquivo visitado é marcado, então a caminhada sempre termina — mesmo com
// dados corrompidos (não há limite arbitrário de profundidade aqui).
func hasParentCycle(parentOf map[string]string, start string) bool {
	seen := map[string]bool{start: true}
	for cur := parentOf[start]; cur != ""; cur = parentOf[cur] {
		if seen[cur] {
			return true
		}
		seen[cur] = true
	}
	return false
}

// rowFile devolve o arquivo de uma linha do Tabulator.
func rowFile(row map[string]interface{}) string {
	f, _ := row["arquivo"].(string)
	return f
}

// rowMtime devolve o mtime exibido de uma linha do Tabulator.
func rowMtime(row map[string]interface{}) string {
	m, _ := row["mtime"].(string)
	return m
}

// buildNoteTree reorganiza as linhas achatadas em uma floresta, injetando o
// campo "_children" nos nós que têm filhos. Devolve as raízes e se existe
// alguma hierarquia (o frontend só liga o módulo Data Tree quando hasTree é
// true).
//
// ⚠️ As linhas do cache (ctx.dbCache) são guardadas POR REFERÊNCIA de map. Por
// isso cada nó é copiado (cópia rasa) antes de receber "_children": sem isso o
// cache acumularia filhos a cada request e a tabela mostraria linhas
// duplicadas.
func buildNoteTree(rows []map[string]interface{}) ([]map[string]interface{}, bool) {
	if len(rows) == 0 {
		return rows, false
	}

	entries := make([]noteEntry, 0, len(rows))
	for _, r := range rows {
		if file := rowFile(r); file != "" {
			entries = append(entries, noteEntry{File: file, Mtime: rowMtime(r)})
		}
	}
	ix := newNoteTreeIndex(entries)

	// parentOf: apenas vínculos válidos (pai existente, não é a própria nota e
	// não fecha ciclo). A varredura é feita na ordem de `rows` (determinística),
	// então em caso de ciclo quem fecha o laço perde o vínculo e volta à raiz —
	// sempre o mesmo, independente da ordem de iteração de qualquer map.
	parentOf := make(map[string]string, len(rows))
	for _, r := range rows {
		key := strings.ToLower(rowFile(r))
		if key == "" {
			continue
		}
		ref := parentRefFromMap(r)
		if ref == "" {
			continue
		}
		target := ix.resolve(ref)
		if target == "" {
			continue // pai inexistente → fica na raiz
		}
		parent := strings.ToLower(target)
		if parent == key {
			continue // auto-pai → fica na raiz
		}
		parentOf[key] = parent
		if hasParentCycle(parentOf, key) {
			delete(parentOf, key)
		}
	}

	childRows := make(map[string][]map[string]interface{}, 8)
	for _, r := range rows {
		key := strings.ToLower(rowFile(r))
		if key == "" {
			continue
		}
		if parent, ok := parentOf[key]; ok {
			childRows[parent] = append(childRows[parent], r)
		}
	}

	// Sem nenhum vínculo válido: devolve as linhas originais (zero custo).
	if len(childRows) == 0 {
		return rows, false
	}

	// Cada linha aparece no máximo uma vez (parentOf é uma função e a floresta é
	// acíclica), então a recursão percorre cada nó uma única vez e termina.
	var attach func(r map[string]interface{}) map[string]interface{}
	attach = func(r map[string]interface{}) map[string]interface{} {
		node := make(map[string]interface{}, len(r)+1)
		for k, v := range r {
			node[k] = v
		}
		if kids := childRows[strings.ToLower(rowFile(r))]; len(kids) > 0 {
			list := make([]map[string]interface{}, 0, len(kids))
			for _, kid := range kids {
				list = append(list, attach(kid))
			}
			node[childrenKey] = list
		}
		return node
	}

	roots := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		if _, isChild := parentOf[strings.ToLower(rowFile(r))]; isChild {
			continue
		}
		roots = append(roots, attach(r))
	}
	return roots, true
}

// validateParentAssignment garante a integridade do vínculo antes de gravar a
// propriedade de hierarquia: bloqueia auto-pai e ciclo. Referência a nota
// inexistente é PERMITIDA de propósito (a nota fica na raiz até a outra
// existir) — ver regras no cabeçalho do arquivo.
func (ctx *HandlerContext) validateParentAssignment(file, ref string) error {
	if ref == "" {
		return nil // remoção da propriedade
	}

	items, err := ctx.Notes.GetMany()
	if err != nil {
		return nil // sem índice, este guard não é o responsável por bloquear
	}
	entries := make([]noteEntry, 0, len(items))
	for _, it := range items {
		entries = append(entries, noteEntry{File: it.Arquivo, Mtime: it.Mtime})
	}
	ix := newNoteTreeIndex(entries)

	self := strings.ToLower(file)
	target := ix.resolve(ref)
	if target == "" {
		return nil // pai inexistente: permitido (a nota fica na raiz)
	}
	if strings.ToLower(target) == self {
		return fmt.Errorf("uma nota não pode ser pai dela mesma")
	}

	// Sobe a cadeia a partir do pai proposto: se alcançar esta nota, o vínculo
	// criaria um ciclo. O memo evita reler a mesma nota.
	memo := make(map[string]string, 8)
	cur := strings.ToLower(target)
	for depth := 0; depth < parentValidationMaxDepth && cur != ""; depth++ {
		if cur == self {
			return fmt.Errorf("vínculo recusado: %q já está abaixo desta nota (criaria um ciclo)", domain.DisplayName(target))
		}
		next, ok := memo[cur]
		if !ok {
			next = ctx.parentRefOf(ix, cur)
			memo[cur] = next
		}
		cur = next
	}
	return nil
}

// parentRefOf descobre o pai declarado por uma nota lendo o frontmatter dela no
// banco. Usado apenas na validação de gravação — o Tabulator resolve tudo o que
// precisa em memória, sem custo de I/O.
func (ctx *HandlerContext) parentRefOf(ix *noteTreeIndex, lowerFile string) string {
	canonical := ix.files[lowerFile]
	if canonical == "" {
		return ""
	}
	content, err := ctx.Store.GetNote(canonical)
	if err != nil || content == "" {
		return ""
	}
	fm, _, err := notes.ParseFrontmatter(content)
	if err != nil || fm == nil {
		return ""
	}
	return strings.ToLower(ix.resolve(parentRefFromMap(fm)))
}
