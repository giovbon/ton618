package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"ton618/core/internal/core/featureflags"
)

type flagStore struct{ value string }

func (s *flagStore) GetSetting(string) (string, error) { return s.value, nil }
func (s *flagStore) SetSetting(_, value string) error  { s.value = value; return nil }

func TestWithFeatureFlags_InjetaEstadoDoCache(t *testing.T) {
	store := &flagStore{value: "0"}
	manager := featureflags.NewManager(store)

	var got bool
	handler := WithFeatureFlags(manager)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = featureflags.FromContext(r.Context()).Rotina
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got {
		t.Fatal("middleware deveria injetar a Rotina desligada")
	}
}

func TestWithFeatureFlags_ManagerNilUsaPadrao(t *testing.T) {
	var got bool
	handler := WithFeatureFlags(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = featureflags.FromContext(r.Context()).Rotina
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if !got {
		t.Fatal("sem manager, o padrão é habilitado")
	}
}
