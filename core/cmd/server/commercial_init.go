//go:build commercial

package main

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"ton618/commercial"
	"ton618/core/internal/middleware"
)

// init é chamado na inicialização do servidor em build commercial.
// Configura multi-tenant e valida licença.
func init() {
	// 1. Valida licença
	pubKeyPath := os.Getenv("TON_LICENSE_PUB_KEY")
	licPath := os.Getenv("TON_LICENSE_FILE")

	if pubKeyPath == "" {
		pubKeyPath = "license.pub"
	}
	if licPath == "" {
		licPath = "license.lic"
	}

	if err := commercial.Init(pubKeyPath, licPath); err != nil {
		slog.Error("commercial: falha na validação da licença", "error", err)
		os.Exit(1)
	}

	slog.Info("commercial: modo SaaS ativado",
		"plan", commercial.Plan(),
		"licensed", commercial.IsLicensed(),
	)
}

// setupMultiTenant configura o middleware de tenants.
// Deve ser chamado na montagem das rotas.
func setupMultiTenant(router *http.ServeMux, tenantsDir string) *middleware.TenantManager {
	manager := middleware.NewTenantManager(tenantsDir)

	slog.Info("commercial: multi-tenant ativo", "tenantsDir", tenantsDir)

	return manager
}

// tenantDataDir retorna o diretório onde os bancos dos tenants ficam.
func tenantDataDir(baseDir string) string {
	return filepath.Join(baseDir, "tenants")
}
