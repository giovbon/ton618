//go:build commercial

package main

import (
	"log/slog"
	"net/http"
	"path/filepath"

	"ton618/core/internal/features/commercial"
	"ton618/core/internal/middleware"
)

func init() {
	slog.Info("commercial: build comercial ativado",
		"multi_user", commercial.MultiUserEnabled,
		"cloud_sync", commercial.CloudSyncEnabled,
	)
}

func setupMultiTenant(router *http.ServeMux, tenantsDir string) *middleware.TenantManager {
	manager := middleware.NewTenantManager(tenantsDir)
	slog.Info("commercial: multi-tenant ativo", "tenantsDir", tenantsDir)
	return manager
}

func tenantDataDir(baseDir string) string {
	return filepath.Join(baseDir, "tenants")
}
