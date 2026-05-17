package ingest

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/blevesearch/bleve/v2"

	"etl/internal/config"
	"etl/internal/search"
)

// VacuumOrphans remove fragmentos do índice que não existem mais no arquivo físico atual.
func VacuumOrphans(cfg *config.AppConfig, filename string, expectedIds []string) {
	idx := search.GetIndex()
	if idx == nil {
		return
	}
	expected := make(map[string]bool)
	for _, id := range expectedIds {
		expected[id] = true
	}
	query := bleve.NewTermQuery(strings.ToLower(filename))
	query.SetField("arquivo")
	req := bleve.NewSearchRequest(query)
	req.Size = 1000
	results, err := idx.Search(req)
	if err != nil {
		return
	}
	for _, hit := range results.Hits {
		if !expected[hit.ID] {
			search.DeleteDocument(hit.ID)
		}
	}
}

// GlobalVacuum realiza uma limpeza profunda, removendo do índice e do estado qualquer arquivo que não exista no disco.
func GlobalVacuum(cfg *config.AppConfig, appState *AppState) {
	idx := search.GetIndex()
	if idx == nil {
		return
	}
	const pageSize = 500
	query := bleve.NewMatchAllQuery()
	arquivosNoIndice := make(map[string]bool)
	for from := 0; ; from += pageSize {
		req := bleve.NewSearchRequestOptions(query, pageSize, from, false)
		req.Fields = []string{"arquivo"}
		res, err := idx.Search(req)
		if err != nil || len(res.Hits) == 0 {
			break
		}
		for _, hit := range res.Hits {
			if pathVal, ok := hit.Fields["arquivo"]; ok {
				if s, ok := pathVal.(string); ok {
					arquivosNoIndice[s] = true
				}
			}
		}
		if len(res.Hits) < pageSize {
			break
		}
	}
	for filename := range arquivosNoIndice {
		if _, err := os.Stat(filepath.Join(cfg.DocsDir, filename)); os.IsNotExist(err) {
			DeleteFileFromBleve(cfg, filename)
		}
	}

	// 2. Limpeza do AppState (BBolt)
	if appState != nil {
		allMetadata := appState.GetAllFileMetadata()
		for relPath := range allMetadata {
			absPath := filepath.Join(cfg.DocsDir, relPath)
			if _, err := os.Stat(absPath); os.IsNotExist(err) {
				log.Printf("[Vacuum] Removendo metadados órfãos do Estado: %s\n", relPath)
				appState.DeleteFileTags(relPath)
				appState.DeleteFileLinks(relPath)
				appState.DeleteFileSemanticLinks(relPath)
				appState.DeleteFileMetadata(relPath)
				appState.DeleteVectorHash(relPath)
				
				
				// Limpar também do cache de modificação
				appState.DeleteFileMod(absPath)
			}
		}

		allSemanticLinks := appState.GetAllFileSemanticLinks()
		for relPath := range allSemanticLinks {
			absPath := filepath.Join(cfg.DocsDir, relPath)
			if _, err := os.Stat(absPath); os.IsNotExist(err) {
				log.Printf("[Vacuum] Limpando links semânticos órfãos: %s\n", relPath)
				appState.DeleteFileSemanticLinks(relPath)
			}
		}

		appState.RebuildSemanticTopics()
		appState.RebuildKnownTagsCache()
	}
}

// DeleteFileFromBleve remove todas as entradas de um arquivo do motor de busca clássico de forma eficiente.
func DeleteFileFromBleve(cfg *config.AppConfig, filename string) {
	idx := search.GetIndex()
	if idx == nil {
		return
	}

	// Busca por termo exato no campo 'arquivo' (mapeado como keyword)
	query := bleve.NewTermQuery(strings.ToLower(filename))
	query.SetField("arquivo")
	req := bleve.NewSearchRequest(query)
	req.Size = 1000 // Aumentar se um único arquivo puder ter mais de 1000 fragmentos

	res, err := idx.Search(req)
	if err != nil {
		log.Printf("[Erro] Falha ao buscar fragmentos para deleção (%s): %v\n", filename, err)
		return
	}

	if len(res.Hits) == 0 {
		return
	}

	batch := idx.NewBatch()
	for _, hit := range res.Hits {
		batch.Delete(hit.ID)
	}

	if err := idx.Batch(batch); err != nil {
		log.Printf("[Erro] Falha ao executar batch de deleção (%s): %v\n", filename, err)
	} else {
		log.Printf("[Sync] Sincronização concluída: %d fragmentos de %s removidos do índice.\n", len(res.Hits), filename)
	}
}

// CollectBleveIDsForFile retorna os IDs de todos os fragmentos de um arquivo no Bleve.
func CollectBleveIDsForFile(cfg *config.AppConfig, filename string) []string {
	idx := search.GetIndex()
	if idx == nil {
		return nil
	}

	query := bleve.NewTermQuery(strings.ToLower(filename))
	query.SetField("arquivo")
	req := bleve.NewSearchRequest(query)
	req.Size = 1000

	res, err := idx.Search(req)
	if err != nil {
		return nil
	}

	var ids []string
	for _, hit := range res.Hits {
		ids = append(ids, hit.ID)
	}
	return ids
}

// CleanupExpiredBundles remove notas de bundle e arquivos ZIP que passaram da data de expiração.
