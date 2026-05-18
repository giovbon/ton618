package search

import (
	"context"
	"encoding/json"
	"etl/internal/models"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildBleveQuery(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		isCompact bool
	}{
		{"Simple", "golang", false},
		{"Compact", "golang", true},
		{"Tags", "+tags:ai", false},
		{"Complex", `+tipo:pdf "machine learning" #ai`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := BuildBleveQuery(tt.raw, tt.isCompact)
			if q == nil {
				t.Errorf("BuildBleveQuery() returned nil for %s", tt.raw)
			}
		})
	}
}

func TestBuildBleveQuery_PhraseQueryPreserved(t *testing.T) {
	// Garante que frases entre aspas nao sao removidas pela limpeza de query
	q := BuildBleveQuery(`"inteligencia artificial"`, false)
	if q == nil {
		t.Fatal("BuildBleveQuery() returned nil for quoted phrase")
	}
	// Se a query contem PhraseQuery, o ExecuteSearch vai retornar resultados
	// mesmo que nao haja indice. Apenas verifica que nao deu panic.
}

func TestSearchCache(t *testing.T) {
	ClearCache()

	payload := []byte(`{"query":"teste"}`)
	result := models.SearchResults{}
	result.Hits.Total.Value = 1

	// 1. Deve ser MISS inicialmente
	if _, found := GetCachedResult(payload); found {
		t.Error("Esperava MISS para cache vazio")
	}

	// 2. Salvar e recuperar (HIT)
	SetCachedResult(payload, result)
	got, found := GetCachedResult(payload)
	if !found {
		t.Error("Esperava HIT após SetCachedResult")
	}
	if got.Hits.Total.Value != 1 {
		t.Errorf("Resultado recuperado incorreto: %v", got)
	}

	// 3. ClearCache deve invalidar
	ClearCache()
	if _, found := GetCachedResult(payload); found {
		t.Error("Esperava MISS após ClearCache")
	}
}

func TestCacheEviction(t *testing.T) {
	ClearCache()

	// Salvar maxEntries (15) + 1
	for i := 0; i < 16; i++ {
		key, _ := json.Marshal(map[string]int{"q": i})
		SetCachedResult(key, models.SearchResults{})
	}

	// O primeiro item (índice 0) deve ter sido expulso
	firstKey, _ := json.Marshal(map[string]int{"q": 0})
	if _, found := GetCachedResult(firstKey); found {
		t.Error("O primeiro item deveria ter sido expulso (LRU/Limit)")
	}

	// O último item deve estar presente
	lastKey, _ := json.Marshal(map[string]int{"q": 15})
	if _, found := GetCachedResult(lastKey); !found {
		t.Error("O último item deveria estar no cache")
	}
}

func TestCacheTTL(t *testing.T) {
	ClearCache()

	// Modificar TTL temporariamente para o teste
	oldTTL := ttl
	ttl = 10 * time.Millisecond
	defer func() { ttl = oldTTL }()

	payload := []byte(`{"q":"timeout"}`)
	SetCachedResult(payload, models.SearchResults{})

	// HIT imediato
	if _, found := GetCachedResult(payload); !found {
		t.Error("Deveria ser HIT imediato")
	}

	// Esperar TTL
	time.Sleep(15 * time.Millisecond)
	if _, found := GetCachedResult(payload); found {
		t.Error("Deveria ser MISS após expiração do TTL")
	}
}

func TestGetHeuristicTerms(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"Simple Text", "golang code", []string{"golang", "code"}},
		{"Only Tag", `+tags:"golang"`, []string{"golang"}},
		{"Mixed Text and Tag", `+tipo:"markdown" +tags:"ai" searching docs`, []string{"searching", "docs", "ai"}},
		{"Multiple Tags", `+tags:"go" +tags:"fast"`, []string{"go", "fast"}},
		{"Tag without +", `tags:"ai"`, []string{"ai"}},
		{"Hashtag Direct", `#youtube`, []string{"youtube"}},
		{"Hashtag with Accents", `#inteligência`, []string{"inteligência"}},
		{"Stopwords should be filtered", "o golang em acao", []string{"golang", "acao"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetHeuristicTerms(tt.raw)
			if len(got) != len(tt.want) {
				t.Errorf("GetHeuristicTerms() returned %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("At index %d: got %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExecuteSearch(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "search_exec.bleve")

	// Setup Index
	InitIndex(indexPath)
	defer CloseIndex()

	docID := "doc-1"
	docData := map[string]interface{}{
		"texto":   "Aprendendo Golang com testes automatizados",
		"arquivo": "golang.md",
		"tags":    []string{"go", "teste"},
		"tipo":    "markdown",
	}
	IndexDocument(docID, docData)

	// 1. Busca simples (HIT)
	res, err := ExecuteSearch(context.Background(), "golang", false, 0, 10)
	if err != nil {
		t.Fatalf("Erro ao executar busca: %v", err)
	}
	if res.Hits.Total.Value == 0 {
		t.Fatal("Esperado encontrar 1 documento, obteve 0")
	}

	// 2. Busca compacta
	resCompact, _ := ExecuteSearch(context.Background(), "golang.md", true, 0, 10)
	if resCompact.Hits.Total.Value == 0 {
		t.Error("Busca compacta por nome de arquivo deveria retornar resultados")
	}

	// 3. Filtro de sistema
	resFilter, _ := ExecuteSearch(context.Background(), "tipo:markdown", false, 0, 10)
	if resFilter.Hits.Total.Value == 0 {
		t.Error("Busca com filtro de sistema deveria retornar resultados")
	}
}

func TestExecuteSearch_PhraseExact(t *testing.T) {
	// Teste de regressao: busca por frase exata entre aspas deve retornar resultados
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "search_phrase.bleve")

	InitIndex(indexPath)
	defer CloseIndex()

	// Indexar documento com texto que contem a frase exata
	docData := map[string]interface{}{
		"texto":   "Este documento contem a frase exata inteligencia artificial para testes",
		"arquivo": "teste.md",
		"tipo":    "markdown",
	}
	IndexDocument("doc-phrase", docData)

	// Indexar documento SEM a frase exata
	docData2 := map[string]interface{}{
		"texto":   "Outro texto qualquer sobre inteligencia e tambem artificial mas fora de ordem",
		"arquivo": "outro.md",
		"tipo":    "markdown",
	}
	IndexDocument("doc-no-match", docData2)

	t.Run("Phrase com aspas deve encontrar correspondencia exata", func(t *testing.T) {
		res, err := ExecuteSearch(context.Background(), `"inteligencia artificial"`, false, 0, 10)
		if err != nil {
			t.Fatalf("Erro: %v", err)
		}
		if res.Hits.Total.Value == 0 {
			t.Fatal("PhraseQuery 'inteligencia artificial' deveria retornar resultados, mas retornou 0. Verifique se o indice foi construido com IncludeTermVectors=true")
		}
		// Deve encontrar APENAS o documento que tem a frase na ordem correta
		for _, hit := range res.Hits.Hits {
			if hit.Source.Arquivo == "outro.md" {
				t.Errorf("Documento 'outro.md' nao deveria corresponder a frase exata 'inteligencia artificial', mas foi retornado")
			}
		}
	})

	t.Run("Palavras soltas sem aspas ainda devem funcionar", func(t *testing.T) {
		res, err := ExecuteSearch(context.Background(), "inteligencia artificial", false, 0, 10)
		if err != nil {
			t.Fatalf("Erro: %v", err)
		}
		if res.Hits.Total.Value == 0 {
			t.Error("Busca por palavras soltas deveria retornar resultados")
		}
	})

	t.Run("Busca por termo que nao existe deve vir vazia", func(t *testing.T) {
		res, err := ExecuteSearch(context.Background(), `"termo inexistente XYZ123"`, false, 0, 10)
		if err != nil {
			t.Fatalf("Erro: %v", err)
		}
		if res.Hits.Total.Value > 0 {
			t.Error("Frase que nao existe no indice nao deveria retornar resultados")
		}
	})
}

func TestExecuteSearch_CaseInsensitiveCompact(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "search_case_insensitive.bleve")

	InitIndex(indexPath)
	defer CloseIndex()

	docData := map[string]interface{}{
		"texto":   "budegueira",
		"arquivo": "notes/Esperança.md",
		"tipo":    "markdown",
	}
	IndexDocument("doc-esperanca", docData)

	t.Run("Busca compacta com minúsculas e acentos deve encontrar", func(t *testing.T) {
		res, err := ExecuteSearch(context.Background(), "esperança", true, 0, 10)
		if err != nil {
			t.Fatalf("Erro ao buscar: %v", err)
		}
		if res.Hits.Total.Value == 0 {
			t.Fatal("Esperava encontrar 'notes/Esperança.md' ao buscar por 'esperança'")
		}
	})

	t.Run("Busca compacta com maiúsculas e acentos deve encontrar", func(t *testing.T) {
		res, err := ExecuteSearch(context.Background(), "Esperança", true, 0, 10)
		if err != nil {
			t.Fatalf("Erro ao buscar: %v", err)
		}
		if res.Hits.Total.Value == 0 {
			t.Fatal("Esperava encontrar 'notes/Esperança.md' ao buscar por 'Esperança'")
		}
	})

	t.Run("Busca compacta com prefixo minúsculo deve encontrar", func(t *testing.T) {
		res, err := ExecuteSearch(context.Background(), "espera", true, 0, 10)
		if err != nil {
			t.Fatalf("Erro ao buscar: %v", err)
		}
		if res.Hits.Total.Value == 0 {
			t.Fatal("Esperava encontrar 'notes/Esperança.md' ao buscar por 'espera'")
		}
	})

	t.Run("Busca compacta com termo sem acento deve encontrar", func(t *testing.T) {
		res, err := ExecuteSearch(context.Background(), "esperanca", true, 0, 10)
		if err != nil {
			t.Fatalf("Erro ao buscar: %v", err)
		}
		if res.Hits.Total.Value == 0 {
			t.Fatal("Esperava encontrar 'notes/Esperança.md' ao buscar por 'esperanca'")
		}
	})
}
