package domain

import "runtime"

// NormalizeWorkers resolve o número de workers de um pool CPU-bound:
// workers <= 0 → automático (GOMAXPROCS, no máximo 4 — adequado a ARMv7 de 4
// núcleos sem estourar memória); nunca menos que 1 nem mais que n.
//
// É a regra única usada pela indexação (watcher) e pelo backup.
func NormalizeWorkers(workers, n int) int {
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
		if workers > 4 {
			workers = 4
		}
	}
	if workers > n {
		workers = n
	}
	if workers < 1 {
		workers = 1
	}
	return workers
}
