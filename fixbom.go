// +build ignore

package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("internal/db/db.go")
	if err != nil {
		panic(err)
	}
	// Remove BOM
	cleaned := strings.TrimLeft(string(data), "\ufeff\u00ef\u00bb\u00bf\xef\xbb\xbf")
	if err := os.WriteFile("internal/db/db.go", []byte(cleaned), 0644); err != nil {
		panic(err)
	}
	fmt.Println("BOM removido")
}
