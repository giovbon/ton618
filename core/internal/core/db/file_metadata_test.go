package db

import "testing"

// ── file_metadata (metadados externos por arquivo) ──
//
// Arquivos (PDF/EPUB/ZIP/...) não têm frontmatter: o `pai` deles vive nesta
// tabela. Ver DECISIONS §6.24.

func TestFileMetadata_CRUD(t *testing.T) {
	s := newTestStore(t)

	if err := s.SetFileMetadata("pdfs/manual.pdf", "pai", "projeto"); err != nil {
		t.Fatalf("SetFileMetadata: %v", err)
	}
	if err := s.SetFileMetadata("attachments/x.zip", "pai", "projeto"); err != nil {
		t.Fatalf("SetFileMetadata: %v", err)
	}

	meta, err := s.GetFileMetadata("pdfs/manual.pdf")
	if err != nil {
		t.Fatalf("GetFileMetadata: %v", err)
	}
	if meta["pai"] != "projeto" {
		t.Errorf("pai = %q, esperava \"projeto\"", meta["pai"])
	}

	all, err := s.GetAllFileMetadata()
	if err != nil {
		t.Fatalf("GetAllFileMetadata: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("esperava 2 arquivos com metadados, got %d: %v", len(all), all)
	}
	if all["attachments/x.zip"]["pai"] != "projeto" {
		t.Errorf("metadado do zip ausente/errado: %v", all["attachments/x.zip"])
	}

	// Upsert: regravar a mesma chave sobrescreve.
	if err := s.SetFileMetadata("pdfs/manual.pdf", "pai", "outro"); err != nil {
		t.Fatalf("SetFileMetadata (upsert): %v", err)
	}
	meta, _ = s.GetFileMetadata("pdfs/manual.pdf")
	if meta["pai"] != "outro" {
		t.Errorf("upsert não sobrescreveu: %v", meta)
	}

	// Valor vazio REMOVE a chave (é como a UI devolve o arquivo para a raiz).
	if err := s.SetFileMetadata("pdfs/manual.pdf", "pai", ""); err != nil {
		t.Fatalf("SetFileMetadata (remove): %v", err)
	}
	meta, _ = s.GetFileMetadata("pdfs/manual.pdf")
	if len(meta) != 0 {
		t.Errorf("valor vazio deveria remover a chave, got %v", meta)
	}

	// DeleteFileMetadata remove tudo do arquivo.
	if err := s.DeleteFileMetadata("attachments/x.zip"); err != nil {
		t.Fatalf("DeleteFileMetadata: %v", err)
	}
	all, _ = s.GetAllFileMetadata()
	if len(all) != 0 {
		t.Errorf("esperava mapa vazio, got %v", all)
	}
}

// GetFileMetadata de um arquivo sem metadados devolve mapa vazio (não nil), e
// a tabela suporta várias chaves por arquivo (o `pai` é só a primeira).
func TestFileMetadata_MultiplasChaves(t *testing.T) {
	s := newTestStore(t)

	vazio, err := s.GetFileMetadata("pdfs/inexistente.pdf")
	if err != nil {
		t.Fatalf("GetFileMetadata: %v", err)
	}
	if vazio == nil || len(vazio) != 0 {
		t.Errorf("esperava mapa vazio não-nil, got %v", vazio)
	}

	s.SetFileMetadata("pdfs/a.pdf", "pai", "p1")
	s.SetFileMetadata("pdfs/a.pdf", "autor", "fulano")
	all, _ := s.GetAllFileMetadata()
	if all["pdfs/a.pdf"]["pai"] != "p1" || all["pdfs/a.pdf"]["autor"] != "fulano" {
		t.Errorf("esperava as duas chaves, got %v", all["pdfs/a.pdf"])
	}
}

// DeleteAllFileRecords é a fonte única de limpeza ao remover/renomear um
// arquivo: precisa levar os metadados junto (menos o `pai` de notas, que vive
// no frontmatter e é limpo com a nota).
func TestDeleteAllFileRecords_LimpaFileMetadata(t *testing.T) {
	s := newTestStore(t)

	if err := s.SaveNote("notes/n.md", "---\npai: x\n---\n", "2026-01-01T00:00:00Z"); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	if err := s.SetFileMetadata("notes/n.md", "pai", "x"); err != nil {
		t.Fatalf("SetFileMetadata: %v", err)
	}

	if err := s.DeleteAllFileRecords("notes/n.md"); err != nil {
		t.Fatalf("DeleteAllFileRecords: %v", err)
	}

	all, err := s.GetAllFileMetadata()
	if err != nil {
		t.Fatalf("GetAllFileMetadata: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("DeleteAllFileRecords deveria limpar file_metadata, got %v", all)
	}
}
