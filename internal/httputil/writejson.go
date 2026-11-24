package httputil

import (
	"encoding/json"
	"net/http"
)

// WriteJSON escreve v como JSON em w, definindo Content-Type como application/json.
func WriteJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// WriteJSONStatus escreve v como JSON em w com o status HTTP especificado,
// definindo Content-Type como application/json.
func WriteJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
