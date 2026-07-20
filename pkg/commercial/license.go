// Package commercial contém funcionalidades da versão comercial do TON-618.
// Este módulo é separado do core e só é compilado com a build tag `commercial`.
//
// Uso no core/cmd/server/main.go:
//
//	import _ "ton618/commercial"
//
// Build:
//
//	go build -tags commercial -o ton618-pro ./cmd/server
package commercial

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

var (
	mu         sync.RWMutex
	verifyKey  *rsa.PublicKey
	license    *LicenseInfo
	licensed   bool
)

// Planos disponíveis
const (
	PlanPersonal   = "personal"
	PlanTeam       = "team"
	PlanEnterprise = "enterprise"
)

// Features disponíveis
const (
	FeatureMultiUser    = "multi_user"
	FeatureCloudSync    = "cloud_sync"
	FeatureShareNotes   = "share_notes"
	FeaturePublicAPI    = "public_api"
	FeatureDesktop      = "desktop_app"
	FeatureMobile       = "mobile_app"
	FeatureTeam         = "team_collab"
	FeatureAuditLog     = "audit_log"
	FeatureWhiteLabel   = "white_label"
	FeaturePriority     = "priority_support"
)

// FeatureTiers mapeia cada plano às suas features.
var FeatureTiers = map[string][]string{
	PlanPersonal: {
		FeatureDesktop,
		FeatureCloudSync,
	},
	PlanTeam: {
		FeatureDesktop,
		FeatureCloudSync,
		FeatureMultiUser,
		FeatureTeam,
		FeatureShareNotes,
		FeaturePublicAPI,
	},
	PlanEnterprise: {
		FeatureDesktop,
		FeatureMobile,
		FeatureCloudSync,
		FeatureMultiUser,
		FeatureTeam,
		FeatureShareNotes,
		FeaturePublicAPI,
		FeatureAuditLog,
		FeatureWhiteLabel,
		FeaturePriority,
	},
}

// LicenseInfo armazena os dados de uma licença validada.
type LicenseInfo struct {
	User      string   `json:"user"`
	Email     string   `json:"email"`
	Plan      string   `json:"plan"`
	ExpiresAt int64    `json:"expires_at"` // Unix timestamp
	Features  []string `json:"features"`
	Tenants   int      `json:"tenants"` // Máximo de tenants permitido (0 = ilimitado)
}

// signedLicense é a estrutura completa do arquivo de licença assinado.
type signedLicense struct {
	Info      LicenseInfo `json:"info"`
	Signature []byte      `json:"signature"`
}

// Init carrega a chave pública e valida a licença.
// Deve ser chamada na inicialização do servidor.
// Se os arquivos não existirem, roda sem licença (modo core-only).
func Init(publicKeyPath, licensePath string) error {
	keyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		slog.Warn("commercial: chave pública não encontrada, rodando sem licenciamento", "path", publicKeyPath)
		return nil
	}

	verifyKey, err = parsePublicKey(keyData)
	if err != nil {
		return fmt.Errorf("commercial: erro ao parsear chave pública: %w", err)
	}

	licData, err := os.ReadFile(licensePath)
	if err != nil {
		slog.Warn("commercial: licença não encontrada, rodando sem features comerciais", "path", licensePath)
		return nil
	}

	lic, err := validateLicense(licData, verifyKey)
	if err != nil {
		return fmt.Errorf("commercial: licença inválida: %w", err)
	}

	mu.Lock()
	license = lic
	licensed = true
	mu.Unlock()

	slog.Info("commercial: licença validada",
		"user", lic.User,
		"plan", lic.Plan,
		"expires", time.Unix(lic.ExpiresAt, 0).Format(time.RFC3339),
	)

	return nil
}

// IsLicensed retorna true se uma licença válida foi carregada e não expirou.
func IsLicensed() bool {
	mu.RLock()
	defer mu.RUnlock()
	if !licensed || license == nil {
		return false
	}
	return time.Now().Unix() < license.ExpiresAt
}

// Plan retorna o nome do plano atual.
func Plan() string {
	mu.RLock()
	defer mu.RUnlock()
	if license == nil {
		return ""
	}
	return license.Plan
}

// MaxTenants retorna o número máximo de tenants permitido.
// 0 = ilimitado.
func MaxTenants() int {
	mu.RLock()
	defer mu.RUnlock()
	if license == nil {
		return 0
	}
	return license.Tenants
}

// HasFeature verifica se a licença inclui uma feature específica.
func HasFeature(feature string) bool {
	if !IsLicensed() {
		return false
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, f := range license.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// CurrentLicense retorna as informações da licença atual.
func CurrentLicense() *LicenseInfo {
	mu.RLock()
	defer mu.RUnlock()
	return license
}

// GenerateLicense gera um arquivo de licença assinado (uso administrativo).
// Requer a chave privada RSA em formato PEM.
func GenerateLicense(privateKeyPath string, info LicenseInfo, outputPath string) error {
	keyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return fmt.Errorf("erro ao ler chave privada: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return fmt.Errorf("formato PEM inválido")
	}

	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("erro ao parsear chave privada: %w", err)
	}

	rsaPriv, ok := priv.(*rsa.PrivateKey)
	if !ok {
		return fmt.Errorf("chave não é RSA")
	}

	// Preenche features baseadas no plano
	if info.Features == nil {
		info.Features = FeatureTiers[info.Plan]
	}

	data, _ := json.Marshal(info)
	hash := sha256.Sum256(data)
	signature, err := rsa.SignPKCS1v15(rand.Reader, rsaPriv, crypto.SHA256, hash[:])
	if err != nil {
		return fmt.Errorf("erro ao assinar licença: %w", err)
	}

	lic := signedLicense{
		Info:      info,
		Signature: signature,
	}

	output, _ := json.MarshalIndent(lic, "", "  ")
	if err := os.WriteFile(outputPath, output, 0644); err != nil {
		return fmt.Errorf("erro ao escrever licença: %w", err)
	}

	return nil
}

// ── Funções internas ──

func parsePublicKey(data []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("formato PEM inválido")
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("erro ao parsear chave pública: %w", err)
	}

	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("chave não é RSA")
	}

	return rsaKey, nil
}

func validateLicense(data []byte, pub *rsa.PublicKey) (*LicenseInfo, error) {
	var lic signedLicense
	if err := json.Unmarshal(data, &lic); err != nil {
		return nil, fmt.Errorf("erro ao parsear licença: %w", err)
	}

	// Verifica assinatura
	infoData, _ := json.Marshal(lic.Info)
	hash := sha256.Sum256(infoData)
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, hash[:], lic.Signature); err != nil {
		return nil, fmt.Errorf("assinatura inválida: %w", err)
	}

	// Verifica expiração
	if time.Now().Unix() > lic.Info.ExpiresAt {
		return nil, fmt.Errorf("licença expirada em %s", time.Unix(lic.Info.ExpiresAt, 0).Format(time.RFC3339))
	}

	return &lic.Info, nil
}
