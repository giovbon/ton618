//go:build ignore

package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	path := "internal/db/db.go"
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	// Remove BOM sequence (UTF-8 BOM: EF BB BF)
	cleaned := strings.TrimLeft(string(data), "\ufeff")
	cleaned = strings.TrimLeft(cleaned, "\xef\xbb\xbf")
	if len(cleaned) < len(data) {
		if err := os.WriteFile(path, []byte(cleaned), 0644); err != nil {
			panic(err)
		}
		fmt.Println("BOM removido com sucesso")
	} else {
		fmt.Println("Nenhum BOM encontrado")
	}
}
