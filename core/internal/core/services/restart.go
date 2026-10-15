package services

// RestartRequested é sinalizado quando uma operação exige reiniciar o processo.
//
// Hoje só a restauração de backup: ela troca o arquivo do banco em disco, e a
// conexão SQLite já aberta continua apontando para o arquivo antigo. O processo
// encerra graciosamente (o main observa este canal) e o supervisor traz o servidor
// de volta — ForegroundService no app Android, `restart: unless-stopped` no Docker.
var RestartRequested = make(chan struct{}, 1)

// RequestRestart solicita o encerramento graciosamente (idempotente).
func RequestRestart() {
	select {
	case RestartRequested <- struct{}{}:
	default: // já solicitado
	}
}
