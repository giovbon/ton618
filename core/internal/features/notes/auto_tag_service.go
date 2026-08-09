package notes

import (
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"ton618/core/internal/core/db"
	"ton618/core/internal/core/domain"
)

// ApplyDecayTags verifica a idade das notas e aplica (ou remove)
// tags configuradas pelo usuário baseando-se na inatividade (idade do mtime).
func ApplyDecayTags(store *db.Store, noteSvc *NoteService) error {
	slog.Info("Iniciando varredura de auto-tagging (decay)...")
	
	val, err := store.GetSetting("auto_tag_decay_config")
	if err != nil || val == "" || val == "[]" {
		// Nenhuma regra configurada
		return nil
	}

	var rules []domain.AutoTagRule
	if err := json.Unmarshal([]byte(val), &rules); err != nil {
		slog.Error("ApplyDecayTags: erro ao dar parse nas regras", "error", err)
		return err
	}

	if len(rules) == 0 {
		return nil
	}

	notesList, err := noteSvc.GetMany()
	if err != nil {
		slog.Error("ApplyDecayTags: erro ao listar notas", "error", err)
		return err
	}

	modifiedCount := 0

	for _, n := range notesList {
		// Só aplica em notas markdown normais, ignorando anexos, pdfs, epubs
		if !strings.HasSuffix(n.Arquivo, ".md") {
			continue
		}
		if strings.HasPrefix(n.Arquivo, "archives/") {
			continue
		}

		mtime, err := time.Parse(time.RFC3339, n.Mtime)
		if err != nil {
			continue
		}

		ageDays := time.Since(mtime).Hours() / 24.0

		// Mapa rápido das tags atuais para busca e modificação
		currentTagsMap := make(map[string]bool)
		for _, t := range n.Tags {
			currentTagsMap[t] = true
		}

		tagsChanged := false

		// Para cada regra configurada, verifica se a nota atende o critério de idade
		for _, rule := range rules {
			targetTag := rule.Tag
			
			if ageDays >= float64(rule.Days) {
				// Deveria ter a tag
				if !currentTagsMap[targetTag] {
					currentTagsMap[targetTag] = true
					tagsChanged = true
				}
			} else {
				// NÃO deveria ter a tag (é jovem demais)
				if currentTagsMap[targetTag] {
					delete(currentTagsMap, targetTag)
					tagsChanged = true
				}
			}
		}

		if tagsChanged {
			// Reconstroi a lista de tags
			var newTags []string
			for t := range currentTagsMap {
				newTags = append(newTags, t)
			}

			// Lê o conteúdo atual
			content, err := store.GetNote(n.Arquivo)
			if err != nil {
				continue
			}

			// Atualiza a propriedade no frontmatter
			newTagsStr := strings.Join(newTags, ", ")
			
			newContent, err := UpdateFrontmatterProperty(content, "tags", newTagsStr)
			if err != nil {
				slog.Error("ApplyDecayTags: erro ao atualizar frontmatter", "file", n.Arquivo, "error", err)
				continue
			}

			// Salva a nota no DB com o mtime *antigo*, e apenas atualizamos as tags.
			if err := store.SaveNote(n.Arquivo, newContent, n.Mtime); err != nil {
				slog.Error("ApplyDecayTags: erro ao salvar no bd", "file", n.Arquivo, "error", err)
				continue
			}
			
			if err := store.SetFileTags(n.Arquivo, newTags); err != nil {
				slog.Error("ApplyDecayTags: erro ao setar file tags", "file", n.Arquivo, "error", err)
				continue
			}

			modifiedCount++
		}
	}

	if modifiedCount > 0 {
		slog.Info("Varredura de auto-tagging concluída", "notas_modificadas", modifiedCount)
	}
	return nil
}
