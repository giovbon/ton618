package db

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

// backupBeforeMigration copia o banco SQLite (arquivo principal + WAL + SHM)
// para <dir-do-banco>/backups/pre-migration.* ANTES de qualquer migration rodar.
// Objetivo — integridade (#6): se um ALTER TABLE etc. falhar no meio e deixar o
// schema num estado parcial/corrompido, o operador tem um snapshot do último
// estado bom para restaurar.
//
// O backup é feito no boot, ANTES de abrir a conexão SQLite (o app está ocioso,
// sem escritores ativos). Copiar os três arquivos juntos (ton618.db, -wal, -shm)
// é o método documentado do SQLite para backup quando não há escrita
// concorrente. É um backup do "tipo rolling" (1 arquivo): o snapshot da
// execução anterior é sobrescrito, então sempre representa o estado
// imediatamente anterior à tentativa de migration ATUAL.
//
// Nunca é um ponto de falha do boot: se não der para criar/ copiar, apenas
// registra um warning e segue — falhar o startup seria pior do que continuar
// sem backup.
func backupBeforeMigration(path string) {
	if _, err := os.Stat(path); err != nil {
		return // banco ainda não existe (primeiro boot) — nada a preservar
	}
	dir := filepath.Dir(path)
	backupsDir := filepath.Join(dir, "backups")
	if err := os.MkdirAll(backupsDir, 0o755); err != nil {
		log.Printf("[db] pre-migration backup: não foi possível criar %s: %v", backupsDir, err)
		return
	}
	dest := filepath.Join(backupsDir, "pre-migration.db")
	for _, suffix := range []string{"", "-wal", "-shm"} {
		src := path + suffix
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := copySQLiteFile(src, dest+suffix); err != nil {
			log.Printf("[db] pre-migration backup: falha ao copiar %s: %v", src, err)
			return
		}
	}
}

func copySQLiteFile(src, dest string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	d, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer d.Close()
	if _, err := io.Copy(d, s); err != nil {
		return err
	}
	return d.Sync()
}
