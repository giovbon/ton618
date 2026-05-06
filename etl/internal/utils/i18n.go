package utils

// GetTag retorna a tag traduzida de acordo com o idioma.
func GetTag(lang string, key string) string {
	// Mapa de tags por idioma
	tags := map[string]map[string]string{
		"pt-BR": {
			"imagem":      "imagem",
			"pdf":         "pdf",
			"arquivos":    "arquivos",
			"captura_url": "captura_url",
			"captura_yt":  "captura_yt",
		},
		"en-US": {
			"imagem":      "image",
			"pdf":         "pdf",
			"arquivos":    "files",
			"captura_url": "capture_url",
			"captura_yt":  "capture_yt",
		},
	}

	if l, ok := tags[lang]; ok {
		if t, ok := l[key]; ok {
			return t
		}
	}

	// Fallback para pt-BR se não encontrar
	if t, ok := tags["pt-BR"][key]; ok {
		return t
	}

	return key
}
