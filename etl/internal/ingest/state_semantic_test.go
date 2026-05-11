package ingest

import (
	"os"
	"testing"

	bolt "go.etcd.io/bbolt"
)

func TestRebuildSemanticTopics(t *testing.T) {
	// Setup DB
	dbFile := "test_semantic_rebuild.db"
	defer os.Remove(dbFile)

	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		t.Fatalf("Erro ao abrir db: %v", err)
	}
	defer db.Close()

	// Inicializar buckets (normalmente feito pelo AppState, mas faremos manual para o teste isolado)
	db.Update(func(tx *bolt.Tx) error {
		tx.CreateBucketIfNotExists(bucketFileSemanticLinks)
		tx.CreateBucketIfNotExists(bucketSemanticTopics)
		return nil
	})

	sm := newSemanticManager(db)

	// Injetar lixo no banco de dados para simular corrupção ou órfãos
	db.Update(func(tx *bolt.Tx) error {
		bt := tx.Bucket(bucketSemanticTopics)
		bt.Put([]byte("fantasma1"), []byte("true"))
		bt.Put([]byte("fantasma2"), []byte("true"))
		return nil
	})

	// Injetar links válidos
	validLinks1 := []string{"topico_valido", "outro_topico"}
	validLinks2 := []string{"outro_topico", "mais_um"}
	
	// Gravar links no manager (que também grava no banco)
	sm.SetFileSemanticLinks("nota1.md", validLinks1)
	sm.SetFileSemanticLinks("nota2.md", validLinks2)

	// No momento o manager tem os validos E os fantasmas no bucket de topicos
	var countBefore int
	db.View(func(tx *bolt.Tx) error {
		bt := tx.Bucket(bucketSemanticTopics)
		countBefore = bt.Stats().KeyN
		return nil
	})

	// Esperamos os 3 válidos (topico_valido, outro_topico, mais_um) + 2 fantasmas (fantasma1, fantasma2) = 5
	if countBefore != 5 {
		t.Errorf("Esperava 5 tópicos antes do rebuild, obteve %d", countBefore)
	}

	// Executar reconstrução!
	sm.RebuildSemanticTopics()

	// Verificar se os tópicos fantasmas foram deletados e apenas os válidos restaram
	var countAfter int
	db.View(func(tx *bolt.Tx) error {
		bt := tx.Bucket(bucketSemanticTopics)
		countAfter = bt.Stats().KeyN

		if bt.Get([]byte("fantasma1")) != nil {
			t.Errorf("fantasma1 não foi limpo do DB!")
		}
		if bt.Get([]byte("topico_valido")) == nil {
			t.Errorf("topico_valido deveria estar no DB!")
		}
		return nil
	})

	if countAfter != 3 {
		t.Errorf("Esperava 3 tópicos após rebuild (limpos), obteve %d", countAfter)
	}

	// Checar cache em memória
	topicsCache := sm.GetAllSemanticTopics()
	if len(topicsCache) != 3 {
		t.Errorf("Cache em memória deveria ter 3 tópicos, obteve %d", len(topicsCache))
	}
}
