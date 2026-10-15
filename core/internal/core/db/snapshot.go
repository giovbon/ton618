package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
)

// snapshotDroppedTables são tabelas DERIVADAS — reconstruíveis a partir das notas —
// que ficam de fora do snapshot de backup. `note_embeddings` (vec0, 384 floats por
// chunk) é de longe a maior parte do banco, e o browser a regenera sozinho quando a
// busca semântica é aberta (indexação lazy, DECISIONS §3.8). Sem isso o arquivo de
// backup carregaria centenas de MB de dados que se recriam.
var snapshotDroppedTables = []string{"note_embeddings", "note_chunks"}

// Snapshot grava uma cópia consistente do banco em dest, sem as tabelas derivadas.
//
// Usa `VACUUM INTO` em vez de copiar o arquivo: aplica o WAL pendente e nunca produz
// uma cópia rasgada de um banco em uso (o app roda com `_journal_mode=WAL`).
// O banco vivo não é alterado — as limpezas acontecem na cópia.
func (s *Store) Snapshot(dest string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	// VACUUM INTO recusa um destino que já exista.
	if err := os.Remove(dest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("snapshot: remover destino: %w", err)
	}
	if _, err := s.DB.Exec(`VACUUM INTO ?`, dest); err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}

	snap, err := sql.Open("sqlite", dest)
	if err != nil {
		return fmt.Errorf("snapshot: abrir cópia: %w", err)
	}
	defer snap.Close()

	for _, table := range snapshotDroppedTables {
		if _, err := snap.Exec("DELETE FROM " + table); err != nil {
			// Não é fatal: o backup fica maior, mas continua completo.
			slog.Warn("snapshot: não foi possível limpar tabela derivada", "table", table, "error", err)
		}
	}
	// Compacta a cópia: o DELETE acima só devolve as páginas para a lista de livres.
	if _, err := snap.Exec("VACUUM"); err != nil {
		slog.Warn("snapshot: não foi possível compactar a cópia", "error", err)
	}
	// Um journal remanescente deixaria o arquivo dependente de um .journal externo.
	_ = os.Remove(dest + "-journal")

	return nil
}

// ValidateDatabaseFile confirma que path é um banco SQLite íntegro do TON-618.
// É a barreira antes de aplicar um backup enviado pelo usuário: nunca substituímos
// o estado atual por um arquivo que não conseguimos abrir e verificar.
func ValidateDatabaseFile(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("arquivo do banco ausente: %w", err)
	}

	// Sem DSN de somente-leitura de propósito: os testes de integridade não escrevem
	// e o arquivo é uma cópia temporária sob nosso controle.
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("abrir banco: %w", err)
	}
	defer conn.Close()

	var check string
	if err := conn.QueryRow(`PRAGMA integrity_check`).Scan(&check); err != nil {
		return fmt.Errorf("integrity_check: %w", err)
	}
	if check != "ok" {
		return fmt.Errorf("integrity_check retornou %q", check)
	}

	var n int
	if err := conn.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'notes'`,
	).Scan(&n); err != nil {
		return fmt.Errorf("inspecionar schema: %w", err)
	}
	if n == 0 {
		return errors.New("não parece um banco do TON-618 (tabela `notes` ausente)")
	}
	return nil
}
