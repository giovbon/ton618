// Package domain contém os tipos centrais do domínio TON-618.
// Estes tipos são independentes de storage, HTTP ou qualquer infraestrutura externa.
package domain

import "time"

// Document representa um fragmento indexável (seção de nota, página de PDF, etc).
type Document struct {
	ID        string
	Type      string
	File      string   // caminho relativo (ex: "notes/foo.md", "pdfs/bar.pdf")
	Section   string
	Text      string
	Tags      []string
	Page      int
	Order     int
	Timestamp time.Time
	CreatedAt time.Time
	Hash      string
}

// Note representa uma nota markdown armazenada no banco.
type Note struct {
	Filename string
	Content  string
	Mtime    time.Time
}

// FileMod representa o registro de modificação de um arquivo indexado.
type FileMod struct {
	File  string
	Mtime time.Time
}

// FTSResult é o resultado bruto de uma busca FTS5.
type FTSResult struct {
	DocID   string
	Type    string
	File    string
	Section string
	Text    string
	Tags    string
	Rank    float64
}

// SearchHit é um resultado de busca enriquecido com score e highlight.
type SearchHit struct {
	ID           string
	Score        float64
	FinalScore   float64
	ScoreDetails map[string]float64
	Highlight    map[string][]string
	Doc          Document
}

// SearchResults contém os hits e o total de uma busca.
type SearchResults struct {
	Hits  []SearchHit
	Total int
}

// Link representa um wikilink entre dois arquivos.
type Link struct {
	From string
	To   string
}

// ArchiveInfo descreve um arquivo ZIP de backup/arquivo morto.
type ArchiveInfo struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Modified  string `json:"modified"`
	FileCount int    `json:"file_count"`
}

// NoteItem é usado na listagem compacta de notas (JSON).
type NoteItem struct {
	File  string   `json:"arquivo"`
	Tags  []string `json:"tags"`
	Mtime string   `json:"mtime"`
}
