package services

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ton618/core/internal/core/db"
	"ton618/core/internal/processor"
)

// Formato do backup TOTAL — a 3ª opção de backup, que (ao contrário das opções
// "rápida" e "completa") também pode ser IMPORTADA para restaurar o estado.
//
//	manifest.json   metadados + checksum do banco (primeira entrada do ZIP)
//	db/ton618.db    snapshot consistente do SQLite (VACUUM INTO, sem derivadas)
//	docs/**         todos os arquivos do disco, inclusive archives/
//
// Decisão de design (ver docs/DECISIONS.md): snapshot do banco em vez de exportar
// entidade por entidade — fidelidade total (configurações, agenda, marcadores e a
// hierarquia de arquivos de `file_metadata`) com muito menos código. A contrapartida
// é depender do schema, o que é aceitável porque `db.NewStore` roda as migrações no
// boot: um backup antigo é migrado ao restaurar.
const (
	backupFormatName    = "ton618-backup"
	backupFormatVersion = 1

	backupManifestEntry = "manifest.json"
	backupDBEntry       = "db/ton618.db"
	backupDocsPrefix    = "docs/"

	// maxEntryBytes limita cada entrada extraída (barreira contra ZIP absurdo ou
	// corrompido enchendo o disco).
	maxEntryBytes = 2 << 30 // 2 GiB
)

var (
	errNoManifest = errors.New("backup sem manifest.json")
	errNoDatabase = errors.New("backup sem " + backupDBEntry)
)

// BackupManifest é a primeira entrada do ZIP e descreve o conteúdo do backup.
type BackupManifest struct {
	Format        string `json:"format"`
	FormatVersion int    `json:"format_version"`
	CreatedAt     string `json:"created_at"`
	Notes         int    `json:"notes"`
	Files         int    `json:"files"`
	DBSize        int64  `json:"db_bytes"`
	DBSHA256      string `json:"db_sha256"`
}

// RestoreResult resume uma restauração concluída.
type RestoreResult struct {
	Notes      int    // notas no backup restaurado
	Files      int    // arquivos gravados em docs/
	Bytes      int64  // tamanho do banco restaurado
	Quarantine string // diretório com o estado anterior (rede de segurança)
}

// FullBackupService exporta e restaura o estado completo do app (banco + arquivos).
type FullBackupService struct {
	store    *db.Store
	dbPath   string
	docsDir  string
	stateDir string
}

// NewFullBackupService cria o serviço de backup total/restauração.
func NewFullBackupService(store *db.Store, dbPath, docsDir, stateDir string) *FullBackupService {
	return &FullBackupService{store: store, dbPath: dbPath, docsDir: docsDir, stateDir: stateDir}
}

// CreateStream escreve o ZIP do backup total em out (streaming: nada é acumulado
// em memória além das notas de metadados).
func (s *FullBackupService) CreateStream(ctx context.Context, out io.Writer) error {
	staging, err := os.MkdirTemp(s.stateDir, ".snapshot-")
	if err != nil {
		return fmt.Errorf("backup total: preparar staging: %w", err)
	}
	defer os.RemoveAll(staging)

	snapPath := filepath.Join(staging, "ton618.db")
	if err := s.store.Snapshot(snapPath); err != nil {
		return err
	}

	info, err := os.Stat(snapPath)
	if err != nil {
		return fmt.Errorf("backup total: inspecionar snapshot: %w", err)
	}
	sum, err := sha256File(snapPath)
	if err != nil {
		return err
	}

	files, err := listDocFiles(s.docsDir)
	if err != nil {
		return err
	}
	allNotes, err := s.store.GetAllNotes()
	if err != nil {
		return fmt.Errorf("backup total: listar notas: %w", err)
	}

	manifest := BackupManifest{
		Format:        backupFormatName,
		FormatVersion: backupFormatVersion,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		Notes:         len(allNotes),
		Files:         len(files),
		DBSize:        info.Size(),
		DBSHA256:      sum,
	}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("backup total: manifest: %w", err)
	}

	zw := zip.NewWriter(out)
	// O manifest vem primeiro para que um leitor possa inspecionar o backup sem
	// varrer o arquivo inteiro.
	if err := addBytesToZip(zw, backupManifestEntry, manifestJSON, time.Now()); err != nil {
		return err
	}
	if err := addFileToZip(zw, backupDBEntry, snapPath, info.ModTime()); err != nil {
		return err
	}
	for _, rel := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		full := filepath.Join(s.docsDir, filepath.FromSlash(rel))
		fi, statErr := os.Stat(full)
		if statErr != nil || fi.IsDir() {
			continue
		}
		if err := addFileToZip(zw, backupDocsPrefix+rel, full, fi.ModTime()); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("backup total: fechar zip: %w", err)
	}
	return nil
}

// Restore aplica um ZIP de backup total. fluxo:
//
//  1. lê o manifest e EXTRAI para um staging temporário (nada do estado atual é tocado);
//  2. valida o banco extraído (integridade + tabela `notes`);
//  3. move o estado atual para uma quarentena (banco + docs) e aplica o backup;
//  4. quem chama deve reiniciar o processo — o banco aberto é o arquivo antigo.
//
// Um erro em qualquer passo antes do item 3 deixa o app exatamente como estava.
func (s *FullBackupService) Restore(ctx context.Context, zipPath string) (*RestoreResult, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("arquivo inválido (não é um ZIP legível): %w", err)
	}
	defer zr.Close()

	manifest, err := readManifest(zr)
	if err != nil {
		return nil, err
	}
	if manifest.Format != backupFormatName {
		return nil, fmt.Errorf("formato inesperado %q (esperado %q)", manifest.Format, backupFormatName)
	}
	if manifest.FormatVersion > backupFormatVersion {
		return nil, fmt.Errorf("backup da versão %d é mais novo que este app suporta (%d)",
			manifest.FormatVersion, backupFormatVersion)
	}

	// Staging FORA de docsDir (precisa estar no mesmo sistema de arquivos para o
	// rename final) e fora do banco.
	staging := filepath.Join(filepath.Dir(s.docsDir), ".ton618-restore-"+processor.GenerateCUID2())
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return nil, fmt.Errorf("preparar staging: %w", err)
	}
	defer os.RemoveAll(staging)

	stagedDB := filepath.Join(staging, "ton618.db")
	stagedDocs := filepath.Join(staging, "docs")
	if err := os.MkdirAll(stagedDocs, 0o755); err != nil {
		return nil, fmt.Errorf("preparar staging: %w", err)
	}

	dbFound := false
	fileCount := 0
	for _, f := range zr.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := filepath.ToSlash(f.Name)
		switch {
		case name == backupDBEntry:
			if err := extractZipEntry(f, stagedDB); err != nil {
				return nil, fmt.Errorf("extrair banco: %w", err)
			}
			dbFound = true
		case strings.HasPrefix(name, backupDocsPrefix):
			// Entradas de diretório não são arquivos: extractZipEntry já as
			// ignora, mas o contador somava +1 e o RestoreResult.Files ficava
			// maior que o número real de arquivos restaurados.
			if f.FileInfo().IsDir() {
				continue
			}
			rel, err := safeZipRel(name, backupDocsPrefix)
			if err != nil {
				return nil, err
			}
			if err := extractZipEntry(f, filepath.Join(stagedDocs, rel)); err != nil {
				return nil, fmt.Errorf("extrair %s: %w", name, err)
			}
			fileCount++
		}
	}
	if !dbFound {
		return nil, errNoDatabase
	}
	// Barreira final antes de tocar no estado atual.
	if err := db.ValidateDatabaseFile(stagedDB); err != nil {
		return nil, fmt.Errorf("banco do backup reprovado na validação: %w", err)
	}

	// ── A partir daqui o estado atual é substituído ──
	quarantine := filepath.Join(s.stateDir, ".pre-restore-"+time.Now().UTC().Format("20060102-150405"))
	if err := os.MkdirAll(quarantine, 0o755); err != nil {
		return nil, fmt.Errorf("criar quarentena: %w", err)
	}

	// docs: renomeia a pasta inteira (instantâneo, mesmo filesystem).
	if _, err := os.Stat(s.docsDir); err == nil {
		if err := os.Rename(s.docsDir, filepath.Join(quarantine, "docs")); err != nil {
			return nil, fmt.Errorf("quarentena dos arquivos: %w", err)
		}
	}
	if err := os.Rename(stagedDocs, s.docsDir); err != nil {
		return nil, fmt.Errorf("aplicar arquivos (estado anterior em %s): %w", quarantine, err)
	}

	// banco + WAL/SHM: os três juntos, senão o WAL órfão ficaria inconsistente.
	for _, suffix := range []string{"", "-wal", "-shm"} {
		src := s.dbPath + suffix
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(quarantine, filepath.Base(s.dbPath)+suffix)
		if err := os.Rename(src, dst); err != nil {
			return nil, fmt.Errorf("quarentena do banco (estado anterior em %s): %w", quarantine, err)
		}
	}
	if err := os.Rename(stagedDB, s.dbPath); err != nil {
		return nil, fmt.Errorf("aplicar banco (estado anterior em %s): %w", quarantine, err)
	}
	// Recria pdfs/attachments/archives dentro do novo docs/.
	if err := os.MkdirAll(filepath.Join(s.docsDir, "pdfs"), 0o755); err != nil {
		return nil, fmt.Errorf("recriar estrutura de docs: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(s.docsDir, "attachments"), 0o755); err != nil {
		return nil, fmt.Errorf("recriar estrutura de docs: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(s.docsDir, "archives"), 0o755); err != nil {
		return nil, fmt.Errorf("recriar estrutura de docs: %w", err)
	}

	return &RestoreResult{
		Notes:      manifest.Notes,
		Files:      fileCount,
		Bytes:      manifest.DBSize,
		Quarantine: quarantine,
	}, nil
}

// readManifest lê e valida o manifest.json do backup.
func readManifest(zr *zip.ReadCloser) (*BackupManifest, error) {
	for _, f := range zr.File {
		if filepath.ToSlash(f.Name) != backupManifestEntry {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("ler manifest: %w", err)
		}
		defer rc.Close()

		var m BackupManifest
		if err := json.NewDecoder(io.LimitReader(rc, 1<<20)).Decode(&m); err != nil {
			return nil, fmt.Errorf("manifest inválido: %w", err)
		}
		return &m, nil
	}
	return nil, errNoManifest
}

// safeZipRel valida o caminho de uma entrada do ZIP (proteção contra "zip slip":
// um ZIP malicioso com `../../etc/x` não pode escrever fora do destino).
func safeZipRel(name, prefix string) (string, error) {
	rel := strings.TrimPrefix(filepath.ToSlash(name), prefix)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		return "", fmt.Errorf("caminho vazio no backup: %q", name)
	}
	if filepath.VolumeName(rel) != "" {
		return "", fmt.Errorf("caminho com volume no backup: %q", name)
	}
	cleaned := filepath.Clean(filepath.FromSlash(rel))
	if cleaned == "." || cleaned == ".." || filepath.IsAbs(cleaned) ||
		strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("caminho inválido no backup: %q", name)
	}
	return cleaned, nil
}

// extractZipEntry grava uma entrada do ZIP em dest (ignora diretórios).
func extractZipEntry(f *zip.File, dest string) error {
	if f.FileInfo().IsDir() {
		return nil
	}
	if f.UncompressedSize64 > maxEntryBytes {
		return fmt.Errorf("entrada grande demais (%d bytes)", f.UncompressedSize64)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, io.LimitReader(rc, maxEntryBytes)); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// listDocFiles devolve os caminhos relativos (slash) de todos os arquivos de
// docsDir. O backup total não exclui nada — inclusive archives/.
func listDocFiles(docsDir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(docsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// Um arquivo ilegível não invalida o backup inteiro.
			slog.Warn("backup total: entrada ilegível ignorada", "path", path, "error", err)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(docsDir, path)
		if relErr != nil {
			return nil
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("backup total: varrer docs: %w", err)
	}
	// Ordem determinística (facilita diff entre backups).
	sort.Strings(out)
	return out, nil
}

func addBytesToZip(zw *zip.Writer, name string, data []byte, modTime time.Time) error {
	h := &zip.FileHeader{Name: name, Method: zip.Deflate}
	h.SetModTime(modTime)
	w, err := zw.CreateHeader(h)
	if err != nil {
		return fmt.Errorf("zip: %s: %w", name, err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("zip: %s: %w", name, err)
	}
	return nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
