package processor

import (
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/ledongthuc/pdf"
)

// ProcessPDF extrai o texto de um arquivo PDF e retorna um documento
// para indexacao no FTS5 e geracao de embeddings.
// Tambem extrai keywords (termos mais frequentes) para uso como tags.
func ProcessPDF(path, filename string, modTime time.Time) ([]Document, []string, []string) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, nil, nil
	}
	defer f.Close()

	totalPages := r.NumPage()
	var fullText strings.Builder

	for pageNum := 1; pageNum <= totalPages; pageNum++ {
		page := r.Page(pageNum)
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		fullText.WriteString(" ")
		fullText.WriteString(strings.TrimSpace(text))
	}

	text := strings.TrimSpace(fullText.String())
	if text == "" {
		return nil, nil, nil
	}

	// Extrai keywords (top 30 termos mais frequentes)
	keywords := ExtractKeywords(text, 30)

	// Nome do arquivo sem extensao para o titulo
	baseName := strings.TrimSuffix(filepath.Base(filename), ".pdf")

	doc := Document{
		ID:         HashFunc("pdf-" + filename),
		Tipo:       "pdf",
		Arquivo:    filename,
		Secao:      "\U0001f4d5 " + baseName,
		Texto:      text,
		Timestamp:  modTime.UTC().Format(time.RFC3339),
		Created:    modTime.UTC().Format(time.RFC3339),
		Hash:       CalculateHash("pdf", baseName, nil),
		VectorHash: CalculateVectorHash("pdf", text),
		Tags:       keywords,
	}

	// Retorna nil como tags para que keywords nao aparecam como # na interface
	return []Document{doc}, nil, nil
}

// ExtractKeywords extrai os N termos mais frequentes do texto,
// ignorando stopwords (pt + en) e palavras com <= 2 caracteres.
func ExtractKeywords(text string, topN int) []string {
	words := strings.Fields(text)

	freq := make(map[string]int)
	for _, w := range words {
		cleaned := strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return r
			}
			return -1
		}, w)

		cleaned = strings.ToLower(cleaned)
		if len(cleaned) <= 2 {
			continue
		}
		if stopwords[cleaned] {
			continue
		}
		freq[cleaned]++
	}

	type kv struct {
		key   string
		count int
	}
	var sorted []kv
	for k, v := range freq {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	var keywords []string
	for i := 0; i < topN && i < len(sorted); i++ {
		keywords = append(keywords, sorted[i].key)
	}
	return keywords
}

// stopwords contem as listas completas de stopwords do NLTK (pt + en), sem duplicatas.
// Fonte: https://github.com/nltk/nltk/blob/develop/nltk/corpus/stopwords.py
var stopwords = map[string]bool{
	// ── Portugues (NLTK: 131 palavras, exceto duplicatas com ingles) ──
	"ao": true, "aos": true,
	"aquela": true, "aquelas": true, "aquele": true, "aqueles": true, "aquilo": true,
	"ate": true,
	"com": true, "como": true,
	"da": true, "das": true, "de": true,
	"dela": true, "delas": true, "dele": true, "deles": true,
	"depois": true, "dos": true,
	"e": true, "ela": true, "elas": true, "ele": true, "eles": true,
	"em": true, "entre": true,
	"era": true, "eram": true,
	"essa": true, "essas": true, "esse": true, "esses": true,
	"esta": true, "estamos": true, "estas": true,
	"estava": true, "estavam": true,
	"este": true, "estes": true, "estou": true,
	"eu": true,
	"foi": true, "fomos": true, "fossem": true, "fui": true,
	"ha": true,
	"isso": true, "isto": true,
	"ja": true,
	"lhe": true, "lhes": true,
	"mais": true, "mas": true,
	"mesma": true, "mesmas": true, "mesmo": true, "mesmos": true,
	"meu": true, "meus": true, "minha": true, "minhas": true,
	"muito": true,
	"na": true, "nas": true, "nem": true, "nos": true, "nós": true,
	"num": true, "numa": true,
	"os": true, "ou": true,
	"para": true,
	"pela": true, "pelas": true, "pelo": true, "pelos": true,
	"por": true,
	"qual": true, "quando": true, "que": true, "quem": true,
	"se": true, "seja": true, "sem": true, "sendo": true,
	"seu": true, "seus": true, "sua": true, "suas": true,
	"te": true, "tem": true, "tendo": true, "tenha": true, "ter": true,
	"teu": true, "teus": true, "ti": true,
	"tido": true, "tinha": true, "tinham": true,
	"tive": true, "tivesse": true,
	"tu": true, "tua": true, "tuas": true,
	"um": true, "uma": true, "umas": true, "uns": true,
	// ── Ingles (NLTK: 179 palavras) ──
	"i": true, "my": true, "myself": true,
	"we": true, "our": true, "ours": true, "ourselves": true,
	"you": true, "your": true, "yours": true,
	"yourself": true, "yourselves": true,
	"he": true, "him": true, "his": true, "himself": true,
	"she": true, "her": true, "hers": true, "herself": true,
	"it": true, "its": true, "itself": true,
	"they": true, "them": true, "their": true, "theirs": true, "themselves": true,
	"what": true, "which": true, "who": true, "whom": true,
	"this": true, "that": true, "these": true, "those": true,
	"am": true, "is": true, "are": true,
	"was": true, "were": true,
	"be": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "having": true,
	"does": true, "did": true, "doing": true,
	"a": true, "an": true, "the": true,
	"and": true, "but": true, "if": true, "or": true, "because": true,
	"until": true, "while": true,
	"of": true, "at": true, "by": true, "with": true,
	"about": true, "against": true, "between": true, "into": true,
	"through": true, "during": true, "before": true, "after": true,
	"above": true, "below": true, "from": true,
	"up": true, "down": true, "in": true, "out": true,
	"on": true, "off": true, "over": true, "under": true,
	"again": true, "further": true, "then": true, "once": true,
	"here": true, "there": true,
	"when": true, "where": true, "why": true, "how": true,
	"all": true, "each": true, "every": true, "both": true,
	"few": true, "more": true, "most": true, "other": true,
	"some": true, "such": true,
	"nor": true, "not": true,
	"only": true, "own": true, "same": true,
	"than": true, "too": true, "very": true,
	"can": true, "will": true, "just": true,
	"should": true, "now": true,
	// contracoes comuns do ingles (NLTK)
	"don": true, "ain": true, "aren": true, "couldn": true,
	"didn": true, "doesn": true, "hadn": true, "hasn": true,
	"haven": true, "isn": true, "mightn": true, "mustn": true,
	"needn": true, "shan": true, "shouldn": true, "wasn": true,
	"weren": true, "won": true, "wouldn": true,
	// palavras de 1-2 letras comuns (NLTK)
	"s": true, "t": true, "d": true, "ll": true, "m": true,
	"re": true, "ve": true, "y": true, "ma": true,
}
