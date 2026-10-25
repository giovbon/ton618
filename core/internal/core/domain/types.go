package domain

// NoteItem represents a note in lists
type NoteItem struct {
	Arquivo string   `json:"arquivo"`
	Tags    []string `json:"tags"`
	Type    string   `json:"type,omitempty"`
	Mtime   string   `json:"mtime"`

	// ChildrenCount é o número de notas que declaram esta como pai (`pai:` no
	// frontmatter). Só é populado nos handlers que precisam exibir a hierarquia
	// (ex: sidebar) — GetMany não lê o conteúdo das notas, então deixa 0.
	ChildrenCount int `json:"children_count,omitempty"`
}

// BacklinkItem represents a single link to a note
type BacklinkItem struct {
	SourceFile string
	Context    string
	IsDirect   bool
}

// BacklinksResult holds direct and indirect backlinks
type BacklinksResult struct {
	Level1 []string
	Level2 []string
}

// SearchSnippet é um fragmento ADICIONAL da mesma nota que também casou na
// busca (ex.: outra linha da planilha Tabulator, outra seção do markdown),
// com o rótulo da seção e o HTML já destacado.
type SearchSnippet struct {
	Secao string
	HTML  string
}

type SearchResultItem struct {
	ID         string
	Tipo       string
	Arquivo    string
	Secao      string
	Snippet    string
	Snippets   []SearchSnippet // demais trechos da mesma nota (além do principal)
	ExtraCount int             // ocorrências restantes fora do teto de exibição
	Tags       []string
	RawTags    []string
	Timestamp  string
	Line       int
}

type ArchiveInfo struct {
	Name      string
	Modified  string
	Size      int64
	FileCount int
}

type SearchResultsData struct {
	Query   string
	Total   int
	Results []SearchResultItem
}

// DayActivity represents one activity in the owner's daily routine (Rotina).
//
// O TON-618 é de uso pessoal: não existe coluna de usuário/dono — toda a tabela
// pertence ao dono do cofre (a única credencial do BasicAuth). O guard de
// schema em internal/core/db garante isso.
type DayActivity struct {
	ID          string `json:"id"`
	Day         string `json:"date"` // YYYY-MM-DD
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// Appointment represents a task or appointment
type Appointment struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	EventDate   string `json:"event_date"`
	Year        int    `json:"year"`
	Month       int    `json:"month"`
	WeekNumber  int    `json:"week_number"`
	CreatedAt   string `json:"created_at"`
}
