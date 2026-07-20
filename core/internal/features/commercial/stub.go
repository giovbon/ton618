//go:build !commercial

// Package commercial contém feature flags para a versão paga.
// Build sem a tag → todas falsas.
package commercial

const (
	MultiUserEnabled  = false
	CloudSyncEnabled  = false
	DesktopEnabled    = false
	MobileEnabled     = false
	WhiteLabelEnabled = false
)
