package clustering

import (
	"testing"
)

func TestIsNoise(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"massa", false},
		{"horizonte", false},
		{"12345", true}, // Números
		{"concept", false},
	}

	for _, tt := range tests {
		if got := isNoise(tt.input); got != tt.expected {
			t.Errorf("isNoise(%q) = %v; want %v", tt.input, got, tt.expected)
		}
	}
}

func TestTokenize(t *testing.T) {
	input := "Esta é uma nota sobre #astronomia e massa-fresca!"
	expected := []string{"esta", "é", "uma", "nota", "sobre", "astronomia", "e", "massa", "fresca"}

	got := tokenize(input)
	if len(got) != len(expected) {
		t.Fatalf("tokenize length mismatch: got %v, want %v", got, expected)
	}

	for i := range got {
		if got[i] != expected[i] {
			t.Errorf("tokenize[%d] = %q; want %q", i, got[i], expected[i])
		}
	}
}

func TestStopwordsFilter(t *testing.T) {
	// Testar se as novas stopwords (PT e EN) estão no mapa unificado
	tests := []string{"estão", "fazer", "about", "after"}

	for _, word := range tests {
		if !stopwordMap[word] {
			t.Errorf("Stopword %q should be in stopwordMap", word)
		}
	}
}
