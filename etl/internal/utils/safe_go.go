package utils

import (
	"log"
	"runtime/debug"
)

// SafeGo executa uma função em uma nova goroutine com um bloco defer recover()
// para evitar que pânicos não tratados derrubem a aplicação inteira.
func SafeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[ERRO CRÍTICO] Panic evitado em rotina de background: %v\nStack trace:\n%s\n", r, string(debug.Stack()))
			}
		}()
		fn()
	}()
}
