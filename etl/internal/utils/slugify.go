package utils

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// SlugifyFilename limpa o nome do arquivo, remove acentos e limita o tamanho.
func SlugifyFilename(name string) string {
	if name == "" {
		return ""
	}

	// Caso especial: arquivo oculto (ex: .hidden)
	if strings.HasPrefix(name, ".") && !strings.Contains(name[1:], ".") {
		return name
	}

	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)

	// Lowercase e remoção de acentos via map (expandido)
	base = strings.ToLower(base)
	base = strings.NewReplacer(
		"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
		"é", "e", "è", "e", "ê", "e", "ë", "e",
		"í", "i", "ì", "i", "î", "i", "ï", "i",
		"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
		"ú", "u", "ù", "u", "û", "u", "ü", "u",
		"ç", "c", "ñ", "n",
	).Replace(base)

	// Substituir qualquer coisa que não seja a-z ou 0-9 por hífen
	var result strings.Builder
	for _, r := range base {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
		} else {
			// Evitar hífens duplicados ou no início
			if result.Len() > 0 && result.String()[result.Len()-1] != '-' {
				result.WriteRune('-')
			}
		}
	}

	finalBase := strings.Trim(result.String(), "-")

	// Limitar tamanho (max 50 chars para o base)
	if len(finalBase) > 50 {
		finalBase = finalBase[:50]
		finalBase = strings.Trim(finalBase, "-")
	}

	if finalBase == "" {
		if ext != "" {
			// Se sobrou só a extensão, tenta usar um placeholder
			finalBase = "file"
		} else {
			return fmt.Sprintf("upload-%d", time.Now().Unix())
		}
	}

	return finalBase + strings.ToLower(ext)
}
