package layout

import (
	"strconv"
	"time"
)

// badgeCacheBuster gera um token único a cada renderização da página para a URL
// do badge de tarefas do cabeçalho.
//
// Motivo: a contagem é dinâmica e um valor antigo em cache do browser fazia o
// número parecer "travado" (já aconteceu: o usuário desmarcava um marcador e o
// cabeçalho continuava contando). O endpoint responde com `Cache-Control: no-store`,
// mas o token garante que nem uma resposta guardada antes desta mudança seja
// reaproveitada — cada página nova busca a contagem de verdade.
func badgeCacheBuster() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}
