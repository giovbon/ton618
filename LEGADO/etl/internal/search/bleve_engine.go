package search

import (
	"fmt"
	"os"
	"sync"

	"github.com/blevesearch/bleve/v2"
	_ "github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	_ "github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	_ "github.com/blevesearch/bleve/v2/analysis/analyzer/simple"
	_ "github.com/blevesearch/bleve/v2/analysis/analyzer/standard"
	_ "github.com/blevesearch/bleve/v2/analysis/char/asciifolding"
	"github.com/blevesearch/bleve/v2/analysis/lang/pt"
	_ "github.com/blevesearch/bleve/v2/analysis/token/lowercase"
	_ "github.com/blevesearch/bleve/v2/analysis/tokenizer/single"
	"github.com/blevesearch/bleve/v2/mapping"
)

var (
	index   bleve.Index
	indexMu sync.RWMutex
)

// InitIndex inicializa o índice Bleve.
func InitIndex(path string) error {
	indexMu.Lock()
	defer indexMu.Unlock()

	// Se já existe um índice aberto, fecha antes de abrir um novo (importante para testes)
	if index != nil {
		index.Close()
		index = nil
	}

	var newIndex bleve.Index
	var err error
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		// Criar novo índice
		mapping, mErr := buildMapping()
		if mErr != nil {
			return mErr
		}
		newIndex, err = bleve.New(path, mapping)
	} else {
		// Abrir índice existente
		newIndex, err = bleve.Open(path)
	}

	if err != nil {
		return err
	}

	index = newIndex
	return nil
}

// GetIndex retorna a instância global do índice.
func GetIndex() bleve.Index {
	indexMu.RLock()
	defer indexMu.RUnlock()
	return index
}

// CloseIndex fecha o índice de forma segura.
func CloseIndex() error {
	indexMu.Lock()
	defer indexMu.Unlock()
	if index != nil {
		err := index.Close()
		index = nil
		return err
	}
	return nil
}

// buildMapping configura como os campos dos documentos serão indexados.
func buildMapping() (mapping.IndexMapping, error) {
	// Analisador de Português
	ptMapping := bleve.NewTextFieldMapping()
	ptMapping.Analyzer = pt.AnalyzerName

	// Mapeamento de Documentos
	docMapping := bleve.NewDocumentMapping()

	// Campos de texto para busca
	textMapping := bleve.NewTextFieldMapping()
	textMapping.Analyzer = pt.AnalyzerName
	textMapping.Store = true
	textMapping.IncludeTermVectors = true // Necessario para PhraseQuery (busca por frases exatas)
	docMapping.AddFieldMappingsAt("texto", textMapping)

	secaoMapping := bleve.NewTextFieldMapping()
	secaoMapping.Analyzer = pt.AnalyzerName
	secaoMapping.Store = true
	secaoMapping.IncludeTermVectors = true
	docMapping.AddFieldMappingsAt("secao", secaoMapping)

	// Caminhos de arquivo (Custom Lowercase Keyword Analyzer para busca case-insensitive, deleção exata e vacuum seguro)
	pathMapping := bleve.NewTextFieldMapping()
	pathMapping.Analyzer = "lowercase_keyword"
	pathMapping.Store = true
	docMapping.AddFieldMappingsAt("arquivo", pathMapping)

	// Tags (keywords) e armazenadas
	tagMapping := bleve.NewTextFieldMapping()
	tagMapping.Analyzer = "keyword"
	tagMapping.Store = true
	docMapping.AddFieldMappingsAt("tags", tagMapping)

	// Metadados
	// Metadados (Stored e Keyword para tipos)
	typeMapping := bleve.NewTextFieldMapping()
	typeMapping.Analyzer = "keyword"
	typeMapping.Store = true
	docMapping.AddFieldMappingsAt("tipo", typeMapping)

	// Data de modificação (Stored para freshness score)
	timeMapping := bleve.NewTextFieldMapping()
	timeMapping.Analyzer = "keyword"
	timeMapping.Store = true
	docMapping.AddFieldMappingsAt("@timestamp", timeMapping)

	docMapping.AddFieldMappingsAt("ordem", bleve.NewNumericFieldMapping())
	docMapping.AddFieldMappingsAt("pagina", bleve.NewNumericFieldMapping())

	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultMapping = docMapping
	indexMapping.DefaultAnalyzer = pt.AnalyzerName

	// Registrar o analyzer customizado para caminhos case-insensíveis com folding de acentos
	err := indexMapping.AddCustomAnalyzer("lowercase_keyword", map[string]interface{}{
		"type":          "custom",
		"char_filters":  []interface{}{"asciifolding"},
		"tokenizer":     "single",
		"token_filters": []interface{}{"to_lower"},
	})
	if err != nil {
		return nil, err
	}

	return indexMapping, nil
}

// IndexDocument adiciona ou atualiza um documento no índice.
func IndexDocument(id string, data interface{}) error {
	idx := GetIndex()
	if idx == nil {
		return fmt.Errorf("índice não inicializado")
	}
	return idx.Index(id, data)
}

// BatchIndexDocuments realiza a indexação de múltiplos documentos em uma única transação (lote).
func BatchIndexDocuments(docs map[string]interface{}) error {
	idx := GetIndex()
	if idx == nil {
		return fmt.Errorf("índice não inicializado")
	}

	batch := idx.NewBatch()
	for id, doc := range docs {
		if err := batch.Index(id, doc); err != nil {
			return err
		}
	}
	return idx.Batch(batch)
}

// DeleteDocument remove um documento do índice.
func DeleteDocument(id string) error {
	idx := GetIndex()
	if idx == nil {
		return fmt.Errorf("índice não inicializado")
	}
	return idx.Delete(id)
}
