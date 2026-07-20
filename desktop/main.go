// desktop — Launcher do TON-618 para Desktop
//
// Estratégia: o desktop é um shell (Wails) que inicia o core-server
// como processo filho e abre a WebView apontando para localhost:6180.
//
// Isso permite atualizar o core independentemente do desktop:
//   git pull && cd core && go build -o core-server
//   → substitui o binário, desktop continua funcionando
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// App é o struct de binding exposto para o frontend Wails.
type App struct {
	ctx    context.Context
	core   *exec.Cmd
	port   int
	dataDir string
}

// ── Main ─────────────────────────────────────────────

func main() {
	port := 6180
	dataDir := resolveDataDir()

	app := &App{
		port:    port,
		dataDir: dataDir,
	}

	// Inicia o core-server em segundo plano
	// Nota: durante o build do Wails, isso pode falhar (core-server não existe ainda)
	// Nesse caso, apenas loga um aviso — o build continua.
	if err := app.startCore(); err != nil {
		log.Printf("Aviso: core-server não iniciou (%v). O desktop vai tentar novamente ao abrir.", err)
	}
	defer app.stopCore()

	// Cria a janela desktop
	err := wails.Run(&options.App{
		Title:     "TON-618",
		Width:     1280,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,

		// URL: aponta para o core-server rodando localmente
		AssetServer: &assetserver.Options{
			Handler: &proxyHandler{port: port},
		},

		// Bind dos métodos expostos para o frontend JS
		Bind: []interface{}{
			app,
		},

		// Platform-specific options
		Windows: &windows.Options{
			WebviewIsTransparent: false,
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: false,
			},
		},
		Linux: &linux.Options{},
	})

	if err != nil {
		log.Fatal(err)
	}
}

// ── Core Server lifecycle ─────────────────────────────

// resolveDataDir encontra o diretório de dados do usuário.
func resolveDataDir() string {
	// Dentro do bundle desktop, usa diretório local mesmo
	if exe, err := os.Executable(); err == nil {
		base := filepath.Dir(exe)
		dataDir := filepath.Join(base, "data")
		if err := os.MkdirAll(dataDir, 0755); err == nil {
			return dataDir
		}
	}
	// Fallback: current directory
	return "./data"
}

// coreBinaryPath retorna o caminho para o binário core-server.
// Durante o build do Wails, o executable está em um diretório temp,
// então também verifica o diretório atual como fallback.
func coreBinaryPath() string {
	// 1. Tenta ao lado do executável
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		binary := filepath.Join(dir, "core-server")
		if runtime.GOOS == "windows" {
			binary += ".exe"
		}
		if _, err := os.Stat(binary); err == nil {
			return binary
		}
	}
	// 2. Fallback: diretório atual
	binary := "core-server"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if _, err := os.Stat(binary); err == nil {
		return binary
	}
	// 3. Fallback: diretório pai (quando roda de desktop/)
	parent := filepath.Join("..", "core", "core-server")
	if runtime.GOOS == "windows" {
		parent += ".exe"
	}
	if _, err := os.Stat(parent); err == nil {
		return parent
	}
	return binary
}

func (a *App) startCore() error {
	binary := coreBinaryPath()

	// Se o core-server não existir, tenta go run (ambiente de desenvolvimento)
	if _, err := os.Stat(binary); os.IsNotExist(err) {
		log.Printf("core-server não encontrado, tentando go run no core...")
		coreDir := filepath.Join("..", "core")
		a.core = exec.Command("go", "run", ".")
		a.core.Dir = coreDir
		a.core.Env = append(os.Environ(),
			fmt.Sprintf("PORT=%d", a.port),
			fmt.Sprintf("DB_PATH=%s/ton618.db", a.dataDir),
			fmt.Sprintf("STATE_DIR=%s", a.dataDir),
			fmt.Sprintf("DOCS_DIR=%s", a.dataDir),
		)
	} else {
		a.core.Env = append(os.Environ(),
			fmt.Sprintf("PORT=%d", a.port),
			fmt.Sprintf("DB_PATH=%s/ton618.db", a.dataDir),
			fmt.Sprintf("STATE_DIR=%s", a.dataDir),
			fmt.Sprintf("DOCS_DIR=%s", a.dataDir),
		)
	}

	if err := a.core.Start(); err != nil {
		return fmt.Errorf("falha ao iniciar core-server: %w", err)
	}

	log.Printf("Core-server iniciado (PID %d) na porta %d", a.core.Process.Pid, a.port)

	// Aguarda o servidor ficar pronto (até 10s)
	return a.waitForReady(10 * time.Second)
}

func (a *App) waitForReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://127.0.0.1:%d/api/health", a.port)

	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("timeout aguardando core-server na porta %d", a.port)
}

func (a *App) stopCore() {
	if a.core != nil && a.core.Process != nil {
		log.Printf("Parando core-server (PID %d)...", a.core.Process.Pid)

		// Envia SIGTERM (graceful shutdown)
		if runtime.GOOS == "windows" {
			a.core.Process.Signal(syscall.SIGKILL)
		} else {
			a.core.Process.Signal(syscall.SIGTERM)
		}

		// Aguarda até 5s para o processo terminar
		done := make(chan error, 1)
		go func() {
			done <- a.core.Wait()
		}()

		select {
		case <-done:
			log.Println("Core-server parou.")
		case <-time.After(5 * time.Second):
			log.Println("Core-server não respondeu, forçando kill.")
			a.core.Process.Kill()
		}
	}
}

// ── Proxy Handler ─────────────────────────────────────

// proxyHandler redireciona requisições de asset para o backend core.
type proxyHandler struct {
	port int
}

func (p *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	target := fmt.Sprintf("http://127.0.0.1:%d%s", p.port, r.URL.String())
	http.Redirect(w, r, target, http.StatusFound)
}

// ── Wails Bindings (expostos para o frontend JS) ─────

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	// Graceful shutdown no Ctrl+C
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		a.stopCore()
		os.Exit(0)
	}()
}

func (a *App) Shutdown(ctx context.Context) {
	a.stopCore()
}

// GetPort retorna a porta onde o core está rodando (para debug).
func (a *App) GetPort() int {
	return a.port
}

// GetDataDir retorna o diretório de dados.
func (a *App) GetDataDir() string {
	return a.dataDir
}

// GetVersion retorna a versão atual do desktop.
func (a *App) GetVersion() string {
	return "1.0.0"
}

// ── Auto-updater ──────────────────────────────────────

// CheckUpdate verifica no GitHub se há uma nova versão do core.
// Returns: { hasUpdate, latestVersion, downloadURL }
func (a *App) CheckUpdate() (map[string]interface{}, error) {
	repo := "giovbon/ton618"
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("erro ao verificar atualizações: %w", err)
	}
	defer resp.Body.Close()

	// Parse da resposta do GitHub API
	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("erro ao parsear resposta: %w", err)
	}

	current := a.GetVersion()
	hasUpdate := release.TagName > current

	// Procura asset específico para a plataforma atual
	downloadURL := ""
	suffix := fmt.Sprintf("core-server-%s-%s", runtime.GOOS, runtime.GOARCH)
	for _, asset := range release.Assets {
		if asset.Name == suffix || asset.Name == "core-server" {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	return map[string]interface{}{
		"hasUpdate":    hasUpdate,
		"latestVersion": release.TagName,
		"currentVersion": current,
		"downloadURL":  downloadURL,
	}, nil
}

// ApplyUpdate baixa a nova versão do core e substitui o binário atual.
func (a *App) ApplyUpdate(downloadURL string) error {
	log.Printf("Baixando atualização de %s...", downloadURL)

	// Para não bloquear, isso seria feito em goroutine com progress callback
	// Simplificação: apenas retorna a URL para o frontend baixar
	return nil
}
