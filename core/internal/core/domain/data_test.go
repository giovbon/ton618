package domain

import (
	"testing"
)

func TestDetectNoteType_Mermaid(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		content  string
		arquivo  string
		expected NoteType
	}{
		{
			name:     "Explicit tag",
			tags:     []string{"mermaid"},
			content:  "some random text",
			arquivo:  "notes/Calendário Acadêmico 2026-2.md",
			expected: NoteTypeMermaid,
		},
		{
			name:     "Frontmatter type: mermaid",
			tags:     nil,
			content:  "---\ntype: mermaid\n---\ngantt title Calendário Acadêmico",
			arquivo:  "notes/Calendário Acadêmico 2026-2.md",
			expected: NoteTypeMermaid,
		},
		{
			name:     "Markdown code block ```mermaid",
			tags:     nil,
			content:  "```mermaid\ngraph TD\nA --> B\n```",
			arquivo:  "notes/Diagrama.md",
			expected: NoteTypeMermaid,
		},
		{
			name:     "Direct gantt content without frontmatter or tag",
			tags:     nil,
			content:  "gantt title Calendário Acadêmico 2026-2 dateFormat YYYY-MM-DD\nsection AGO\nInício veteranos :a2, 2026-08-03, 1d",
			arquivo:  "notes/Calendário Acadêmico 2026-2.md",
			expected: NoteTypeMermaid,
		},
		{
			name:     "Direct graph content without frontmatter or tag",
			tags:     nil,
			content:  "graph TD\n    A[Início] --> B(Fim)",
			arquivo:  "notes/Fluxo.md",
			expected: NoteTypeMermaid,
		},
		{
			name:     "Normal markdown note",
			tags:     nil,
			content:  "# Minha nota normal\nEste é um texto comum.",
			arquivo:  "notes/Calendário Acadêmico 2026-2.md",
			expected: NoteTypeMarkdown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectNoteType(tt.tags, tt.content, tt.arquivo)
			if result != tt.expected {
				t.Errorf("DetectNoteType() = %v, want %v", result, tt.expected)
			}
		})
	}
}
