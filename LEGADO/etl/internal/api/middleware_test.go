package api

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBasicAuthMiddleware_PublicPaths(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	auth := BasicAuthMiddleware(handler, "admin", "secret")

	tests := []struct {
		name string
		path string
	}{
		{"root", "/"},
		{"SPA route", "/notes/some-file"},
		{"static file", "/assets/main.js"},
		{"avatar", "/api/tags/avatar"},
		{"file assets", "/api/file?name=assets/image.png"},
		{"file attachments", "/api/file?name=attachments/doc.pdf"},
		{"file pdfs", "/api/file?name=pdfs/report.pdf"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rec := httptest.NewRecorder()
			auth.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("path %q should be public, got status %d", tt.path, rec.Code)
			}
		})
	}
}

func TestBasicAuthMiddleware_APIPathsRequireAuth(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	auth := BasicAuthMiddleware(handler, "admin", "secret")

	req := httptest.NewRequest("GET", "/api/search", nil)
	rec := httptest.NewRecorder()
	auth.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("API path without auth should return 401, got %d", rec.Code)
	}
	if rec.Body.String() == "" {
		t.Error("401 response body should not be empty")
	}
}

func TestBasicAuthMiddleware_ValidBasicAuth(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	auth := BasicAuthMiddleware(handler, "admin", "ton618")

	req := httptest.NewRequest("GET", "/api/status", nil)
	req.SetBasicAuth("admin", "ton618")
	rec := httptest.NewRecorder()
	auth.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("valid credentials should pass, got status %d", rec.Code)
	}
}

func TestBasicAuthMiddleware_InvalidPassword(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	auth := BasicAuthMiddleware(handler, "admin", "correct")

	req := httptest.NewRequest("GET", "/api/status", nil)
	req.SetBasicAuth("admin", "wrong")
	rec := httptest.NewRecorder()
	auth.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong password should return 401, got %d", rec.Code)
	}
}

func TestBasicAuthMiddleware_InvalidUser(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	auth := BasicAuthMiddleware(handler, "admin", "secret")

	req := httptest.NewRequest("GET", "/api/status", nil)
	req.SetBasicAuth("hacker", "secret")
	rec := httptest.NewRecorder()
	auth.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong user should return 401, got %d", rec.Code)
	}
}

func TestBasicAuthMiddleware_TokenAuth(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	auth := BasicAuthMiddleware(handler, "admin", "ton618")

	// Token válido
	token := base64.StdEncoding.EncodeToString([]byte("admin:ton618"))
	req := httptest.NewRequest("GET", "/api/file?name=notes/test.md&token="+token, nil)
	rec := httptest.NewRecorder()
	auth.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("valid token should pass, got status %d", rec.Code)
	}
}

func TestBasicAuthMiddleware_TokenWithBasicPrefix(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	auth := BasicAuthMiddleware(handler, "admin", "ton618")

	token := base64.StdEncoding.EncodeToString([]byte("admin:ton618"))
	req := httptest.NewRequest("GET", "/api/file?name=notes/test.md&token=Basic%20"+token, nil)
	rec := httptest.NewRecorder()
	auth.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("valid token with Basic prefix should pass, got status %d", rec.Code)
	}
}

func TestBasicAuthMiddleware_InvalidToken(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	auth := BasicAuthMiddleware(handler, "admin", "ton618")

	req := httptest.NewRequest("GET", "/api/file?token=invalid123", nil)
	rec := httptest.NewRecorder()
	auth.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("invalid token should return 401, got %d", rec.Code)
	}
}

func TestBasicAuthMiddleware_NoAuthHeader(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	auth := BasicAuthMiddleware(handler, "admin", "secret")

	req := httptest.NewRequest("POST", "/api/search", nil)
	rec := httptest.NewRecorder()
	auth.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no auth header should return 401, got %d", rec.Code)
	}
}

func TestBasicAuthMiddleware_DocsPath(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	auth := BasicAuthMiddleware(handler, "admin", "secret")

	// /docs/ paths should require auth
	req := httptest.NewRequest("GET", "/docs/readme.md", nil)
	rec := httptest.NewRecorder()
	auth.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("/docs/ paths should require auth, got status %d", rec.Code)
	}
}
