// Package middleware fornece middlewares HTTP, incluindo o TenantManager
// para operação multi-tenant (SaaS).
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"ton618/core/internal/core/db"
)

type tenantContextKey string

const (
	// TenantIDKey é a chave no context para obter o ID do tenant atual.
	TenantIDKey tenantContextKey = "tenant_id"
	// TenantStoreKey é a chave no context para obter o *db.Store do tenant.
	TenantStoreKey tenantContextKey = "tenant_store"
)

// TenantManager gerencia bancos SQLite por tenant (multi-tenant).
// Cada tenant tem seu próprio arquivo .db isolado.
type TenantManager struct {
	mu      sync.RWMutex
	stores  map[string]*db.Store
	dataDir string
}

// NewTenantManager cria um gerenciador de tenants.
// dataDir é o diretório raiz onde os bancos ficam (ex: /data/tenants/).
func NewTenantManager(dataDir string) *TenantManager {
	return &TenantManager{
		stores:  make(map[string]*db.Store),
		dataDir: dataDir,
	}
}

// getOrCreateStore retorna o store de um tenant, criando o banco se necessário.
func (tm *TenantManager) getOrCreateStore(tenantID string) (*db.Store, error) {
	tm.mu.RLock()
	store, ok := tm.stores[tenantID]
	tm.mu.RUnlock()
	if ok {
		return store, nil
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Double-check após adquirir lock
	if store, ok := tm.stores[tenantID]; ok {
		return store, nil
	}

	dbPath := filepath.Join(tm.dataDir, tenantID, "ton618.db")
	slog.Info("criando banco para tenant", "tenant", tenantID, "path", dbPath)

	store, err := db.NewStore(dbPath)
	if err != nil {
		return nil, err
	}

	tm.stores[tenantID] = store
	return store, nil
}

// Middleware extrai o tenant do subdomínio e injeta o store no context.
// Ex: "clienteA.ton618.io" → tenantID = "clienteA"
func (tm *TenantManager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := extractTenantFromHost(r.Host)
		if tenantID == "" {
			// Modo single-tenant (auto-hospedado) — passa sem tenant
			next.ServeHTTP(w, r)
			return
		}

		store, err := tm.getOrCreateStore(tenantID)
		if err != nil {
			slog.Error("erro ao abrir banco do tenant", "tenant", tenantID, "error", err)
			http.Error(w, "Erro interno", http.StatusInternalServerError)
			return
		}

		ctx := context.WithValue(r.Context(), TenantIDKey, tenantID)
		ctx = context.WithValue(ctx, TenantStoreKey, store)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetTenantStore extrai o *db.Store do context.
// Retorna nil se não estiver em modo multi-tenant.
func GetTenantStore(ctx context.Context) *db.Store {
	store, _ := ctx.Value(TenantStoreKey).(*db.Store)
	return store
}

// GetTenantID extrai o ID do tenant do context.
// Retorna vazio se não estiver em modo multi-tenant.
func GetTenantID(ctx context.Context) string {
	id, _ := ctx.Value(TenantIDKey).(string)
	return id
}

// extractTenantFromHost extrai o subdomínio do host.
// Regras:
//   - "localhost" ou "127.0.0.1" → vazio (single-tenant)
//   - "ton618.io" → vazio (domínio principal)
//   - "cliente.ton618.io" → "cliente"
func extractTenantFromHost(host string) string {
	// Remove porta se houver
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	parts := strings.SplitN(host, ".", 3)
	if len(parts) < 3 {
		return "" // localhost ou domínio sem sub
	}

	// Pula se for www
	if parts[0] == "www" {
		return ""
	}

	return parts[0]
}

// Close fecha todos os stores dos tenants.
func (tm *TenantManager) Close() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	for id, store := range tm.stores {
		store.Close()
		delete(tm.stores, id)
	}
}
