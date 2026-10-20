package notes

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseFrontmatter separa o bloco YAML inicial do conteúdo Markdown.
// Retorna o mapa de propriedades, o corpo e qualquer erro de parser.
func ParseFrontmatter(content string) (map[string]interface{}, string, error) {
	text := strings.TrimLeft(content, " \t\r\n\xef\xbb\xbf")

	// Verifica se começa com frontmatter
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return nil, text, nil
	}

	endIdx := strings.Index(text[4:], "\n---")
	if endIdx == -1 {
		return nil, text, nil
	}
	endIdx += 4

	yamlContent := text[4:endIdx]
	var fm map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
		return nil, text, err
	}

	body := text[endIdx+4:]
	// Remove até uma quebra de linha após os três traços, se existir
	if len(body) > 0 && body[0] == '\r' {
		body = body[1:]
	}
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	}

	return fm, body, nil
}

func toFlowStyleNode(val interface{}) interface{} {
	var list []string

	switch v := val.(type) {
	case []string:
		list = v
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				list = append(list, s)
			}
		}
	default:
		return val
	}

	node := &yaml.Node{
		Kind:  yaml.SequenceNode,
		Style: yaml.FlowStyle,
	}
	for _, s := range list {
		node.Content = append(node.Content, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: s,
		})
	}
	return node
}

// UpdateFrontmatterProperty altera ou insere uma chave no YAML do Markdown
// preservando o corpo da nota intacto.
func UpdateFrontmatterProperty(content string, key string, value interface{}) (string, error) {
	fm, body, err := ParseFrontmatter(content)
	if err != nil {
		return "", err
	}
	if fm == nil {
		fm = make(map[string]interface{})
	}

	// Normalize keys to lowercase to avoid duplicate keys (e.g. tags vs Tags)
	normalizedKey := strings.ToLower(key)
	for k, v := range fm {
		if strings.ToLower(k) == normalizedKey {
			delete(fm, k)
			fm[normalizedKey] = v
		}
	}

	// Se o valor for vazio ou nil, podemos considerar remover a chave (opcional)
	// Para este contexto, vamos manter a lógica de definir o valor ou deletar se string vazia.
	if strVal, isStr := value.(string); isStr && strVal == "" {
		delete(fm, normalizedKey)
	} else if value == nil {
		delete(fm, normalizedKey)
	} else {
		// Se a chave for tags e o valor string, tentar separar por vírgula
		if normalizedKey == "tags" {
			if s, ok := value.(string); ok {
				var tags []string
				for _, t := range strings.Split(s, ",") {
					t = strings.TrimSpace(t)
					t = strings.TrimPrefix(t, "#")
					if t != "" {
						tags = append(tags, t)
					}
				}
				fm[normalizedKey] = tags
			} else {
				fm[normalizedKey] = value
			}
		} else {
			fm[normalizedKey] = value
		}
	}

	// Se o map ficou vazio após remoções, podemos até remover o bloco
	if len(fm) == 0 {
		return strings.TrimLeft(body, " \t\r\n"), nil
	}

	// Força o campo tags a usar formato de lista flow (inline) [tag1, tag2]
	if tagsVal, ok := fm["tags"]; ok {
		fm["tags"] = toFlowStyleNode(tagsVal)
	}

	var yamlBuf strings.Builder
	yamlBuf.WriteString("---\n")
	encoder := yaml.NewEncoder(&yamlBuf)
	encoder.SetIndent(2)
	if err := encoder.Encode(fm); err != nil {
		return "", err
	}
	yamlBuf.WriteString("---\n")

	return yamlBuf.String() + body, nil
}

// MergeFrontmatterTags garante que TODAS as tags informadas estejam presentes no
// campo `tags:` do frontmatter, preservando as que já existem (união, sem
// duplicatas, comparação case-insensitive).
//
// Existe porque a reindexação (ReplaceFileIndexes) reconstrói a tabela `tags` a
// partir do conteúdo: tags que só vivem no banco (ex: tags de tipo persistidas
// por EnsureTypeTags) seriam perdidas ao duplicar/reescrever uma nota. Se nada
// novo for adicionado, o conteúdo é devolvido intacto.
func MergeFrontmatterTags(content string, extra []string) (string, error) {
	fm, _, err := ParseFrontmatter(content)
	if err != nil {
		return "", err
	}

	var current []string
	if fm != nil {
		switch v := fm["tags"].(type) {
		case []string:
			current = append(current, v...)
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok {
					current = append(current, s)
				}
			}
		case string:
			for _, t := range strings.Split(v, ",") {
				if t = strings.TrimSpace(t); t != "" {
					current = append(current, t)
				}
			}
		}
	}

	seen := make(map[string]bool, len(current)+len(extra))
	for _, t := range current {
		seen[strings.ToLower(strings.TrimSpace(t))] = true
	}

	added := false
	for _, t := range extra {
		t = strings.TrimSpace(strings.TrimPrefix(t, "#"))
		if t == "" || seen[strings.ToLower(t)] {
			continue
		}
		seen[strings.ToLower(t)] = true
		current = append(current, t)
		added = true
	}
	if !added {
		return content, nil
	}

	return UpdateFrontmatterProperty(content, "tags", strings.Join(current, ", "))
}
