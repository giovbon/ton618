package domain


// NoteItem represents a note in lists
type NoteItem struct {
	Arquivo string   `json:"arquivo"`
	Tags    []string `json:"tags"`
	Type      string   `json:"type,omitempty"`
	Mtime   string   `json:"mtime"`
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

type SearchResultItem struct {
	ID        string
	Tipo      string
	Arquivo   string
	Secao     string
	Snippet   string
	Tags      []string
	RawTags   []string
	Timestamp string
	Line      int
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
