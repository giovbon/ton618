package models

// Subpastas monitoradas
var MonitoredSubDirs = []string{"notes", "links", "voice"}

// Configurações de Seção
const (
	SectionImages  = "Anexos / Imagens"
	SectionDefault = "Geral"
)

// Extensões suportadas
var (
	ExtMarkdown = []string{".md"}
	ExtPDF      = []string{".pdf"}
	ExtImage    = []string{".png", ".jpg", ".jpeg"}
)

// Heurísticas de Ranking
var Stopwords = map[string]bool{
	"de": true, "da": true, "do": true, "em": true, "no": true, "na": true,
	"um": true, "uma": true, "os": true, "as": true, "com": true, "por": true,
	"para": true, "que": true, "seu": true, "sua": true, "dos": true, "das": true,
	"pelo": true, "pela": true, "nos": true, "nas": true, "isto": true, "isso": true,
}

var GenericTitles = map[string]bool{
	"introdução": true, "introducao": true, "resumo": true, "conclusão": true,
	"conclusao": true, "regras": true, "objetivo": true, "links": true,
	"referências": true, "referencias": true,
}
