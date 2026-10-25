package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"ton618/core/internal/core/config"
	"ton618/core/internal/core/db"
	tonmcp "ton618/core/internal/mcp"
)

// runMCP implementa o subcomando `ton618 mcp`: um servidor MCP local via stdio,
// SOMENTE LEITURA, que dá a qualquer cliente de IA acesso às notas.
//
//	stdout é exclusivo do protocolo JSON-RPC — por isso logs e erros vão para
//	stderr. Nenhuma migração, indexação ou criação de diretório é executada.
func runMCP(args []string) int {
	fs := flag.NewFlagSet("ton618 mcp", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dbFlag := fs.String("db", "", "caminho do banco SQLite (padrão: DB_PATH, ./data/ton618.db ou <binário>/data/ton618.db)")
	docsFlag := fs.String("docs", "", "diretório do acervo (DOCS_DIR); usado só como contexto do NoteService")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Uso: ton618 mcp [--db CAMINHO] [--docs DIR]")
		fmt.Fprintln(os.Stderr, "\nServidor MCP local (stdio) de leitura das notas do TON-618.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "ton618 mcp: argumentos inesperados: %v\n", fs.Args())
		fs.Usage()
		return 2
	}

	// stdout é o canal do protocolo: qualquer log no stdout corromperia o JSON-RPC.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.Load()
	dbPath, err := resolveMCPDBPath(*dbFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ton618 mcp: %v\n", err)
		return 1
	}
	store, err := db.NewReaderStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ton618 mcp: %v\n", err)
		return 1
	}
	defer store.Close()

	docsDir := resolveMCPDocsDir(*docsFlag, cfg)
	slog.Info("MCP read-only pronto", "db", dbPath, "docs", docsDir)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := tonmcp.NewServer(store, docsDir).Serve(ctx); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "ton618 mcp: %v\n", err)
		return 1
	}
	return 0
}

// resolveMCPDBPath aplica a precedência flag > DB_PATH > ./data > <binário>/data.
func resolveMCPDBPath(flagValue string) (string, error) {
	if flagValue != "" {
		path, err := filepath.Abs(flagValue)
		if err != nil {
			return "", fmt.Errorf("--db inválido: %w", err)
		}
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("--db %q: %w", path, err)
		}
		return path, nil
	}

	type candidate struct {
		origem string
		path   string
	}
	var candidates []candidate
	if env := os.Getenv("DB_PATH"); env != "" {
		candidates = append(candidates, candidate{"DB_PATH", env})
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, candidate{"diretório atual", filepath.Join(cwd, "data", "ton618.db")})
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			candidate{"diretório do binário", filepath.Join(dir, "data", "ton618.db")},
			candidate{"diretório do binário (../data)", filepath.Join(dir, "..", "data", "ton618.db")},
		)
	}

	for _, c := range candidates {
		abs, err := filepath.Abs(c.path)
		if err != nil {
			continue
		}
		if info, err := os.Stat(abs); err == nil && !info.IsDir() {
			return abs, nil
		}
	}

	msg := "banco SQLite não encontrado. Procurado em:"
	for _, c := range candidates {
		msg += fmt.Sprintf("\n  - %s (%s)", c.path, c.origem)
	}
	msg += "\nPasse o caminho explicitamente com --db CAMINHO (ou defina DB_PATH)."
	return "", errors.New(msg)
}

// resolveMCPDocsDir aplica flag > DOCS_DIR > <binário>/docs > cwd/docs. O diretório
// não precisa existir: os caminhos de leitura das tools não tocam o disco.
func resolveMCPDocsDir(flagValue string, cfg *config.AppConfig) string {
	if flagValue != "" {
		if abs, err := filepath.Abs(flagValue); err == nil {
			return abs
		}
		return flagValue
	}
	if env := os.Getenv("DOCS_DIR"); env != "" {
		if abs, err := filepath.Abs(env); err == nil {
			return abs
		}
		return env
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, candidate := range []string{filepath.Join(dir, "docs"), filepath.Join(dir, "..", "docs")} {
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return candidate
			}
		}
	}
	return cfg.DocsDir
}
