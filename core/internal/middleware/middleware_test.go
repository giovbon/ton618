package middleware

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// ── checkCredentials ────────────────────────────────────────────

func TestCheckCredentials_BasicAuthHeader(t *testing.T) {
	// Via r.BasicAuth() — qualquer usuário, senha correta
	req := httptest.NewRequest("GET", "/", nil)
	req.SetBasicAuth("ton618", "ton618")

	if !checkCredentials(req, "ton618") {
		t.Error("senha correta via BasicAuth() deveria retornar true")
	}
}

func TestCheckCredentials_BasicAuthHeader_SenhaErrada(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.SetBasicAuth("ton618", "senha-errada")

	if checkCredentials(req, "ton618") {
		t.Error("senha errada deveria retornar false")
	}
}

func TestCheckCredentials_UsuarioDiferenteIgnorado(t *testing.T) {
	// Usuário diferente mas senha correta — deve ser aceito (username é ignorado)
	req := httptest.NewRequest("GET", "/", nil)
	req.SetBasicAuth("qualquer-usuario", "ton618")

	if !checkCredentials(req, "ton618") {
		t.Error("username é ignorado: senha correta com qualquer usuário deveria retornar true")
	}
}

func TestCheckCredentials_SemAuthHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)

	if checkCredentials(req, "ton618") {
		t.Error("sem auth header deveria retornar false")
	}
}

func TestCheckCredentials_AuthorizationHeaderManual(t *testing.T) {
	// Header Authorization setado manualmente (como o JS faz)
	req := httptest.NewRequest("GET", "/", nil)
	raw := base64.StdEncoding.EncodeToString([]byte("ton618:ton618"))
	req.Header.Set("Authorization", "Basic "+raw)

	if !checkCredentials(req, "ton618") {
		t.Error("senha correta via Authorization header manual deveria retornar true")
	}
}

func TestCheckCredentials_CookieBase64Bruto(t *testing.T) {
	// Cookie com base64 bruto (formato atual)
	req := httptest.NewRequest("GET", "/", nil)
	raw := base64.StdEncoding.EncodeToString([]byte("ton618:ton618"))
	req.AddCookie(&http.Cookie{Name: "ton_auth", Value: raw})

	if !checkCredentials(req, "ton618") {
		t.Error("cookie com base64 bruto deveria retornar true")
	}
}

func TestCheckCredentials_CookieUrlEncoded(t *testing.T) {
	// Cookie URL-encoded (ex: se encodeURIComponent foi usado)
	req := httptest.NewRequest("GET", "/", nil)
	raw := base64.StdEncoding.EncodeToString([]byte("ton618:ton618"))
	urlEncoded := url.QueryEscape(raw)
	req.AddCookie(&http.Cookie{Name: "ton_auth", Value: urlEncoded})

	if !checkCredentials(req, "ton618") {
		t.Error("cookie URL-encoded deveria retornar true")
	}
}

func TestCheckCredentials_CookieComPrefixoBasic(t *testing.T) {
	// Cookie no formato legado: "Basic " + base64 (sem URL-encode)
	req := httptest.NewRequest("GET", "/", nil)
	raw := base64.StdEncoding.EncodeToString([]byte("ton618:ton618"))
	req.AddCookie(&http.Cookie{Name: "ton_auth", Value: "Basic " + raw})

	if !checkCredentials(req, "ton618") {
		t.Error("cookie com prefixo 'Basic ' legado deveria retornar true")
	}
}

func TestCheckCredentials_CookieComPrefixoBasicUrlEncoded(t *testing.T) {
	// Cookie legado com URL-encoding: "Basic%20" + base64
	req := httptest.NewRequest("GET", "/", nil)
	raw := base64.StdEncoding.EncodeToString([]byte("ton618:ton618"))
	urlEncoded := strings.ReplaceAll("Basic "+raw, " ", "%20")
	req.AddCookie(&http.Cookie{Name: "ton_auth", Value: urlEncoded})

	if !checkCredentials(req, "ton618") {
		t.Error("cookie com prefixo 'Basic%20' URL-encoded deveria retornar true")
	}
}

func TestCheckCredentials_CookieSenhaErrada(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	raw := base64.StdEncoding.EncodeToString([]byte("ton618:senha-errada"))
	req.AddCookie(&http.Cookie{Name: "ton_auth", Value: raw})

	if checkCredentials(req, "ton618") {
		t.Error("cookie com senha errada deveria retornar false")
	}
}

func TestCheckCredentials_CookieInvalido(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "ton_auth", Value: "nao-e-base64!!!@@@"})

	if checkCredentials(req, "ton618") {
		t.Error("cookie com valor invalido deveria retornar false")
	}
}

func TestCheckCredentials_SenhaVazia_RetornaFalse(t *testing.T) {
	// Se pass está vazio, checkCredentials deve retornar false mesmo com header
	req := httptest.NewRequest("GET", "/", nil)
	req.SetBasicAuth("ton618", "ton618")

	if checkCredentials(req, "") {
		t.Error("com pass vazio, checkCredentials deveria retornar false")
	}
}

// ── BasicAuthMiddleware ─────────────────────────────────────────

func checkMiddlewareResult(t *testing.T, req *http.Request, user, pass string) int {
	t.Helper()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := BasicAuthMiddleware(next, user, pass)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec.Code
}

func TestMiddleware_RotaPublica_Health(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/health", nil)
	code := checkMiddlewareResult(t, req, "admin", "ton618")
	if code != http.StatusOK {
		t.Errorf("/api/health publico deveria retornar 200, got %d", code)
	}
}

func TestMiddleware_RotaPublica_Login(t *testing.T) {
	req := httptest.NewRequest("GET", "/login", nil)
	code := checkMiddlewareResult(t, req, "admin", "ton618")
	if code != http.StatusOK {
		t.Errorf("/login publico deveria retornar 200, got %d", code)
	}
}

func TestMiddleware_RotaPublica_Static(t *testing.T) {
	req := httptest.NewRequest("GET", "/static/editor.js", nil)
	code := checkMiddlewareResult(t, req, "admin", "ton618")
	if code != http.StatusOK {
		t.Errorf("/static/ publico deveria retornar 200, got %d", code)
	}
}

func TestMiddleware_RotaProtegida_SemAuth_Redireciona(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := BasicAuthMiddleware(next, "admin", "ton618")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("sem auth deveria redirecionar (302), got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc != "/login" {
		t.Errorf("redirect deveria ser para /login, got %s", loc)
	}
}

func TestMiddleware_RotaProtegida_ComAuthHeader_Permite(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.SetBasicAuth("ton618", "ton618")
	code := checkMiddlewareResult(t, req, "admin", "ton618")
	if code != http.StatusOK {
		t.Errorf("com senha valida deveria retornar 200, got %d", code)
	}
}

func TestMiddleware_RotaProtegida_ComCookie_Permite(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	raw := base64.StdEncoding.EncodeToString([]byte("ton618:ton618"))
	req.AddCookie(&http.Cookie{Name: "ton_auth", Value: raw})
	code := checkMiddlewareResult(t, req, "admin", "ton618")
	if code != http.StatusOK {
		t.Errorf("com cookie valido deveria retornar 200, got %d", code)
	}
}

func TestMiddleware_ApiStatus_SemAuth_Retorna401(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/status", nil)
	code := checkMiddlewareResult(t, req, "admin", "ton618")
	if code != http.StatusUnauthorized {
		t.Errorf("/api/status sem auth deveria retornar 401, got %d", code)
	}
}

func TestMiddleware_ApiStatus_ComAuth_Permite(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/status", nil)
	req.SetBasicAuth("ton618", "ton618")
	code := checkMiddlewareResult(t, req, "admin", "ton618")
	if code != http.StatusOK {
		t.Errorf("/api/status com senha valida deveria retornar 200, got %d", code)
	}
}

func TestMiddleware_UserPassVazio_SkipAuth(t *testing.T) {
	// Se user e pass sao vazios, middleware permite tudo sem auth
	req := httptest.NewRequest("GET", "/", nil)
	code := checkMiddlewareResult(t, req, "", "")
	if code != http.StatusOK {
		t.Errorf("user/pass vazio deveria pular auth e retornar 200, got %d", code)
	}
}

func TestMiddleware_POST_SemAuth_Retorna401(t *testing.T) {
	// POST sem auth sempre retorna 401 (nao redireciona)
	req := httptest.NewRequest("POST", "/file/save", nil)
	code := checkMiddlewareResult(t, req, "admin", "ton618")
	if code != http.StatusUnauthorized {
		t.Errorf("POST sem auth deveria retornar 401, got %d", code)
	}
}

func TestMiddleware_POST_ComAuth_Permite(t *testing.T) {
	req := httptest.NewRequest("POST", "/file/save", nil)
	req.SetBasicAuth("ton618", "ton618")
	code := checkMiddlewareResult(t, req, "admin", "ton618")
	if code != http.StatusOK {
		t.Errorf("POST com senha valida deveria retornar 200, got %d", code)
	}
}

func TestMiddleware_MultiplasFontes_Prioridade(t *testing.T) {
	// Cookie com senha invalida + header valido → deve permitir (header tem prioridade)
	req := httptest.NewRequest("GET", "/", nil)
	req.SetBasicAuth("ton618", "ton618")

	wrongRaw := base64.StdEncoding.EncodeToString([]byte("ton618:senha-errada"))
	req.AddCookie(&http.Cookie{Name: "ton_auth", Value: wrongRaw})

	code := checkMiddlewareResult(t, req, "admin", "ton618")
	if code != http.StatusOK {
		t.Errorf("header valido deve prevalecer sobre cookie invalido, got %d", code)
	}
}

func TestMiddleware_CookieSobrepoeHeaderVazio(t *testing.T) {
	// Sem header mas com cookie valido
	req := httptest.NewRequest("GET", "/", nil)
	raw := base64.StdEncoding.EncodeToString([]byte("ton618:ton618"))
	req.AddCookie(&http.Cookie{Name: "ton_auth", Value: raw})

	code := checkMiddlewareResult(t, req, "admin", "ton618")
	if code != http.StatusOK {
		t.Errorf("cookie valido sem header deveria permitir acesso, got %d", code)
	}
}

// ── Recovery ────────────────────────────────────────────────────────

func TestRecovery_Panics(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	handler := Recovery(next)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("esperado 500 apos panic, got %d", rec.Code)
	}
	if rec.Body.String() != "Internal Server Error\n" {
		t.Errorf("esperado 'Internal Server Error', got %q", rec.Body.String())
	}
}

func TestRecovery_NoPanic(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	handler := Recovery(next)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("esperado 200 sem panic, got %d", rec.Code)
	}
	if rec.Body.String() != "OK" {
		t.Errorf("esperado 'OK', got %q", rec.Body.String())
	}
}
