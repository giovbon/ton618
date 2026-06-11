package service

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

	// Se o valor for vazio ou nil, podemos considerar remover a chave (opcional)
	// Para este contexto, vamos manter a lógica de definir o valor ou deletar se string vazia.
	if strVal, isStr := value.(string); isStr && strVal == "" {
		delete(fm, key)
	} else if value == nil {
		delete(fm, key)
	} else {
		// Se a chave for tags e o valor string, tentar separar por vírgula
		if key == "tags" {
			if s, ok := value.(string); ok {
				var tags []string
				for _, t := range strings.Split(s, ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						tags = append(tags, t)
					}
				}
				fm[key] = tags
			} else {
				fm[key] = value
			}
		} else {
			fm[key] = value
		}
	}

	// Se o map ficou vazio após remoções, podemos até remover o bloco
	if len(fm) == 0 {
		return strings.TrimLeft(body, " \t\r\n"), nil
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
