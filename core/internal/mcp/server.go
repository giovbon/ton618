// Package mcp expõe o acervo do TON-618 como um servidor MCP (Model Context
// Protocol) SOMENTE LEITURA.
//
// O objetivo é dar a qualquer cliente de IA (OpenCode, Claude Desktop, Cursor,
// etc.) acesso às notas sem expor a API HTTP do app — que usa uma senha única
// com poder de escrita/restauração. Aqui o processo é iniciado pelo cliente,
// fala stdio e abre o SQLite em mode=ro (ver db.NewReaderStore).
//
// A lógica de negócio NÃO é duplicada: busca usa search.Search, consultas usam
// o mesmo query.Engine da linguagem ```consulta e listagem/backlinks usam o
// NoteService.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"ton618/core/internal/core/db"
	"ton618/core/internal/features/notes"
	"ton618/core/internal/query"
)

const serverInstructions = `Servidor de LEITURA do acervo pessoal TON-618 (notas em SQLite + FTS5).

- Arquivos são identificados por caminhos como "notes/nome.md". As ferramentas
  aceitam "nome" e normalizam para "notes/nome.md".
- Ferramentas: search_notes (busca global rankeada), get_note (conteúdo
  completo), list_notes (metadados), query_notes (linguagem de consulta) e
  backlinks (links de entrada/saída).
- search_notes entende texto livre e os filtros da busca do app:
  tags:tag, tipo:mapa_mental, tipo:desenho, tipo:tabulator, tipo:semanal, etc.
  Vários termos são combinados; use aspas para uma expressão.
- query_notes recebe o YAML de um bloco ` + "```consulta" + `:
  tipo: notas | tarefas | hierarquia; filtros texto:, tags:, sem-tags:, pasta:,
  onde: (expressão CEL), ordenar: campo[:asc|desc], limite:, colunas:.
  É a mesma linguagem documentada em docs/linguagem-de-consulta.md.
- O campo tipo: do frontmatter usa o vocabulário PT: nota, desenho, mapa_mental,
  tabulator, semanal, pdf, anexo, artigo, captura. A hierarquia é declarada em
  pai: (nome ou [[wikilink]]).
- O índice FTS não indexa o CORPO de desenhos, mapas mentais e tabelas
  Tabulator. Para esses tipos, encontre a nota por título/tag e leia com
  get_note.
- Nada aqui escreve, renomeia ou apaga: o banco é aberto em modo somente
  leitura.`

// Server agrupa as dependências das ferramentas MCP.
type Server struct {
	store  *db.Store
	notes  *notes.NoteService
	engine *query.Engine
}

// NewServer cria o servidor MCP sobre um Store já aberto (de preferência com
// db.NewReaderStore). docsDir é repassado ao NoteService por paridade com o
// servidor HTTP; os caminhos de leitura não tocam o disco.
func NewServer(store *db.Store, docsDir string) *Server {
	return &Server{
		store:  store,
		notes:  notes.NewNoteService(store, store, store, store, store, store, docsDir),
		engine: query.NewEngine(),
	}
}

// SDK monta o servidor do SDK com as ferramentas registradas. Exposto para os
// testes usarem transportes em memória; produção usa Serve.
func (s *Server) SDK() *sdk.Server {
	srv := sdk.NewServer(
		&sdk.Implementation{Name: "ton618-notes", Title: "TON-618 Notes", Version: "1.0.0"},
		&sdk.ServerOptions{Instructions: serverInstructions},
	)
	s.registerTools(srv)
	return srv
}

// Serve roda o servidor em stdio até o contexto ser cancelado (ou o cliente
// fechar o pipe). Nada é escrito em stdout além do protocolo.
func (s *Server) Serve(ctx context.Context) error {
	return s.SDK().Run(ctx, &sdk.StdioTransport{})
}

// ── Helpers de resultado ──────────────────────────────────────────────────

// reply serializa v como JSON indentado — formato estável e fácil de o modelo
// consumir sem precisar adivinhar separadores.
func reply(v any) *sdk.CallToolResult {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return toolError("não foi possível serializar o resultado: %v", err)
	}
	return &sdk.CallToolResult{
		Content: []sdk.Content{&sdk.TextContent{Text: string(b)}},
	}
}

func textReply(text string) *sdk.CallToolResult {
	return &sdk.CallToolResult{
		Content: []sdk.Content{&sdk.TextContent{Text: text}},
	}
}

func toolError(format string, args ...any) *sdk.CallToolResult {
	return &sdk.CallToolResult{
		IsError: true,
		Content: []sdk.Content{&sdk.TextContent{Text: fmt.Sprintf(format, args...)}},
	}
}
