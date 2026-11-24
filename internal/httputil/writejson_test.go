package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type sampleData struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	data := sampleData{Name: "ton618", Value: 42}

	WriteJSON(rec, data)

	if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Content-Type incorreto: got %q, want %q", contentType, "application/json")
	}

	if status := rec.Code; status != http.StatusOK {
		t.Errorf("Status code incorreto: got %d, want %d", status, http.StatusOK)
	}

	var res sampleData
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("Erro ao decodificar JSON de resposta: %v", err)
	}

	if res != data {
		t.Errorf("Conteúdo JSON incorreto: got %+v, want %+v", res, data)
	}
}

func TestWriteJSONStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	data := map[string]string{"error": "not found"}

	WriteJSONStatus(rec, http.StatusNotFound, data)

	if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Content-Type incorreto: got %q, want %q", contentType, "application/json")
	}

	if status := rec.Code; status != http.StatusNotFound {
		t.Errorf("Status code incorreto: got %d, want %d", status, http.StatusNotFound)
	}

	var res map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("Erro ao decodificar JSON de resposta: %v", err)
	}

	if res["error"] != "not found" {
		t.Errorf("Conteúdo JSON incorreto: got %v, want %v", res["error"], "not found")
	}
}
