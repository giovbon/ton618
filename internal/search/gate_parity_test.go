package search

import (
	"testing"

	"ton618/core/internal/core/db"
)

// TestHasContentEvidenceParity fixa o contrato do gate de evidência: a decisão
// deve casar com o que o FTS realmente destacaria, evitando falsos negativos
// (remover doc com evidência real) e falsos positivos (manter match só-em-tag).
func TestHasContentEvidenceParity(t *testing.T) {
	tests := []struct {
		name     string
		doc      db.Document
		query    string
		expected bool
	}{
		{
			name: "acento dobrado pelo FTS conta como evidencia",
			doc: db.Document{
				Texto: "# Reunião\nReunião de equipe semanal.",
				Secao: "Reunião",
			},
			query:    "reuniao",
			expected: true,
		},
		{
			name: "prefixo (pythonica) conta como evidencia",
			doc: db.Document{
				Texto: "Programacao pythonica e boas praticas.",
				Secao: "Python",
			},
			query:    "python",
			expected: true,
		},
		{
			name: "match apenas como hashtag NAO e evidencia de conteudo",
			doc: db.Document{
				Texto: "Avaliacao de dados marcada para sexta. #programacao",
				Secao: "Prova",
			},
			query:    "programacao",
			expected: false,
		},
		{
			name: "termo ausente nao e evidencia",
			doc: db.Document{
				Texto: "Sobre musica classica e orquestras.",
				Secao: "Musica",
			},
			query:    "python",
			expected: false,
		},
		{
			name: "query so com stopword nao filtra (indefinido = true)",
			doc: db.Document{
				Texto: "Qualquer conteudo aqui.",
				Secao: "Geral",
			},
			query:    "de",
			expected: true,
		},
		{
			name: "match no arquivo conta como evidencia",
			doc: db.Document{
				Arquivo: "notas/go-lang.md",
				Texto:   "Conteudo qualquer.",
				Secao:   "Geral",
			},
			query:    "lang",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasContentEvidence(tt.doc, tt.query)
			if got != tt.expected {
				t.Errorf("HasContentEvidence(%q) = %v, want %v", tt.query, got, tt.expected)
			}
		})
	}
}
