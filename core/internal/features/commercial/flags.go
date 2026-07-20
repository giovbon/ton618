//go:build commercial

// Package commercial contém feature flags para a versão paga.
// Só é compilado com a build tag `commercial`.
package commercial

// Feature flags — true no build comercial
const (
	MultiUserEnabled  = true
	CloudSyncEnabled  = true
	DesktopEnabled    = true
	MobileEnabled     = false
	WhiteLabelEnabled = false
)
