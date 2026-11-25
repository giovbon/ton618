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

type ArchiveInfo struct {
	Name      string
	Modified  string
	Size      int64
	FileCount int
}
