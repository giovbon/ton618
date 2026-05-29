package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetEnv_UsaVariavel(t *testing.T) {
	os.Setenv("TEST_MY_KEY", "valor")
	defer os.Unsetenv("TEST_MY_KEY")

	got := getEnv("TEST_MY_KEY", "fallback")
	if got != "valor" {
		t.Fatalf("esperado 'valor', got %q", got)
	}
}

func TestGetEnv_Fallback(t *testing.T) {
	got := getEnv("TEST_NAO_EXISTE_12345", "fallback_padrao")
	if got != "fallback_padrao" {
		t.Fatalf("esperado 'fallback_padrao', got %q", got)
	}
}

func TestGetEnv_VariavelVazia_RetornaVazio(t *testing.T) {
	os.Setenv("TEST_EMPTY", "")
	defer os.Unsetenv("TEST_EMPTY")

	// getEnv retorna o valor real (string vazia) quando a var existe
	got := getEnv("TEST_EMPTY", "fallback")
	if got != "" {
		t.Fatalf("esperado '' para variavel vazia, got %q", got)
	}
}

func TestGetEnvAsInt_Valido(t *testing.T) {
	os.Setenv("TEST_INT", "42")
	defer os.Unsetenv("TEST_INT")

	got := getEnvAsInt("TEST_INT", 0)
	if got != 42 {
		t.Fatalf("esperado 42, got %d", got)
	}
}

func TestGetEnvAsInt_Fallback(t *testing.T) {
	got := getEnvAsInt("TEST_INT_INEXISTENTE", 99)
	if got != 99 {
		t.Fatalf("esperado 99, got %d", got)
	}
}

func TestGetEnvAsInt_Invalido(t *testing.T) {
	os.Setenv("TEST_INT_INVALIDO", "nao_eh_numero")
	defer os.Unsetenv("TEST_INT_INVALIDO")

	got := getEnvAsInt("TEST_INT_INVALIDO", 77)
	if got != 77 {
		t.Fatalf("esperado 77 (fallback para valor invalido), got %d", got)
	}
}

func TestGetEnvAsBool_True(t *testing.T) {
	for _, v := range []string{"true", "1", "yes"} {
		os.Setenv("TEST_BOOL", v)
		got := getEnvAsBool("TEST_BOOL", false)
		if got != true {
			t.Fatalf("esperado true para %q, got false", v)
		}
		os.Unsetenv("TEST_BOOL")
	}
}

func TestGetEnvAsBool_False(t *testing.T) {
	for _, v := range []string{"false", "0", "no", "qualquer", ""} {
		os.Setenv("TEST_BOOL", v)
		got := getEnvAsBool("TEST_BOOL", true)
		if got != false {
			t.Fatalf("esperado false para %q, got true", v)
		}
		os.Unsetenv("TEST_BOOL")
	}
}

func TestGetEnvAsBool_Fallback(t *testing.T) {
	got := getEnvAsBool("TEST_BOOL_INEXISTENTE", true)
	if got != true {
		t.Fatalf("esperado true (fallback), got false")
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Garante que nenhuma variavel polua o teste
	for _, k := range []string{"DOCS_DIR", "DB_PATH", "PORT", "WEB_DIR", "STATE_DIR",
		"AUTH_USER", "AUTH_PASS", "EMBEDDING_PROVIDER", "EMBEDDING_DIM", "EMBEDDING_ALL"} {
		os.Unsetenv(k)
	}

	cfg := Load()

	if cfg.Port != "6180" {
		t.Fatalf("esperado porta 6180, got %q", cfg.Port)
	}
	if cfg.AuthUser != "admin" {
		t.Fatalf("esperado auth user 'admin', got %q", cfg.AuthUser)
	}
	if cfg.AuthPass != "ton618" {
		t.Fatalf("esperado auth pass 'ton618', got %q", cfg.AuthPass)
	}
	if cfg.PollIntervalSec != 30*time.Second {
		t.Fatalf("esperado poll interval 30s, got %v", cfg.PollIntervalSec)
	}
}

func TestLoad_ResolveCaminhosAbsolutos(t *testing.T) {
	os.Setenv("DOCS_DIR", "./test_docs")
	os.Setenv("DB_PATH", "./test_data/test.db")
	os.Setenv("WEB_DIR", "./test_web")
	os.Setenv("STATE_DIR", "./test_state")
	defer func() {
		os.Unsetenv("DOCS_DIR")
		os.Unsetenv("DB_PATH")
		os.Unsetenv("WEB_DIR")
		os.Unsetenv("STATE_DIR")
	}()

	cfg := Load()

	// Todos devem ser resolvidos para caminhos absolutos
	if !filepath.IsAbs(cfg.DocsDir) {
		t.Fatalf("DocsDir deveria ser absoluto, got %q", cfg.DocsDir)
	}
	if !filepath.IsAbs(cfg.DBPath) {
		t.Fatalf("DBPath deveria ser absoluto, got %q", cfg.DBPath)
	}
	if !filepath.IsAbs(cfg.WebDir) {
		t.Fatalf("WebDir deveria ser absoluto, got %q", cfg.WebDir)
	}
	if !filepath.IsAbs(cfg.StateDir) {
		t.Fatalf("StateDir deveria ser absoluto, got %q", cfg.StateDir)
	}
}

func TestLoad_LeeVariaveis(t *testing.T) {
	os.Setenv("PORT", "9999")
	os.Setenv("AUTH_USER", "testuser")
	os.Setenv("AUTH_PASS", "testpass")
	os.Setenv("POLL_INTERVAL_SEC", "60")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("AUTH_USER")
		os.Unsetenv("AUTH_PASS")
		os.Unsetenv("POLL_INTERVAL_SEC")
	}()

	cfg := Load()

	if cfg.Port != "9999" {
		t.Fatalf("esperado porta 9999, got %q", cfg.Port)
	}
	if cfg.AuthUser != "testuser" {
		t.Fatalf("esperado 'testuser', got %q", cfg.AuthUser)
	}
	if cfg.AuthPass != "testpass" {
		t.Fatalf("esperado 'testpass', got %q", cfg.AuthPass)
	}
	if cfg.PollIntervalSec != 60*time.Second {
		t.Fatalf("esperado 60s, got %v", cfg.PollIntervalSec)
	}
}

func TestEnsureDirs_CriaDiretorios(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &AppConfig{
		DocsDir:  filepath.Join(tmpDir, "docs"),
		StateDir: filepath.Join(tmpDir, "state"),
	}

	err := cfg.EnsureDirs()
	if err != nil {
		t.Fatalf("EnsureDirs retornou erro: %v", err)
	}

	// Verifica se os diretorios principais existem
	for _, d := range []string{cfg.DocsDir, cfg.StateDir} {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			t.Fatalf("diretorio %q nao foi criado", d)
		}
	}

	// Verifica subdiretorios monitorados
	for _, sub := range []string{"notes", "links", "voice", "pdfs"} {
		path := filepath.Join(cfg.DocsDir, sub)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Fatalf("subdiretorio %q nao foi criado", path)
		}
	}
}

func TestEnsureDirs_CaminhoInvalido_RetornaErro(t *testing.T) {
	cfg := &AppConfig{
		DocsDir:  "/proc/nao_posso_escrever_aqui",
		StateDir: "/proc/tambem_nao",
	}

	err := cfg.EnsureDirs()
	if err == nil {
		t.Fatal("esperado erro para diretorio protegido, got nil")
	}
}
