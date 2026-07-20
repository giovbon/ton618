// Gera par de chaves RSA e licença de exemplo.
// Uso: go run pkg/commercial/cmd/genkey/main.go
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ton618/commercial"
)

func main() {
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	// 1. Gera chave privada RSA 4096
	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		panic(err)
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		panic(err)
	}

	privFile := filepath.Join(dir, "license.key")
	if err := os.WriteFile(privFile, pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes,
	}), 0600); err != nil {
		panic(err)
	}
	fmt.Println("✅ Chave privada:", privFile)

	// 2. Extrai chave pública
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		panic(err)
	}

	pubFile := filepath.Join(dir, "license.pub")
	if err := os.WriteFile(pubFile, pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	}), 0644); err != nil {
		panic(err)
	}
	fmt.Println("✅ Chave pública:", pubFile)

	// 3. Gera licença de exemplo (plano Enterprise, 1 ano)
	lic := commercial.LicenseInfo{
		User:      "Cliente Exemplo",
		Email:     "cliente@exemplo.com",
		Plan:      commercial.PlanEnterprise,
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour).Unix(),
		Tenants:   10,
	}

	licFile := filepath.Join(dir, "license.lic")
	if err := commercial.GenerateLicense(privFile, lic, licFile); err != nil {
		panic(err)
	}
	fmt.Println("✅ Licença:", licFile)

	fmt.Println("\n📋 Para usar:")
	fmt.Printf("  export TON_LICENSE_PUB_KEY=%s\n", pubFile)
	fmt.Printf("  export TON_LICENSE_FILE=%s\n", licFile)
	fmt.Println("  go build -tags commercial -o ton618-pro ./cmd/server")
}
