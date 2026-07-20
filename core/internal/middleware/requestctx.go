package middleware

import (
	"context"
	"net/http"
)

// ctxKey é a chave para armazenar/recuperar o contexto da request.
type ctxKeyType struct{}

var reqCtxKey ctxKeyType

// WithRequestContext armazena o contexto da requisição HTTP no contexto da
// própria request, permitindo que handlers e serviços o extraiam via
// RequestContextFromCtx para propagar cancelamento até o banco de dados.
func WithRequestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), reqCtxKey, r.Context())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestContextFromCtx extrai o contexto da requisição HTTP do context.Context.
// Retorna o próprio ctx se a chave não for encontrada (fallback seguro).
func RequestContextFromCtx(ctx context.Context) context.Context {
	if rc, ok := ctx.Value(reqCtxKey).(context.Context); ok && rc != nil {
		return rc
	}
	return ctx
}
