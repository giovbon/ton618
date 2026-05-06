package utils

import (
	"strings"
	"testing"
)

func TestSlugifyFilename_Comprehensive(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Normal File.pdf", "normal-file.pdf"},
		{"File With spaces  .md", "file-with-spaces.md"},
		{"UPPERCASE.PNG", "uppercase.png"},
		{"accentué à l'école.txt", "accentue-a-l-ecole.txt"},
		{"!@#$%^&*().pdf", "file.pdf"}, // Caracteres especiais removidos, vira 'file'
		{"very_long_filename_that_should_be_truncated_at_some_point_hopefully_it_works_well.md", "very-long-filename-that-should-be-truncated-at-som.md"},
		{"multiple...dots.pdf", "multiple-dots.pdf"},
		{"---dashes---.md", "dashes.md"},
		{"", ""},
		{" ", "upload-"}, // Vai começar com upload- timestamps
		{".hidden", ".hidden"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SlugifyFilename(tt.input)
			if tt.expected == "upload-" {
				if !strings.HasPrefix(got, "upload-") {
					t.Errorf("SlugifyFilename(%q) = %q, want prefix upload-", tt.input, got)
				}
			} else {
				if got != tt.expected {
					t.Errorf("SlugifyFilename(%q) = %q, want %q", tt.input, got, tt.expected)
				}
			}
		})
	}
}
