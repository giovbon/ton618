package semantic

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

func HashFunc(input string) string {
	h := sha256.New()
	h.Write([]byte(input))
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

func CalculateHash(metadata, text string, tags []string) string {
	h := sha256.New()
	h.Write([]byte(metadata))
	h.Write([]byte("|"))
	h.Write([]byte(text))
	h.Write([]byte("|"))
	h.Write([]byte(strings.Join(tags, ",")))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func CalculateVectorHash(metadata, text string) string {
	h := sha256.New()
	h.Write([]byte(metadata))
	h.Write([]byte("|"))
	h.Write([]byte(text))
	return fmt.Sprintf("%x", h.Sum(nil))
}
