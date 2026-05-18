package ingest

import (
	"encoding/json"
	"testing"

	"etl/internal/config"

	bolt "go.etcd.io/bbolt"
)

// TestNoteVectorBackwardCompat verifica que GetAllNoteVectors consegue ler
// tanto o formato novo ({"v":[...],"t":"..."}) quanto o formato antigo
// (apenas array de float32), e que os dados sobrevivem a um close/reopen.
func TestNoteVectorBackwardCompat(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.AppConfig{StateDir: tmpDir}

	appState := NewAppState(cfg)

	// Escreve os dois formatos diretamente no bucket note_vectors.
	// Formato novo: {"v": [1.0, 2.0], "t": "My Title"}
	nv := NoteVector{Vector: []float32{1.0, 2.0}, Title: "My Title"}
	newFormatData, _ := json.Marshal(nv)

	// Formato antigo: apenas array [3.0, 4.0, 5.0]
	oldVec := []float32{3.0, 4.0, 5.0}
	oldFormatData, _ := json.Marshal(oldVec)

	appState.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNoteVectors)
		if b == nil {
			t.Fatal("bucket note_vectors nao encontrado")
		}
		if err := b.Put([]byte("notes/new_key"), newFormatData); err != nil {
			return err
		}
		if err := b.Put([]byte("notes/old_key"), oldFormatData); err != nil {
			return err
		}
		return nil
	})

	// Fecha e reabre
	appState.Close()
	appState = NewAppState(cfg)
	defer appState.Close()

	// Carrega (popula o cache do VectorManager a partir do DB)
	appState.Load(cfg)

	all := appState.GetAllNoteVectors()

	// Verifica entrada no formato novo
	newEntry, ok := all["notes/new_key"]
	if !ok {
		t.Fatal("entrada 'notes/new_key' (formato novo) nao foi recuperada")
	}
	if newEntry.Title != "My Title" {
		t.Errorf("titulo esperado 'My Title', obteve '%s'", newEntry.Title)
	}
	if len(newEntry.Vector) != 2 || newEntry.Vector[0] != 1.0 || newEntry.Vector[1] != 2.0 {
		t.Errorf("vetor esperado [1.0, 2.0], obteve %v", newEntry.Vector)
	}

	// Verifica entrada no formato antigo
	oldEntry, ok := all["notes/old_key"]
	if !ok {
		t.Fatal("entrada 'notes/old_key' (formato antigo) nao foi recuperada")
	}
	if oldEntry.Title != "" {
		t.Errorf("titulo esperado vazio (old format), obteve '%s'", oldEntry.Title)
	}
	if len(oldEntry.Vector) != 3 || oldEntry.Vector[0] != 3.0 || oldEntry.Vector[1] != 4.0 || oldEntry.Vector[2] != 5.0 {
		t.Errorf("vetor esperado [3.0, 4.0, 5.0], obteve %v", oldEntry.Vector)
	}
}

// TestDeleteNoteProjectionGranular verifica que DeleteNoteProjection("a")
// remove apenas a projecao "a", mantendo "b" e "c" intactas.
func TestDeleteNoteProjectionGranular(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.AppConfig{StateDir: tmpDir}

	appState := NewAppState(cfg)
	defer appState.Close()

	// Insere 3 projecoes
	appState.SetNoteProjections(map[string][]float64{
		"a": {1.0, 2.0},
		"b": {3.0, 4.0},
		"c": {5.0, 6.0},
	})

	// Confirma que as 3 foram inseridas
	all := appState.GetAllNoteProjections()
	if len(all) != 3 {
		t.Fatalf("esperado 3 projecoes apos insercao, obteve %d", len(all))
	}

	// Remove apenas "a"
	appState.DeleteNoteProjection("a")

	// Verifica: "a" deve ter sumido, "b" e "c" devem permanecer
	all = appState.GetAllNoteProjections()
	if len(all) != 2 {
		t.Fatalf("esperado 2 projecoes apos delecao de 'a', obteve %d", len(all))
	}

	if _, ok := all["a"]; ok {
		t.Error("projecao 'a' deveria ter sido removida, mas ainda existe")
	}

	b, ok := all["b"]
	if !ok {
		t.Error("projecao 'b' deveria permanecer, mas foi removida")
	}
	if len(b) != 2 || b[0] != 3.0 || b[1] != 4.0 {
		t.Errorf("projecao 'b' corrompida: %v", b)
	}

	c, ok := all["c"]
	if !ok {
		t.Error("projecao 'c' deveria permanecer, mas foi removida")
	}
	if len(c) != 2 || c[0] != 5.0 || c[1] != 6.0 {
		t.Errorf("projecao 'c' corrompida: %v", c)
	}

	// Verifica persistencia: fecha, reabre e confirma novamente
	appState.Close()
	appState = NewAppState(cfg)
	defer appState.Close()
	appState.Load(cfg)

	all = appState.GetAllNoteProjections()
	if len(all) != 2 {
		t.Fatalf("pos reopen: esperado 2 projecoes, obteve %d", len(all))
	}
	if _, ok := all["a"]; ok {
		t.Error("pos reopen: projecao 'a' deveria ter sido removida, mas ainda existe")
	}
	if _, ok := all["b"]; !ok {
		t.Error("pos reopen: projecao 'b' deveria permanecer")
	}
	if _, ok := all["c"]; !ok {
		t.Error("pos reopen: projecao 'c' deveria permanecer")
	}
}

// TestSetNoteVectorWithTitle verifica que SetNoteVector armazena corretamente
// o vetor e o titulo, e que GetAllNoteVectors os recupera integralmente.
func TestSetNoteVectorWithTitle(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.AppConfig{StateDir: tmpDir}

	appState := NewAppState(cfg)
	defer appState.Close()

	vec := []float32{0.1, 0.2, 0.3, 0.4, 0.5}
	title := "Hello World"

	appState.SetNoteVector("notes/n1", vec, title)

	// Recupera sem reabrir (em memoria + DB)
	all := appState.GetAllNoteVectors()
	nv, ok := all["notes/n1"]
	if !ok {
		t.Fatal("SetNoteVector nao armazenou a entrada 'notes/n1'")
	}
	if nv.Title != title {
		t.Errorf("titulo esperado '%s', obteve '%s'", title, nv.Title)
	}
	if len(nv.Vector) != len(vec) {
		t.Fatalf("tamanho do vetor esperado %d, obteve %d", len(vec), len(nv.Vector))
	}
	for i, v := range vec {
		if nv.Vector[i] != v {
			t.Errorf("vetor[%d] esperado %f, obteve %f", i, v, nv.Vector[i])
		}
	}

	// Verifica persistencia: fecha, reabre e confirma
	appState.Close()
	appState = NewAppState(cfg)
	defer appState.Close()
	appState.Load(cfg)

	all = appState.GetAllNoteVectors()
	nv, ok = all["notes/n1"]
	if !ok {
		t.Fatal("pos reopen: entrada 'notes/n1' nao foi recuperada")
	}
	if nv.Title != title {
		t.Errorf("pos reopen: titulo esperado '%s', obteve '%s'", title, nv.Title)
	}
	if len(nv.Vector) != len(vec) {
		t.Fatalf("pos reopen: tamanho do vetor esperado %d, obteve %d", len(vec), len(nv.Vector))
	}
	for i, v := range vec {
		if nv.Vector[i] != v {
			t.Errorf("pos reopen: vetor[%d] esperado %f, obteve %f", i, v, nv.Vector[i])
		}
	}
}

func TestBinaryEncodingRoundTrip(t *testing.T) {
	nv := NoteVector{
		Vector: []float32{0.1, -0.2, 0.3, -0.4, 0.5},
		Title:  "Teste de Encoding Binario",
		X:      42.5,
		Y:      87.3,
	}

	// Encodificar
	encoded := encodeNoteVector(nv)
	if len(encoded) == 0 {
		t.Fatal("encodeNoteVector retornou vazio")
	}

	// Verificar magic byte
	if encoded[0] != binMagic {
		t.Errorf("magic byte esperado 0xFF, obteve 0x%02X", encoded[0])
	}

	// Decodificar
	decoded := decodeNoteVector(encoded)

	if decoded.Title != nv.Title {
		t.Errorf("titulo: esperado '%s', obteve '%s'", nv.Title, decoded.Title)
	}
	if decoded.X != nv.X || decoded.Y != nv.Y {
		t.Errorf("coords 2D: esperado (%f,%f), obteve (%f,%f)", nv.X, nv.Y, decoded.X, decoded.Y)
	}
	if len(decoded.Vector) != len(nv.Vector) {
		t.Fatalf("tamanho vetor: esperado %d, obteve %d", len(nv.Vector), len(decoded.Vector))
	}
	for i, v := range nv.Vector {
		if decoded.Vector[i] != v {
			t.Errorf("vetor[%d]: esperado %f, obteve %f", i, v, decoded.Vector[i])
		}
	}
}
