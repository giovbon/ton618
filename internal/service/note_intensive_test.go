package service

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"ton618/internal/processor"
)

// TestNoteService_Save_Concurrent testa o salvamento de notas sob alta concorrência.
// Simula múltiplos usuários salvando notas diferentes simultaneamente.
func TestNoteService_Save_Concurrent(t *testing.T) {
	svc, _, store, cleanup := newTestService(t)
	defer cleanup()

	const numWorkers = 30
	var wg sync.WaitGroup
	errs := make(chan error, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			filename := fmt.Sprintf("concurrent-note-%d", id)
			content := fmt.Sprintf("# Nota Concorrente %d\n\nEste é o conteúdo da nota concorrente %d com tags #tag%d e [[link-destino-%d]].\n\nTODO: terminar esta tarefa na nota %d", id, id, id, id, id)

			err := svc.Save(filename, content, nil)
			if err != nil {
				errs <- fmt.Errorf("worker %d falhou: %w", id, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}

	// Verifica se todas as notas foram indexadas no SQLite
	for i := 0; i < numWorkers; i++ {
		filename := fmt.Sprintf("notes/concurrent-note-%d.md", i)
		if !store.NoteExists(filename) {
			t.Errorf("nota %s não existe no banco de dados", filename)
			continue
		}

		// Valida documentos
		docs, err := store.GetDocumentsByFile(filename)
		if err != nil {
			t.Errorf("erro ao obter documentos de %s: %v", filename, err)
		}
		if len(docs) == 0 {
			t.Errorf("nota %s não gerou documentos de busca", filename)
		}

		// Valida tags
		tags, err := store.GetFileTags(filename)
		if err != nil {
			t.Errorf("erro ao obter tags de %s: %v", filename, err)
		}
		expectedTag := fmt.Sprintf("tag%d", i)
		foundTag := false
		for _, tag := range tags {
			if tag == expectedTag {
				foundTag = true
				break
			}
		}
		if !foundTag {
			t.Errorf("tag %s não encontrada para nota %s, tags indexadas: %v", expectedTag, filename, tags)
		}

		// Valida link
		links, err := store.GetLinks(filename)
		if err != nil {
			t.Errorf("erro ao obter links de %s: %v", filename, err)
		}
		expectedLink := fmt.Sprintf("notes/link-destino-%d.md", i)
		foundLink := false
		for _, l := range links {
			if l == expectedLink {
				foundLink = true
				break
			}
		}
		if !foundLink {
			t.Errorf("link para %s não encontrado na nota %s, links indexados: %v", expectedLink, filename, links)
		}
	}
}

// TestNoteService_Save_ConcurrentSameFile testa salvamentos concorrentes no MESMO arquivo.
// Garante que não ocorram deadlocks nem corrupções, e que os índices permaneçam consistentes.
func TestNoteService_Save_ConcurrentSameFile(t *testing.T) {
	svc, _, store, cleanup := newTestService(t)
	defer cleanup()

	const filename = "shared-note"
	const fullFilename = "notes/shared-note.md"
	const numWorkers = 15
	var wg sync.WaitGroup
	errs := make(chan error, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Conteúdo varia ligeiramente por worker
			content := fmt.Sprintf("# Nota Compartilhada\n\nSalvo pelo worker %d.\n\nTODO: tarefa do worker %d", id, id)
			err := svc.Save(filename, content, nil)
			if err != nil {
				errs <- fmt.Errorf("worker %d salvando o mesmo arquivo falhou: %w", id, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}

	// Verifica se a nota existe
	if !store.NoteExists(fullFilename) {
		t.Fatalf("nota compartilhada %s não existe no banco de dados", fullFilename)
	}

	// O conteúdo final deve ser de um dos workers
	content, err := store.GetNote(fullFilename)
	if err != nil {
		t.Fatalf("erro ao obter nota compartilhada: %v", err)
	}
	if !strings.Contains(content, "Salvo pelo worker") {
		t.Errorf("conteúdo da nota está corrompido ou ausente: %q", content)
	}

	// Garante que os documentos indexados não se duplicaram (devem pertencer apenas ao estado final salvo)
	docs, err := store.GetDocumentsByFile(fullFilename)
	if err != nil {
		t.Fatalf("erro ao buscar documentos: %v", err)
	}
	// Cada versão do conteúdo tem 1 cabeçalho e corpo, então deve gerar no máximo 2 documentos.
	// Se houvesse vazamento ou falta de deleção/duplicação na transação, teríamos muitos documentos.
	if len(docs) > 3 {
		t.Errorf("esperava poucos documentos para o arquivo (limpeza da transação falhou?), obteve: %d", len(docs))
	}
}

// TestNoteService_Save_TransactionRollback testa se a transação do banco é revertida (rollback)
// caso ocorra algum erro durante o processo de indexação no SQLite.
func TestNoteService_Save_TransactionRollback(t *testing.T) {
	svc, _, store, cleanup := newTestService(t)
	defer cleanup()

	const filename = "notes/rollback-note.md"

	// 1. Salva inicialmente com dados válidos
	initialContent := "# Nota Inicial\n\nTexto inicial."
	err := svc.Save("rollback-note", initialContent, nil)
	if err != nil {
		t.Fatalf("erro no salvamento inicial: %v", err)
	}

	// Garante que os documentos iniciais foram criados
	docsBefore, err := store.GetDocumentsByFile(filename)
	if err != nil || len(docsBefore) == 0 {
		t.Fatalf("deveria ter indexado documentos inicialmente: %v, total: %d", err, len(docsBefore))
	}

	// 2. Tenta fazer um ReplaceFileIndexes diretamente com dados que causam falha.
	// Vamos forçar um erro de chave duplicada na tabela de TODOS ao passar duas tarefas com o mesmo ID
	// (A tabela 'todos' possui a chave 'id TEXT PRIMARY KEY').
	dupTodoID := "todo-id-duplicado"
	badTodos := []processor.TodoItem{
		{
			ID:      dupTodoID,
			File:    filename,
			Section: "Geral",
			Type:    "TODO",
			Status:  "pending",
			Text:    "Primeira tarefa",
			Line:    5,
			Created: time.Now(),
		},
		{
			ID:      dupTodoID, // ID duplicado provoca UNIQUE constraint failed
			File:    filename,
			Section: "Geral",
			Type:    "TODO",
			Status:  "pending",
			Text:    "Segunda tarefa com mesma ID",
			Line:    6,
			Created: time.Now(),
		},
	}

	badDocs := []processor.Document{
		{
			ID:      "novo-doc-id",
			Tipo:    "markdown",
			Arquivo: filename,
			Secao:   "Geral",
			Texto:   "Este texto nunca deveria ser indexado permanentemente por causa do erro de constraint.",
		},
	}

	// Tenta executar a substituição de índices lógicos. Esperamos um erro!
	err = store.ReplaceFileIndexes(filename, badDocs, nil, nil, badTodos, time.Now())
	if err == nil {
		t.Fatal("esperava erro UNIQUE constraint ao inserir todos duplicados, mas retornou nil")
	}

	// 3. Verifica que os dados originais AINDA existem e NÃO foram apagados/substituídos.
	// A transação inteira deve ter sofrido rollback.
	docsAfter, err := store.GetDocumentsByFile(filename)
	if err != nil {
		t.Fatalf("erro ao buscar documentos após falha: %v", err)
	}

	if len(docsAfter) != len(docsBefore) {
		t.Errorf("Rollback falhou! A quantidade de documentos mudou de %d para %d", len(docsBefore), len(docsAfter))
	}

	for _, doc := range docsAfter {
		if doc.ID == "novo-doc-id" {
			t.Error("Rollback falhou! O documento novo da transação quebrada foi inserido")
		}
	}
}

// TestNoteService_Save_KeywordsYAML valida se a detecção de keywords: true no frontmatter YAML
// dispara a extração RAKE e atualiza tanto o banco de dados como o arquivo físico com as keywords detectadas.
func TestNoteService_Save_KeywordsYAML(t *testing.T) {
	svc, _, store, cleanup := newTestService(t)
	defer cleanup()

	content := `---
keywords: true
---
# Inteligência Artificial e Aprendizado de Máquina
A Inteligência Artificial é um ramo da ciência da computação. O aprendizado de máquina (ou machine learning) e o processamento de linguagem natural são subcampos fascinantes.`

	filename := "nota-keywords-yaml"

	err := svc.Save(filename, content, nil)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verifica se as keywords foram indexadas na tabela notes do DB
	keywords, err := store.GetNoteKeywords("notes/" + filename + ".md")
	if err != nil {
		t.Fatalf("GetNoteKeywords: %v", err)
	}

	if len(keywords) == 0 {
		t.Error("esperava que palavras-chave fossem extraídas e salvas no DB, mas o slice está vazio")
	} else {
		t.Logf("Palavras-chave extraídas (YAML): %v", keywords)
	}

	// Verifica se a nota armazenada no DB contém a propriedade "keywords" preenchida no frontmatter
	dbContent, err := store.GetNote("notes/" + filename + ".md")
	if err != nil {
		t.Fatalf("erro ao ler nota do DB: %v", err)
	}

	if !strings.Contains(dbContent, "keywords:") {
		t.Error("a nota deveria conter a propriedade 'keywords' no frontmatter")
	}

	// Certifica que não contém a string literal "keywords: true" mais (foi substituída pelas palavras-chave)
	if strings.Contains(dbContent, "keywords: true") {
		t.Error("a propriedade 'keywords: true' deveria ter sido substituída pela lista de termos extraídos")
	}
}

// TestNoteService_Save_KeywordsHashtag valida se a tag #keywords no corpo do markdown
// habilita a extração automática de keywords.
func TestNoteService_Save_KeywordsHashtag(t *testing.T) {
	svc, _, store, cleanup := newTestService(t)
	defer cleanup()

	content := `# Minha Nota Especial #keywords
Este texto contém palavras-chave importantes sobre a linguagem Go, programação concorrente e bancos de dados SQLite.`

	filename := "nota-keywords-tag"
	err := svc.Save(filename, content, nil)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verifica keywords no DB
	keywords, err := store.GetNoteKeywords("notes/" + filename + ".md")
	if err != nil {
		t.Fatalf("GetNoteKeywords: %v", err)
	}

	if len(keywords) == 0 {
		t.Error("esperava palavras-chave extraídas por causa da tag #keywords")
	} else {
		t.Logf("Palavras-chave extraídas (Tag): %v", keywords)
	}
}

// TestNoteService_Save_NoKeywords verifica que notas que não solicitam keywords
// não sofrem extração de keywords, e qualquer keywords existente é removida.
func TestNoteService_Save_NoKeywords(t *testing.T) {
	svc, _, store, cleanup := newTestService(t)
	defer cleanup()

	// Nota sem keywords: true e sem tag #keywords
	content := `# Nota Comum
Texto explicativo normal sobre qualquer assunto.`

	filename := "nota-sem-keywords"
	err := svc.Save(filename, content, nil)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	keywords, err := store.GetNoteKeywords("notes/" + filename + ".md")
	if err != nil {
		t.Fatalf("GetNoteKeywords: %v", err)
	}
	if len(keywords) > 0 {
		t.Errorf("não deveria extrair keywords se não solicitado, got: %v", keywords)
	}
}

// TestNoteService_Save_RapidRepetitive testa a robustez do salvamento repetitivo e rápido (stress test).
// Simula a escrita sequencial rápida (auto-save agressivo).
func TestNoteService_Save_RapidRepetitive(t *testing.T) {
	svc, _, store, cleanup := newTestService(t)
	defer cleanup()

	const filename = "stress-note"
	const fullFilename = "notes/stress-note.md"
	const iterations = 100

	for i := 0; i < iterations; i++ {
		content := fmt.Sprintf("# Nota Stress\n\nVersão %d da nota.\n\nTODO: tarefa %d", i, i)
		err := svc.Save(filename, content, nil)
		if err != nil {
			t.Fatalf("falhou na iteração %d: %v", i, err)
		}
	}

	// Valida se o estado final corresponde à última iteração
	gotNote, err := store.GetNote(fullFilename)
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	expectedContent := fmt.Sprintf("# Nota Stress\n\nVersão %d da nota.\n\nTODO: tarefa %d", iterations-1, iterations-1)
	if gotNote != expectedContent {
		t.Errorf("conteúdo final incorreto.\nEsperado: %q\nObtido: %q", expectedContent, gotNote)
	}

	// Valida que os todos contêm apenas o último estado (tarefa 99)
	rows, err := store.DB.Query("SELECT text FROM todos WHERE file = ?", fullFilename)
	if err != nil {
		t.Fatalf("query todos: %v", err)
	}
	defer rows.Close()

	var todoTexts []string
	for rows.Next() {
		var text string
		rows.Scan(&text)
		todoTexts = append(todoTexts, text)
	}

	if len(todoTexts) != 1 {
		t.Errorf("esperava exatamente 1 tarefa indexada, obteve %d: %v", len(todoTexts), todoTexts)
	} else if todoTexts[0] != fmt.Sprintf("tarefa %d", iterations-1) {
		t.Errorf("tarefa incorreta. Esperava 'tarefa %d', obteve %q", iterations-1, todoTexts[0])
	}
}

// TestNoteService_Rename_NoDuplication verifica se ao renomear uma nota,
// todas as tabelas de índice (documents, docs_fts, tags, links, todos)
// limpam os registros do nome antigo e transferem para o nome novo,
// garantindo que não ocorra nenhuma duplicação de dados no SQLite.
func TestNoteService_Rename_NoDuplication(t *testing.T) {
	svc, _, store, cleanup := newTestService(t)
	defer cleanup()

	const oldName = "notes/nota-original.md"
	const newName = "notes/nota-renomeada.md"

	content := `---
tags: [tag-rename, teste]
---
# Título Original
Conteúdo de texto único para busca.

## Seção 1
Link importante para [[outra-nota]].
TODO: tarefa importante de rename.

## Seção 2
Outro parágrafo de conteúdo.
`

	// 1. Salva a nota com o nome antigo
	err := svc.Save("nota-original", content, nil)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Helper para contar registros nas tabelas lógicas
	countTable := func(table, fileCol, file string) int {
		var count int
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = ?", table, fileCol)
		err := store.DB.QueryRow(query, file).Scan(&count)
		if err != nil {
			t.Fatalf("erro ao contar %s: %v", table, err)
		}
		return count
	}

	// Verifica quantidades iniciais sob o nome antigo
	docsInit := countTable("documents", "arquivo", oldName)
	ftsInit := countTable("docs_fts", "arquivo", oldName)
	tagsInit := countTable("tags", "arquivo", oldName)
	linksInit := countTable("links", "from_file", oldName)
	todosInit := countTable("todos", "file", oldName)

	if docsInit != 3 { // Geral (frontmatter), Seção 1, Seção 2
		t.Errorf("esperava 3 documentos inicialmente, obteve %d", docsInit)
	}
	if ftsInit != 3 {
		t.Errorf("esperava 3 registros FTS inicialmente, obteve %d", ftsInit)
	}
	if tagsInit != 2 {
		t.Errorf("esperava 2 tags inicialmente, obteve %d", tagsInit)
	}
	if linksInit != 1 {
		t.Errorf("esperava 1 link inicialmente, obteve %d", linksInit)
	}
	if todosInit != 1 {
		t.Errorf("esperava 1 todo inicialmente, obteve %d", todosInit)
	}

	// 2. Executa a renomeação da nota
	err = svc.Rename("nota-original", "nota-renomeada")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}

	// 3. Valida que os registros sob o nome antigo foram COMPLETAMENTE REMOVIDOS
	if store.NoteExists(oldName) {
		t.Error("a nota antiga ainda existe na tabela 'notes'")
	}
	if countTable("documents", "arquivo", oldName) != 0 {
		t.Error("sobraram documentos associados ao arquivo antigo")
	}
	if countTable("docs_fts", "arquivo", oldName) != 0 {
		t.Error("sobraram registros no FTS associados ao arquivo antigo")
	}
	if countTable("tags", "arquivo", oldName) != 0 {
		t.Error("sobraram tags associadas ao arquivo antigo")
	}
	if countTable("links", "from_file", oldName) != 0 {
		t.Error("sobraram links associados ao arquivo antigo")
	}
	if countTable("todos", "file", oldName) != 0 {
		t.Error("sobraram todos associados ao arquivo antigo")
	}

	// 4. Valida que os registros sob o nome novo foram CORRETAMENTE CRIADOS
	if !store.NoteExists(newName) {
		t.Error("a nota nova não existe na tabela 'notes'")
	}
	if countTable("documents", "arquivo", newName) != 3 {
		t.Errorf("esperava 3 documentos no arquivo novo, obteve %d", countTable("documents", "arquivo", newName))
	}
	if countTable("docs_fts", "arquivo", newName) != 3 {
		t.Errorf("esperava 3 registros FTS no arquivo novo, obteve %d", countTable("docs_fts", "arquivo", newName))
	}
	if countTable("tags", "arquivo", newName) != 2 {
		t.Errorf("esperava 2 tags no arquivo novo, obteve %d", countTable("tags", "arquivo", newName))
	}
	if countTable("links", "from_file", newName) != 1 {
		t.Errorf("esperava 1 link no arquivo novo, obteve %d", countTable("links", "from_file", newName))
	}
	if countTable("todos", "file", newName) != 1 {
		t.Errorf("esperava 1 todo no arquivo novo, obteve %d", countTable("todos", "file", newName))
	}

	// 5. Valida os totais no banco para certificar que NÃO houve duplicação acumulada
	var totalNotes, totalDocs, totalTags, totalLinks, totalTodos int
	store.DB.QueryRow("SELECT COUNT(*) FROM notes").Scan(&totalNotes)
	store.DB.QueryRow("SELECT COUNT(*) FROM documents").Scan(&totalDocs)
	store.DB.QueryRow("SELECT COUNT(*) FROM tags").Scan(&totalTags)
	store.DB.QueryRow("SELECT COUNT(*) FROM links").Scan(&totalLinks)
	store.DB.QueryRow("SELECT COUNT(*) FROM todos").Scan(&totalTodos)

	if totalNotes != 1 {
		t.Errorf("duplicação de nota! Total no banco: %d (esperava 1)", totalNotes)
	}
	if totalDocs != 3 {
		t.Errorf("duplicação de documentos! Total no banco: %d (esperava 3)", totalDocs)
	}
	if totalTags != 2 {
		t.Errorf("duplicação de tags! Total no banco: %d (esperava 2)", totalTags)
	}
	if totalLinks != 1 {
		t.Errorf("duplicação de links! Total no banco: %d (esperava 1)", totalLinks)
	}
	if totalTodos != 1 {
		t.Errorf("duplicação de todos! Total no banco: %d (esperava 1)", totalTodos)
	}

	// 6. Faz uma busca no FTS para confirmar que apenas a nota nova é retornada
	results, totalHits, err := store.SearchFTS("tarefa", 0, 10)
	if err != nil {
		t.Fatalf("SearchFTS falhou: %v", err)
	}
	if totalHits != 1 {
		t.Errorf("busca FTS retornou total de hits incorreto: %d (esperava 1)", totalHits)
	}
	if len(results) > 0 && results[0].Arquivo != newName {
		t.Errorf("busca retornou arquivo antigo ou incorreto: %q (esperava %q)", results[0].Arquivo, newName)
	}
}

