package search

import "sort"

// DefaultRRFK é a constante k do Reciprocal Rank Fusion.
// O valor clássico da literatura (Cormack et al.) é 60.
const DefaultRRFK = 60

// FusionScore retorna a contribuição RRF de um rank (1-based) com a constante k.
// Ranks inválidos (<= 0) não contribuem.
func FusionScore(rank, k int) float64 {
	if rank <= 0 {
		return 0
	}
	if k <= 0 {
		k = DefaultRRFK
	}
	return 1.0 / float64(k+rank)
}

// ReciprocalRankFusion combina duas listas ranqueadas (ex: FTS5 e busca semântica)
// usando Reciprocal Rank Fusion. Entradas são maps de docID → rank (1-based;
// o primeiro resultado tem rank 1, o segundo rank 2, etc.).
//
// A pontuação de cada doc é a soma de 1/(k+rank) em cada lista em que aparece;
// docs que aparecem em AMBAS as listas somam as parcelas e sobem na ordenação —
// é isso que reduz o ruído de um único motor. Retorna os docIDs ordenados por
// score decrescente (desempate: docs em ambos os motores primeiro), limitado a
// `limit` itens.
func ReciprocalRankFusion(ftsRanks, semRanks map[string]int, k, limit int) []string {
	if k <= 0 {
		k = DefaultRRFK
	}

	scores := make(map[string]float64, len(ftsRanks)+len(semRanks))
	inBoth := make(map[string]bool, len(ftsRanks))
	for doc, rank := range ftsRanks {
		scores[doc] += FusionScore(rank, k)
	}
	for doc, rank := range semRanks {
		scores[doc] += FusionScore(rank, k)
		if _, ok := ftsRanks[doc]; ok {
			inBoth[doc] = true
		}
	}

	type scored struct {
		doc   string
		score float64
	}
	all := make([]scored, 0, len(scores))
	for doc, score := range scores {
		all = append(all, scored{doc, score})
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].score != all[j].score {
			return all[i].score > all[j].score
		}
		bi, bj := inBoth[all[i].doc], inBoth[all[j].doc]
		if bi != bj {
			return bi
		}
		return all[i].doc < all[j].doc
	})

	if limit <= 0 || limit > len(all) {
		limit = len(all)
	}
	out := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, all[i].doc)
	}
	return out
}
