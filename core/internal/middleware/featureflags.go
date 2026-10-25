package middleware

import (
	"net/http"

	"ton618/core/internal/core/featureflags"
)

// WithFeatureFlags injeta no contexto de cada requisição o retrato atual das
// flags (lido do cache do Manager, sem query por request). Os templates usam
// featureflags.FromContext(ctx) — é assim que a Navbar esconde a Rotina no
// servidor, sem depender de JavaScript.
func WithFeatureFlags(manager *featureflags.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			flags := featureflags.Default()
			if manager != nil {
				flags = manager.Get()
			}
			next.ServeHTTP(w, r.WithContext(featureflags.WithFlags(r.Context(), flags)))
		})
	}
}
