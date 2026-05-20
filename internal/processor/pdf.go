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

	// Extrai keywords (top 3 termos mais frequentes)
	keywords := ExtractKeywords(text, 3)

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

	// Retorna keywords como fileTags para aparecerem na listagem compacta
	return []Document{doc}, nil, keywords
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
	// ── Portugues (stop-words: 329 palavras) ──
	// Fonte: https://github.com/Alir3z4/stop-words/blob/master/portuguese.txt
	"a": true, "acerca": true,
	"afora": true, "agora": true,
	"algmas": true, "alguns": true,
	"ali": true, "ambos": true,
	"ante": true, "antes": true,
	"ao": true, "aos": true,
	"apontar": true, "apos": true, "após": true,
	"aquela": true, "aquelas": true, "aquele": true, "aqueles": true,
	"aqui": true, "aquilo": true,
	"as": true,
	"ate": true, "até": true,
	"atras": true, "atrás": true,
	"bem": true, "bom": true,
	"cada": true, "caminho": true,
	"cara": true, "cima": true,
	"com": true, "como": true,
	"comprido": true, "conhecido": true,
	"connosco": true, "consoante": true,
	"contra": true, "corrente": true,
	"da": true, "das": true,
	"de": true, "debaixo": true,
	"dela": true, "delas": true, "dele": true, "deles": true,
	"dentro": true, "depois": true,
	"desde": true, "desligado": true,
	"deve": true, "devem": true, "devera": true,
	"diante": true, "direita": true,
	"diz": true, "dizer": true,
	"do": true, "dois": true, "dos": true,
	"durante": true,
	"e": true,
	"ela": true, "elas": true, "ele": true, "eles": true,
	"em": true, "enquanto": true,
	"entao": true,
	"entre": true,
	"era": true, "eram": true, "eramos": true,
	"escontra": true,
	"essa": true, "essas": true, "esse": true, "esses": true,
	"esta": true, "estado": true, "estamos": true,
	"estas": true,
	"estava": true, "estavam": true, "estavamos": true,
	"este": true,
	"esteja": true, "estejam": true, "estejamos": true,
	"estes": true,
	"esteve": true,
	"estive": true, "estivemos": true,
	"estiver": true, "estivera": true, "estiveram": true,
	"estiveramos": true,
	"estiverem": true, "estivermos": true,
	"estivesse": true, "estivessem": true,
	"estivessemos": true,
	"estou": true,
	"eu": true,
	"excepto": true, "exceto": true,
	"fara": true,
	"faz": true, "fazer": true,
	"fazia": true, "fez": true,
	"fim": true,
	"foi": true, "fomos": true,
	"for": true, "fora": true, "foram": true,
	"foramos": true,
	"forem": true, "formos": true,
	"fosse": true, "fossem": true,
	"fossemos": true,
	"fui": true,
	"haja": true, "hajam": true, "hajamos": true,
	"havemos": true, "havia": true,
	"hei": true,
	"horas": true,
	"houve": true, "houvemos": true,
	"houver": true, "houvera": true,
	"houveram": true, "houveramos": true,
	"houverei": true,
	"houverem": true, "houveremos": true,
	"houveria": true, "houveriam": true,
	"houveriamos": true,
	"houvermos": true,
	"houvesse": true, "houvessem": true,
	"houvessemos": true,
	"in": true, "iniciar": true, "inicio": true,
	"inte": true,
	"ir": true, "ira": true,
	"isso": true, "ista": true, "iste": true, "isto": true,
	"lhe": true, "lhes": true,
	"ligado": true,
	"maioria": true, "maiorias": true,
	"mais": true, "malgrado": true,
	"mas": true, "me": true,
	"mediante": true, "menos": true,
	"mesmo": true, "meu": true, "meus": true,
	"minha": true, "minhas": true,
	"muito": true, "muitos": true,
	"na": true, "nas": true, "nem": true,
	"no": true, "nome": true,
	"nos": true, "nossa": true, "nossas": true,
	"nosso": true, "nossos": true,
	"novo": true,
	"num": true, "numa": true,
	"nunca": true,
	"o": true, "onde": true, "os": true,
	"ou": true, "outro": true,
	"para": true, "parte": true,
	"pegar": true,
	"pela": true, "pelas": true, "pelo": true, "pelos": true,
	"per": true, "pera": true, "perante": true,
	"pode": true, "podera": true,
	"podia": true,
	"por": true, "porque": true, "povo": true,
	"pra": true,
	"promeiro": true,
	"qual": true, "qualquer": true,
	"quando": true,
	"que": true,
	"quem": true, "quieto": true,
	"saber": true, "salvante": true, "salvo": true,
	"se": true,
	"segundo": true,
	"seja": true, "sejam": true, "sejamos": true,
	"sem": true,
	"ser": true,
	"serei": true, "seremos": true,
	"seria": true, "seriam": true, "seriamos": true,
	"seu": true, "seus": true,
	"sob": true, "sobre": true,
	"somente": true, "somos": true,
	"sou": true,
	"sua": true, "suas": true, "suso": true,
	"tal": true,
	"te": true,
	"temos": true, "tempo": true,
	"tenha": true, "tenham": true, "tenhamos": true,
	"tenho": true,
	"tentar": true, "tentaram": true,
	"tente": true, "tentei": true,
	"ter": true,
	"terei": true, "teremos": true,
	"teria": true, "teriam": true, "teriamos": true,
	"teu": true, "teus": true,
	"teve": true,
	"tinha": true, "tinham": true, "tinhamos": true,
	"tipo": true, "tirante": true,
	"tive": true, "tivemos": true,
	"tiver": true, "tivera": true, "tiveram": true,
	"tiveramos": true,
	"tiverem": true, "tivermos": true,
	"tivesse": true, "tivessem": true,
	"tivessemos": true,
	"todos": true,
	"trabalhar": true, "trabalho": true,
	"tras": true,
	"tu": true, "tua": true, "tuas": true,
	"ultimo": true,
	"um": true, "uma": true, "umas": true, "uns": true,
	"usa": true, "usar": true,
	"valor": true,
	"veja": true, "ver": true,
	"verdade": true, "verdadeiro": true,
	"você": true, "vocês": true, "vos": true,
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
	"an": true, "the": true,
	"and": true, "but": true, "if": true, "or": true, "because": true,
	"until": true, "while": true,
	"of": true, "at": true, "by": true, "with": true,
	"about": true, "against": true, "between": true, "into": true,
	"through": true, "during": true, "before": true, "after": true,
	"above": true, "below": true, "from": true,
	"up": true, "down": true, "out": true,
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
