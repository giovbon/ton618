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

// clampUnit restringe v ao intervalo [0,1].
func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// WeightedFusionScore retorna a contribuição RRF ponderada pela qualidade do
// motor (score em [0,1]). O peso varia de 0.5 (score 0/indisponível → neutro,
// equivalente ao RRF puro) até 1.0 (score 1 → contribuição cheia). Como o score
// é monotônico com o rank (rank 1 tem o melhor score), a ordem dentro de um
// mesmo motor é preservada — a ponderação apenas aproxima/distanzia candidatos
// de motores diferentes conforme a força da evidência de cada um.
func WeightedFusionScore(rank, k int, score float64) float64 {
	base := FusionScore(rank, k)
	if base <= 0 {
		return 0
	}
	w := 0.5 + 0.5*clampUnit(score)
	return base * w
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

// ReciprocalRankFusionWeighted é a variante ciente de qualidade do RRF. Cada
// parcela 1/(k+rank) é multiplicada por um peso derivado do score normalizado
// do motor (ftsScore/semScore em [0,1] — ex.: FinalScore FTS normalizado e
// similaridade semântica). Score ausente/indisponível (0) mantém peso neutro.
// Isso faz com que um candidato presente nos dois motores (ou com evidência
// muito forte em um deles) se destaque mais do que no RRF puro, que trata
// rank 1 e rank 30 com quase o mesmo peso.
//
// Desempate determinístico: score ponderado desc → doc nos dois motores →
// qualidade total desc → menor rank no FTS → menor rank na semântica → nome.
func ReciprocalRankFusionWeighted(ftsRanks, semRanks map[string]int, ftsScore, semScore map[string]float64, k, limit int) []string {
	if k <= 0 {
		k = DefaultRRFK
	}

	type agg struct {
		score   float64
		quality float64
		inBoth  bool
		ftsRank int
		semRank int
	}
	index := make(map[string]*agg, len(ftsRanks)+len(semRanks))
	for doc, rank := range ftsRanks {
		a := index[doc]
		if a == nil {
			a = &agg{}
			index[doc] = a
		}
		a.score += WeightedFusionScore(rank, k, ftsScore[doc])
		a.quality += clampUnit(ftsScore[doc])
		a.ftsRank = rank
	}
	for doc, rank := range semRanks {
		a := index[doc]
		if a == nil {
			a = &agg{}
			index[doc] = a
		}
		a.score += WeightedFusionScore(rank, k, semScore[doc])
		a.quality += clampUnit(semScore[doc])
		a.semRank = rank
		if _, ok := ftsRanks[doc]; ok {
			a.inBoth = true
		}
	}

	docs := make([]string, 0, len(index))
	for doc := range index {
		docs = append(docs, doc)
	}
	sort.Slice(docs, func(i, j int) bool {
		ai, aj := index[docs[i]], index[docs[j]]
		if ai.score != aj.score {
			return ai.score > aj.score
		}
		if ai.inBoth != aj.inBoth {
			return ai.inBoth
		}
		if ai.quality != aj.quality {
			return ai.quality > aj.quality
		}
		if ai.ftsRank != aj.ftsRank {
			if ai.ftsRank == 0 {
				return false
			}
			if aj.ftsRank == 0 {
				return true
			}
			return ai.ftsRank < aj.ftsRank
		}
		if ai.semRank != aj.semRank {
			if ai.semRank == 0 {
				return false
			}
			if aj.semRank == 0 {
				return true
			}
			return ai.semRank < aj.semRank
		}
		return docs[i] < docs[j]
	})

	if limit <= 0 || limit > len(docs) {
		limit = len(docs)
	}
	return docs[:limit]
}
