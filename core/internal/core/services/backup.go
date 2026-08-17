package services

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"ton618/core/internal/repository"

	"gopkg.in/yaml.v3"
)

// BackupService gera backups ZIP de todos os dados (exceto archives/).
type BackupService struct {
	notes   repository.NoteStore
	fileMod repository.FileModStore
	docsDir string
}

// NewBackupService cria o serviço de backup.
func NewBackupService(notes repository.NoteStore, fm repository.FileModStore, docsDir string) *BackupService {
	return &BackupService{notes: notes, fileMod: fm, docsDir: docsDir}
}

// parseFrontmatterBody separa o frontmatter (bloco YAML entre ---) e o corpo.
func parseFrontmatterBody(content string) (string, string) {
	text := strings.TrimSpace(content)
	if !strings.HasPrefix(text, "---") {
		return "", content
	}

	parts := strings.SplitN(text, "---", 3)
	if len(parts) >= 3 {
		return strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2])
	}
	return "", content
}

// detectNoteTypeFromFrontmatter faz o parse YAML do frontmatter e retorna o valor da chave "type".
// Diferente de parseFrontmatterBody (que só separa texto), esta função usa unmarshal YAML
// para evitar falsos positivos com strings.Contains (ex: "description: 'type: drawing'").
func detectNoteTypeFromFrontmatter(content string) string {
	fm, _ := parseFrontmatterBody(content)
	if fm == "" {
		return ""
	}
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(fm), &data); err != nil {
		return ""
	}
	if t, ok := data["type"]; ok {
		if s, ok := t.(string); ok {
			return s
		}
	}
	return ""
}

var binaryCompressedExts = map[string]bool{
	".zip":  true,
	".rar":  true,
	".7z":   true,
	".tar":  true,
	".gz":   true,
	".tgz":  true,
	".bz2":  true,
	".tbz2": true,
	".xz":   true,
	".txz":  true,
	".lzma": true,
	".zst":  true,
	".apk":  true,
	".jar":  true,
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".webp": true,
	".heic": true,
	".heif": true,
	".tiff": true,
	".ico":  true,
	".mp3":  true,
	".m4a":  true,
	".aac":  true,
	".flac": true,
	".ogg":  true,
	".opus": true,
	".wav":  true,
	".wma":  true,
	".mp4":  true,
	".mkv":  true,
	".avi":  true,
	".mov":  true,
	".webm": true,
	".flv":  true,
	".wmv":  true,
	".mpeg": true,
	".mpg":  true,
	".iso":  true,
	".img":  true,
	".dmg":  true,
	".bin":  true,
	".exe":  true,
	".dll":  true,
	".so":   true,
	".dylib": true,
	".pdf":  true,
	".epub": true,
}

func selectCompressionMethod(filename string) uint16 {
	ext := strings.ToLower(filepath.Ext(filename))
	if binaryCompressedExts[ext] {
		return zip.Store
	}
	return zip.Deflate
}

// CreateStream gera o arquivo ZIP enviando o fluxo de dados diretamente para out (ex: http.ResponseWriter).
// Usa context.Background (sem cancelamento). Prefira CreateStreamContext quando
// houver um contexto HTTP disponível para abortar se o cliente desconectar.
func (s *BackupService) CreateStream(out io.Writer, full bool) error {
	return s.CreateStreamContext(context.Background(), out, full)
}

// CreateStreamContext gera o ZIP em fluxo direto para out, observando ctx.
// Se o cliente desconectar (ctx cancelado) ou qualquer escrita falhar, aborta
// imediatamente e retorna o erro — evita comprimir dados para uma conexão morta
// e garante que o handler registre a falha (em vez de engolir o erro e entregar
// um ZIP truncado/corrompido).
func (s *BackupService) CreateStreamContext(ctx context.Context, out io.Writer, full bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	allNotes, _ := s.notes.GetAllNotes()

	zw := zip.NewWriter(out)
	seen := make(map[string]bool)

	// 1. Notas do DB — conteúdo markdown
	// Recolhe as notas e comprime em paralelo: o Deflate é CPU-bound e era o
	// gargalo em máquinas fracas (ex: ARMv7). A escrita no ZIP permanece
	// serializada, mas a compressão roda num pool limitado de workers.
	var entries []noteEntry
	for filename, mtimeStr := range allNotes {
		if strings.HasPrefix(filename, "archives/") {
			continue
		}
		content, err := s.notes.GetNote(filename)
		if err != nil || content == "" {
			continue
		}

		originalFilename := filename
		if !strings.HasSuffix(filename, ".md") {
			filename += ".md"
		}

		_, body := parseFrontmatterBody(content)
		noteType := detectNoteTypeFromFrontmatter(content)

		zipFilename := filename
		var zipData []byte

		switch noteType {
		case "drawing":
			// Desenho -> .excalidraw
			zipFilename = strings.TrimSuffix(filename, ".md") + ".excalidraw"
			zipData = []byte(body)
		default:
			// Markdown normal
			zipData = []byte(content)
		}

		entries = append(entries, noteEntry{
			name:    zipFilename,
			data:    zipData,
			modTime: repository.ParseMtime(mtimeStr),
		})
		seen[filename] = true
		seen[originalFilename] = true
		seen[zipFilename] = true
	}

	if err := s.writeNotesParallel(ctx, zw, entries); err != nil {
		return err
	}

	// 2. Arquivos do disco (PDFs, attachments, imagens, notas sem conteúdo no DB, etc.)
	// I/O-bound (zip.Store / io.Copy) — mantém sequencial, mas observa o ctx e
	// propaga erros (antes eram engolidos com `_ =`).
	if full {
		if err := filepath.WalkDir(s.docsDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}

			if err := ctx.Err(); err != nil {
				return err
			}

			relPath, err := filepath.Rel(s.docsDir, path)
			if err != nil {
				return nil
			}
			relPath = filepath.ToSlash(relPath)

			if strings.HasPrefix(relPath, "archives/") || seen[relPath] {
				return nil
			}

			info, err := d.Info()
			var modTime time.Time
			if err == nil {
				modTime = info.ModTime()
			} else if mtimeStr, _ := s.fileMod.GetFileMod(relPath); mtimeStr != "" {
				modTime = repository.ParseMtime(mtimeStr)
			}

			if err := addFileToZip(zw, relPath, path, modTime); err != nil {
				return err
			}
			seen[relPath] = true
			return nil
		}); err != nil {
			return err
		}
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("backup: close zip: %w", err)
	}
	return nil
}

// Create gera um ZIP em memória (para uso onde []byte é necessário).
func (s *BackupService) Create(full bool) ([]byte, error) {
	var buf bytes.Buffer
	if err := s.CreateStream(&buf, full); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ── Compressão paralela das notas ──

// noteEntry é uma nota a ser gravada no ZIP (já com nome/transformação aplicados).
type noteEntry struct {
	name    string
	data    []byte
	modTime time.Time
}

// compressedEntry é o resultado da compressão de uma noteEntry, pronto para
// ser gravado via zip.Writer.CreateRaw (que aceita dados já comprimidos).
type compressedEntry struct {
	name    string
	method  uint16
	crc32   uint32
	uncomp  uint64
	comp    uint64
	data    []byte
	modTime time.Time
}

// compressEntry comprime a entrada com DEFLATE e calcula CRC32/tamanhos.
// É chamado concorrentemente pelos workers do pool.
func compressEntry(e noteEntry) compressedEntry {
	var buf bytes.Buffer
	method := uint16(zip.Deflate)
	fw, err := flate.NewWriter(&buf, flate.DefaultCompression)
	if err != nil {
		// Fallback defensivo: entra sem compressão (Store).
		method = zip.Store
		buf.Write(e.data)
	} else {
		fw.Write(e.data)
		fw.Close()
	}
	return compressedEntry{
		name:    e.name,
		method:  method,
		crc32:   crc32.ChecksumIEEE(e.data),
		uncomp:  uint64(len(e.data)),
		comp:    uint64(buf.Len()),
		data:    buf.Bytes(),
		modTime: e.modTime,
	}
}

// writeNotesParallel comprime as notas em um pool limitado de workers
// (compressão é CPU-bound) e grava no ZIP sequencialmente via CreateRaw.
// O número de workers é limitado a GOMAXPROCS (máx. 4) para não sobrecarregar
// máquinas fracas (ex: ARMv7 de 4 núcleos) nem estourar memória.
func (s *BackupService) writeNotesParallel(ctx context.Context, zw *zip.Writer, entries []noteEntry) error {
	if len(entries) == 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	workers := runtime.GOMAXPROCS(0)
	if workers > 4 {
		workers = 4
	}
	if workers < 1 {
		workers = 1
	}
	if len(entries) < 2 {
		workers = 1
	}

	jobs := make(chan int)
	results := make(chan compressedEntry, workers)

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				ent := compressEntry(entries[i])
				select {
				case results <- ent:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Alimenta o pool. Em cancelamento, para de alimentar e fecha o canal para
	// que os workers terminem sem vazar goroutines.
	go func() {
		defer close(jobs)
		for i := range entries {
			select {
			case jobs <- i:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	// Grava as entradas conforme chegam. O ZIP não exige ordem entre arquivos —
	// o diretório central é gravado no Close() com os offsets corretos.
	for res := range results {
		if err := ctx.Err(); err != nil {
			return err
		}
		h := &zip.FileHeader{
			Name:   res.name,
			Method: res.method,
		}
		h.CRC32 = res.crc32
		h.CompressedSize64 = res.comp
		h.UncompressedSize64 = res.uncomp
		if !res.modTime.IsZero() {
			h.SetModTime(res.modTime)
		}
		w, err := zw.CreateRaw(h)
		if err != nil {
			return err
		}
		if _, err := w.Write(res.data); err != nil {
			return err
		}
	}
	return nil
}

func addFileToZip(zw *zip.Writer, zipName string, filePath string, modTime time.Time) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	h := &zip.FileHeader{
		Name:   zipName,
		Method: selectCompressionMethod(zipName),
	}
	if !modTime.IsZero() {
		h.SetModTime(modTime)
	} else {
		h.SetModTime(info.ModTime())
	}

	w, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, file)
	return err
}
