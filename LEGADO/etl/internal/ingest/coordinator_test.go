package ingest

import (
	"etl/internal/config"
	"sync/atomic"
	"testing"
	"time"
)

func TestCoordinatorDebounceAndDeduplication(t *testing.T) {
	cfg := &config.AppConfig{}
	state := &AppState{}
	sc := NewSyncCoordinator(cfg, state)

	var callCount int32
	// Substituímos o processador por um contador para o teste
	sc.processFunc = func(job IndexJob) {
		atomic.AddInt32(&callCount, 1)
	}

	sc.Start()
	defer sc.Stop()

	// Cenário: Disparar 10 atualizações seguidas para o mesmo arquivo
	for i := 0; i < 10; i++ {
		sc.Push("test.md", JobFileUpdate, false)
		time.Sleep(10 * time.Millisecond)
	}

	// O debounce é de 2 segundos. Vamos esperar 3 segundos para garantir.
	time.Sleep(3 * time.Second)

	finalCount := atomic.LoadInt32(&callCount)
	if finalCount != 1 {
		t.Errorf("Esperava exatamente 1 execução após debounce, mas obteve %d", finalCount)
	}
}

func TestCoordinatorMultipleFiles(t *testing.T) {
	cfg := &config.AppConfig{}
	state := &AppState{}
	sc := NewSyncCoordinator(cfg, state)

	var callCount int32
	sc.processFunc = func(job IndexJob) {
		atomic.AddInt32(&callCount, 1)
	}

	sc.Start()
	defer sc.Stop()

	// Cenário: Dois arquivos diferentes mudando ao mesmo tempo
	sc.Push("file1.md", JobFileUpdate, false)
	sc.Push("file2.md", JobFileUpdate, false)

	time.Sleep(3 * time.Second)

	finalCount := atomic.LoadInt32(&callCount)
	if finalCount != 2 {
		t.Errorf("Esperava 2 execuções (uma por arquivo), mas obteve %d", finalCount)
	}
}
