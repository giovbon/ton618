package notes

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"ton618/core/internal/core/services"
	"ton618/core/internal/processor"
)

// maxRestoreUpload limita o tamanho do backup enviado (barreira contra encher o
// disco; um backup pessoal com arquivos pesados fica bem abaixo disso).
const maxRestoreUpload = 4 << 30 // 4 GiB

// HandleFullBackup baixa o backup TOTAL: o banco inteiro (notas, configurações,
// agenda, marcadores e a hierarquia de arquivos) + todos os arquivos do disco,
// inclusive archives/. É a única opção de backup que pode ser restaurada.
//
// GET /api/backup/total
func (ctx *HandlerContext) HandleFullBackup(w http.ResponseWriter, r *http.Request) {
	filename := "ton618-backup-total-" + processor.GenerateCUID2() + ".zip"

	// Headers antes do primeiro Write: depois o Go fixaria text/plain.
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if err := ctx.BackupFull.CreateStream(r.Context(), w); err != nil {
		// Headers já foram enviados: não dá para trocar por http.Error.
		slog.Error("HandleFullBackup: erro ao gerar backup total", "err", err)
	}
}

// HandleRestore aplica um backup total enviado pelo usuário. A restauração é
// SUBSTITUIÇÃO TOTAL: notas, arquivos e configurações atuais vão para uma
// quarentena e o conteúdo do backup passa a valer.
//
// POST /api/restore — multipart/form-data, campo "file"
//
// Responde um fragmento HTML (HTMX). No sucesso o processo se encerra logo depois:
// o banco aberto é o arquivo antigo, então é preciso reabrir — o supervisor
// (ForegroundService no Android, `restart: unless-stopped` no Docker) reinicia.
func (ctx *HandlerContext) HandleRestore(w http.ResponseWriter, r *http.Request) {
	// Streaming manual em vez de ParseMultipartForm: o arquivo pode ter GBs e não
	// deve passar pela memória nem pelo /tmp do sistema.
	mr, err := r.MultipartReader()
	if err != nil {
		writeRestoreResult(w, r, false, "Envio inválido: "+err.Error())
		return
	}

	tmp, err := os.CreateTemp(ctx.Cfg.StateDir, ".restore-upload-*.zip")
	if err != nil {
		slog.Error("HandleRestore: criar arquivo temporário", "err", err)
		writeRestoreResult(w, r, false, "Não foi possível preparar a restauração no servidor.")
		return
	}
	defer os.Remove(tmp.Name())

	var saved int64
	gotFile := false
	for {
		part, partErr := mr.NextPart()
		if partErr == io.EOF {
			break
		}
		if partErr != nil {
			tmp.Close()
			writeRestoreResult(w, r, false, "Falha ao receber o arquivo: "+partErr.Error())
			return
		}
		if part.FormName() != "file" {
			part.Close()
			continue
		}
		saved, err = io.Copy(tmp, io.LimitReader(part, maxRestoreUpload))
		part.Close()
		if err != nil {
			tmp.Close()
			writeRestoreResult(w, r, false, "Falha ao receber o arquivo: "+err.Error())
			return
		}
		gotFile = true
		break
	}
	if closeErr := tmp.Close(); closeErr != nil {
		slog.Warn("HandleRestore: fechar upload", "err", closeErr)
	}

	switch {
	case !gotFile || saved == 0:
		writeRestoreResult(w, r, false, "Selecione o arquivo de backup (.zip).")
		return
	case saved >= maxRestoreUpload:
		writeRestoreResult(w, r, false, "Arquivo maior que o limite aceito (4 GiB).")
		return
	}

	result, err := ctx.BackupFull.Restore(r.Context(), tmp.Name())
	if err != nil {
		slog.Error("HandleRestore: restauração falhou", "err", err)
		writeRestoreResult(w, r, false, "Restauração não aplicada — nada foi alterado. "+err.Error())
		return
	}

	slog.Info("HandleRestore: backup restaurado",
		"notes", result.Notes, "files", result.Files, "quarantine", result.Quarantine)
	writeRestoreResult(w, r, true, fmt.Sprintf(
		"Backup restaurado: %d notas e %d arquivos. O servidor vai reiniciar agora — "+
			"aguarde alguns segundos e recarregue a página. O estado anterior ficou em %s (por segurança).",
		result.Notes, result.Files, result.Quarantine))

	// Encerramento graciosamente: o http.Server aguarda esta requisição terminar,
	// então a resposta acima chega ao browser antes do processo sair.
	services.RequestRestart()
}

func writeRestoreResult(w http.ResponseWriter, r *http.Request, ok bool, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ArchiveAlert(message, ok).Render(r.Context(), w); err != nil {
		slog.Error("HandleRestore: render resultado", "err", err)
	}
}
