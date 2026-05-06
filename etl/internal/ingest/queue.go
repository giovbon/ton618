package ingest

import (
	"context"
	"log"
	"sync"
	"time"

	"etl/internal/config"
)

type JobType int

const (
	JobFullSync JobType = iota
	JobFileUpdate
)

type IndexJob struct {
	Type     JobType
	Path     string
	Force    bool
	Attempts int
}

type SyncCoordinator struct {
	cfg         *config.AppConfig
	state       *AppState
	queue       chan IndexJob
	pending     map[string]*time.Timer
	pmu         sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
	processFunc func(IndexJob)
}

func NewSyncCoordinator(cfg *config.AppConfig, state *AppState) *SyncCoordinator {
	ctx, cancel := context.WithCancel(context.Background())
	sc := &SyncCoordinator{
		cfg:     cfg,
		state:   state,
		queue:   make(chan IndexJob, 100),
		pending: make(map[string]*time.Timer),
		ctx:     ctx,
		cancel:  cancel,
	}
	sc.processFunc = sc.defaultProcessJob
	return sc
}

func (sc *SyncCoordinator) defaultProcessJob(job IndexJob) {
	if job.Type == JobFullSync {
		log.Printf("[Coordinator] Executando Sincronização Global (Force: %v)\n", job.Force)
		RunSync(sc.cfg, job.Force, "auto", sc.state)
	} else {
		log.Printf("[Coordinator] Processando atualização de arquivo: %s\n", job.Path)
		RunSync(sc.cfg, job.Force, "event", sc.state)
	}
}

func (sc *SyncCoordinator) Start() {
	log.Println("[Coordinator] Iniciando motor de sincronização...")

	// Worker principal
	go func() {
		for {
			select {
			case <-sc.ctx.Done():
				return
			case job := <-sc.queue:
				sc.processJob(job)
			}
		}
	}()
}

func (sc *SyncCoordinator) Stop() {
	sc.cancel()
}

// Push dispara uma tarefa de sincronização com debounce.
func (sc *SyncCoordinator) Push(path string, jobType JobType, force bool) {
	sc.pmu.Lock()
	defer sc.pmu.Unlock()

	// Se já existe um timer pendente para este arquivo, cancela o anterior (Debounce)
	if timer, ok := sc.pending[path]; ok {
		timer.Stop()
	}

	// Agenda a execução para daqui a 2 segundos
	sc.pending[path] = time.AfterFunc(2*time.Second, func() {
		sc.pmu.Lock()
		delete(sc.pending, path)
		sc.pmu.Unlock()

		sc.queue <- IndexJob{
			Type:  jobType,
			Path:  path,
			Force: force,
		}
	})
}

func (sc *SyncCoordinator) processJob(job IndexJob) {
	if sc.processFunc != nil {
		sc.processFunc(job)
	}
}

// Lock pausa o watcher de sincronização durante operações críticas (rename/delete).
func (sc *SyncCoordinator) Lock() {
	syncMu.Lock()
}

// Unlock libera o watcher de sincronização.
func (sc *SyncCoordinator) Unlock() {
	syncMu.Unlock()
}
