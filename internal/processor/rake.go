// Package processor provides content processing: markdown parsing, PDF extraction,
// and RAKE keyword extraction.
package processor

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"
)

// ── Custom Stopwords ──
// Os usuários podem adicionar stopwords personalizadas em um arquivo JSON
// dentro do diretório docs. O caminho padrão é: <docsDir>/stopwords-custom.json

var (
	customStopwordsMu sync.RWMutex
	customStopwords   = make(map[string]bool)
)

// stopwordsCustomFile returns the path to the custom stopwords JSON file.
func stopwordsCustomFile(docsDir string) string {
	return filepath.Join(docsDir, "stopwords-custom.json")
}

// LoadCustomStopwords carrega as stopwords personalizadas do arquivo JSON.
// Se o arquivo não existir, não é um erro — retorna nil.
func LoadCustomStopwords(docsDir string) error {
	path := stopwordsCustomFile(docsDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("ler stopwords custom: %w", err)
	}

	var words []string
	if err := json.Unmarshal(data, &words); err != nil {
		return fmt.Errorf("decodificar stopwords custom: %w", err)
	}

	customStopwordsMu.Lock()
	customStopwords = make(map[string]bool, len(words))
	for _, w := range words {
		w = strings.TrimSpace(strings.ToLower(w))
		if w != "" {
			customStopwords[w] = true
		}
	}
	customStopwordsMu.Unlock()
	return nil
}

// SaveCustomStopwords salva a lista de stopwords personalizadas no arquivo JSON.
func SaveCustomStopwords(docsDir string, words []string) error {
	// Limpa e normaliza
	clean := make([]string, 0, len(words))
	seen := make(map[string]bool)
	for _, w := range words {
		w = strings.TrimSpace(strings.ToLower(w))
		if w != "" && !seen[w] {
			clean = append(clean, w)
			seen[w] = true
		}
	}

	data, err := json.MarshalIndent(clean, "", "  ")
	if err != nil {
		return fmt.Errorf("codificar stopwords custom: %w", err)
	}

	path := stopwordsCustomFile(docsDir)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("salvar stopwords custom: %w", err)
	}

	// Atualiza o mapa em memória
	customStopwordsMu.Lock()
	customStopwords = make(map[string]bool, len(clean))
	for _, w := range clean {
		customStopwords[w] = true
	}
	customStopwordsMu.Unlock()

	return nil
}

// GetCustomStopwords retorna a lista atual de stopwords personalizadas.
func GetCustomStopwords() []string {
	customStopwordsMu.RLock()
	defer customStopwordsMu.RUnlock()
	words := make([]string, 0, len(customStopwords))
	for w := range customStopwords {
		words = append(words, w)
	}
	sort.Strings(words)
	return words
}

// AddCustomStopword adiciona uma stopword personalizada.
func AddCustomStopword(docsDir, word string) error {
	word = strings.TrimSpace(strings.ToLower(word))
	if word == "" {
		return fmt.Errorf("stopword não pode ser vazia")
	}

	// Verifica se já existe na lista padrão
	if isStopword(word) {
		// Já é stopword padrão, mas não impede de adicionar — apenas avisa
	}

	words := GetCustomStopwords()
	for _, w := range words {
		if w == word {
			return nil // já existe
		}
	}
	words = append(words, word)
	return SaveCustomStopwords(docsDir, words)
}

// RemoveCustomStopword remove uma stopword personalizada.
func RemoveCustomStopword(docsDir, word string) error {
	word = strings.TrimSpace(strings.ToLower(word))
	if word == "" {
		return nil
	}

	words := GetCustomStopwords()
	filtered := make([]string, 0, len(words))
	for _, w := range words {
		if w != word {
			filtered = append(filtered, w)
		}
	}

	// Se não mudou, não precisa salvar
	if len(filtered) == len(words) {
		return nil
	}

	return SaveCustomStopwords(docsDir, filtered)
}

// ── Stopwords em português (ISO 639-1: pt) ──
// Lista abrangente incluindo artigos, preposições, pronomes, conjunções,
// verbos auxiliares/modo comum, advérbios comuns, além de um pequeno conjunto
// de stopwords em inglês para suporte a conteúdo bilíngue.
var stopwordsPT = map[string]bool{
	// artigos + contrações
	"o": true, "a": true, "os": true, "as": true,
	"um": true, "uma": true, "uns": true, "umas": true,
	"do": true, "da": true, "dos": true, "das": true,
	"no": true, "na": true, "nos": true, "nas": true,
	"pelo": true, "pela": true, "pelos": true, "pelas": true,
	"ao": true, "aos": true, "à": true, "às": true,
	"dum": true, "duma": true, "duns": true, "dumas": true,
	"num": true, "numa": true, "nuns": true, "numas": true,
	"dele": true, "dela": true, "deles": true, "delas": true,
	"nele": true, "nela": true, "neles": true, "nelas": true,
	"deste": true, "desta": true, "destes": true, "destas": true,
	"desse": true, "dessa": true, "desses": true, "dessas": true,
	"daquele": true, "daquela": true, "daqueles": true, "daquelas": true,
	"neste": true, "nesta": true, "nestes": true, "nestas": true,
	"nesse": true, "nessa": true, "nesses": true, "nessas": true,
	"naquele": true, "naquela": true, "naqueles": true, "naquelas": true,
	"àquele": true, "àquela": true, "àqueles": true, "àquelas": true,
	"pro": true, "pra": true, "pros": true, "pras": true,
	"daí": true, "podes": true,

	// preposições
	"de": true, "em": true, "com": true, "por": true, "para": true,
	"sem": true, "sob": true, "sobre": true, "entre": true,
	"perante": true, "após": true, "hasta": true, "até": true, "contra": true,
	"desde": true, "trás": true, "ante": true,

	// pronomes pessoais / possessivos / demonstrativos
	"eu": true, "tu": true, "ele": true, "ela": true,
	"nós": true, "vós": true, "eles": true, "elas": true,
	"me": true, "te": true, "se": true, "lhe": true,
	"vos": true, "lhes": true,
	"meu": true, "minha": true, "meus": true, "minhas": true,
	"teu": true, "tua": true, "teus": true, "tuas": true,
	"seu": true, "sua": true, "seus": true, "suas": true,
	"nosso": true, "nossa": true, "nossos": true, "nossas": true,
	"vosso": true, "vossa": true, "vossos": true, "vossas": true,
	"este": true, "esta": true, "estes": true, "estas": true,
	"esse": true, "essa": true, "esses": true, "essas": true,
	"aquele": true, "aquela": true, "aqueles": true, "aquelas": true,
	"isto": true, "isso": true, "aquilo": true,
	"mesmo": true, "mesma": true, "mesmos": true, "mesmas": true,
	"outro": true, "outra": true, "outros": true, "outras": true,
	"todo": true, "toda": true, "todos": true, "todas": true,
	"muito": true, "muita": true, "muitos": true, "muitas": true,
	"pouco": true, "pouca": true, "poucos": true, "poucas": true,
	"certa": true, "certo": true, "certos": true, "certas": true,
	"algum": true, "alguma": true, "alguns": true, "algumas": true,
	"nenhum": true, "nenhuma": true, "nenhuns": true, "nenhumas": true,
	"quem": true, "que": true, "qual": true, "quais": true,
	"quanto": true, "quanta": true, "quantos": true, "quantas": true,
	"alguém": true, "qualquer": true,
	"você": true, "vocês": true, "ti": true, "mim": true,

	// conjunções
	"e": true, "ou": true, "mas": true, "porém": true,
	"contudo": true, "todavia": true, "entretanto": true,
	"pois": true, "portanto": true, "logo": true, "assim": true,
	"como": true, "quando": true, "enquanto": true,
	"embora": true, "caso": true,
	"porque": true, "causa": true,

	// verbos auxiliares / alta frequência
	"ser": true, "estar": true, "ter": true, "haver": true,
	"fazer": true, "dizer": true, "poder": true, "dar": true,
	"ficar": true, "saber": true, "querer": true, "vir": true,
	"ir": true, "ver": true, "dever": true, "passar": true,
	"achei": true, "achou": true, "acha": true, "acho": true,
	"tem": true, "têm": true, "tinha": true, "tinham": true, "tive": true,
	"era": true, "eram": true, "é": true, "são": true, "sao": true, "foi": true, "foram": true,
	"está": true, "estão": true, "estava": true, "estavam": true, "esteve": true,
	"pode": true, "podem": true, "poderia": true,
	"sera": true, "tera": true, "fara": true, "havera": true,
	"podera": true, "devera": true, "ficara": true,
	"deve": true, "devem": true, "deveria": true,
	"sendo": true, "estando": true, "tendo": true,
	"parece": true, "existe": true, "existem": true,
	"tá": true, "tava": true, "tavam": true, "vou": true, "vai": true, "vão": true, "vira": true,
	"seriam": true, "ficaram": true, "parecia": true,
	// advérbios comuns
	"não": true, "nao": true, "sim": true, "já": true, "ja": true, "mais": true,
	"menos": true,
	"bem":   true, "mal": true, "sempre": true, "nunca": true,
	"aqui": true, "ali": true, "lá": true, "cá": true,
	"agora": true, "hoje": true, "ontem": true, "amanhã": true,
	"antes": true, "depois": true, "ainda": true,
	"tão": true, "tanto": true, "quase": true, "apenas": true,
	"só": true, "so": true, "somente": true, "também": true,
	"demais": true, "demasia": true,
	"então": true, "talvez": true,

	// outras palavras funcionais comuns
	"coisa": true, "coisas": true, "gente": true, "pessoa": true, "pessoas": true,
	"vez": true, "vezes": true, "ano": true, "anos": true,
	"dia": true, "dias": true, "tempo": true,
	"cada": true,
	"tudo": true, "nada": true, "algo": true,
	"cuja": true, "cujo": true, "cujas": true, "cujos": true,
	"através": true, "conforme": true, "consoante": true,
	"durante": true, "mediante": true, "exceto": true,
	"salvo": true, "tirante": true,
	"possivel": true, "principal": true, "importante": true,
	"diferente": true, "necessario": true, "excelente": true,
	// numerais comuns (por extenso)
	"dois": true, "três": true, "quatro": true, "cinco": true,
	"seis": true, "sete": true, "oito": true, "nove": true, "dez": true,
	"cento": true, "cem": true, "mil": true,

	// gírias, contrações de chat e marcadores de discurso (EXPANSÃO COLOQUIAL)
	"mano": true, "miga": true, "cara": true, "velho": true, "bicho": true, // Vocativos comuns
	"tipo": true, "tipo assim": true, // Marcadores de hesitação / preenchimento
	"né": true, "néé": true, "néh": true, "né?": true,
	"hein": true, "uai": true, "oxe": true, "vixe": true, "pô": true,
	"beleza": true, "blz": true, "ok": true, "okay": true, "tapi": true,
	"tamo": true, "tamos": true, // Variações do "estar" cortadas
	"pqp": true, "vc": true, "vcs": true, "tb": true, "tbm": true, // Abreviações de internet
	"pq": true, "oq": true, "q": true, "kd": true, "gnt": true, "cmg": true, "ctg": true,
	"ah": true, "oh": true, "eh": true, "uh": true, "ai": true, "aí": true, // Interjeições vazias
	"entao": true, "dai": true,
	"parada": true, "paradas": true, "bagulho": true, "negócio": true, // Substitutos genéricos para "coisa"
	"ligado": true, "ligada": true, "tá ligado": true, "ta ligado": true, // Expressões de checagem de atenção
	"sabe": true, "veja": true, "olha": true, "olha só": true, "fala": true, // Verbos imperativos de discurso
	"meio": true, "meio que": true, // Atenuadores de discurso
	"bora": true, "partiu": true, "vamos": true,

	// stopwords inglesas básicas (conteúdo bilíngue)
	"the": true, "an": true, "and": true, "or": true, "mark": true,
	"of": true, "to": true, "in": true, "is": true, "it": true,
	"for": true, "on": true, "that": true, "this": true, "with": true,
	"at": true, "by": true, "be": true, "was": true, "were": true,
	"are": true, "been": true, "being": true, "am": true,
	"have": true, "has": true, "had": true, "having": true,
	"does": true, "did": true, "done": true, "doing": true,
	"will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "shall": true, "can": true,
	"from": true, "but": true,
	"about": true, "which": true, "what": true, "when": true, "where": true,
	"who": true, "whom": true, "how": true,
	"all": true, "each": true, "every": true, "both": true,
	"few": true, "more": true, "most": true, "some": true, "any": true,
	"nor": true, "none": true, "nothing": true,
	"only": true, "own": true, "same": true, "than": true, "too": true,
	"very": true, "just": true, "also": true, "well": true,
	"get": true, "got": true, "make": true, "made": true, "use": true, "used": true,
	"like": true, "such": true, "these": true, "those": true,
	"here": true, "there": true, "then": true,
	"because": true, "if": true, "else": true,
	"after": true, "before": true, "between": true, "through": true,
	"during": true, "without": true, "within": true,
	"above": true, "below": true, "over": true, "under": true,
	"again": true, "further": true, "once": true,
	"still": true, "yet": true, "while": true, "though": true,
	"much": true, "many": true, "even": true, "back": true,
	"always": true, "never": true, "often": true, "sometimes": true,
	"first": true, "last": true, "next": true, "other": true, "another": true,
	"way": true, "thing": true, "things": true, "part": true, "parts": true,

	"http": true, "https": true, "www": true, "br": true, "g1": true, "diz": true,
}

// phraseDelimiters são caracteres que separam frases candidatas.
// Inclui pontuação forte e final de linha.
var phraseDelimiters = map[rune]bool{
	'.': true, ',': true, ';': true, ':': true,
	'!': true, '?': true, '\n': true, '\r': true,
	'(': true, ')': true, '[': true, ']': true,
	'{': true, '}': true, '"': true, '\'': true,
	'—': true, '–': true, '-': true, '|': true, '/': true,
	'\\': true, '*': true, '_': true, '~': true,
}

// isStopword verifica se a palavra (minúscula) é stopword,
// considerando tanto a lista padrão quanto as personalizadas do usuário.
func isStopword(w string) bool {
	if stopwordsPT[w] {
		return true
	}
	customStopwordsMu.RLock()
	_, ok := customStopwords[w]
	customStopwordsMu.RUnlock()
	return ok
}

// isPhraseDelimiter verifica se o caractere é um delimitador de frase.
func isPhraseDelimiter(r rune) bool {
	return phraseDelimiters[r] || unicode.IsSpace(r)
}

// KeywordsCount retorna o número ideal de keywords a extrair com base no
// tamanho do texto (em caracteres Unicode):
//   - < 500 chars  → 1 keyword (nota pequena)
//   - 500-3000     → 3 keywords (nota média)
//   - > 3000       → 5 keywords (nota longa)
func KeywordsCount(text string) int {
	n := len([]rune(text))
	switch {
	case n < 500:
		return 1
	case n <= 3000:
		return 3
	default:
		return 5
	}
}

// ExtractKeywords aplica o algoritmo RAKE (Rapid Automatic Keyword Extraction)
// para extrair as N palavras/frases-chave mais relevantes do texto.
// Retorna um slice com as top N keywords, ordenadas por score decrescente.
// O texto em português é processado com stopwords específicas do idioma.
func ExtractKeywords(text string, topN int) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	// ── 1. Dividir em frases candidatas ──
	candidates := splitCandidates(text)

	// ── 2. Calcular degree e frequency de cada palavra ──
	type wordStats struct {
		frequency int
		degree    int
	}
	wordMap := make(map[string]*wordStats)

	for _, phrase := range candidates {
		words := strings.Fields(phrase)
		if len(words) == 0 {
			continue
		}
		seen := make(map[string]bool)
		for _, w := range words {
			w = cleanWord(w)
			if w == "" || isStopword(w) || len(w) < 3 || isNumeric(w) {
				continue
			}
			if wordMap[w] == nil {
				wordMap[w] = &wordStats{}
			}
			wordMap[w].frequency++
			// degree: conta co-ocorrência com outras palavras na frase
			if !seen[w] {
				wordMap[w].degree += len(words)
				seen[w] = true
			}
		}
	}

	if len(wordMap) == 0 {
		return nil
	}

	// ── 3. Calcular score de cada palavra: degree / frequency ──
	wordScores := make(map[string]float64, len(wordMap))
	for w, s := range wordMap {
		if s.frequency > 0 {
			wordScores[w] = float64(s.degree) / float64(s.frequency)
		}
	}

	// ── 4. Calcular score de cada frase candidata ──
	type scoredPhrase struct {
		phrase string
		score  float64
	}
	phraseScores := make(map[string]float64)

	for _, phrase := range candidates {
		words := strings.Fields(phrase)
		if len(words) == 0 {
			continue
		}
		// Filtra apenas palavras não-stopword
		var contentWords []string
		for _, w := range words {
			w = cleanWord(w)
			if w != "" && !isStopword(w) && len(w) >= 3 && !isNumeric(w) {
				contentWords = append(contentWords, w)
			}
		}
		if len(contentWords) == 0 {
			continue
		}

		phraseText := strings.Join(contentWords, " ")
		if prevScore, exists := phraseScores[phraseText]; exists {
			// Se já vimos essa frase, somamos os scores
			var score float64
			for _, w := range contentWords {
				if s, ok := wordScores[w]; ok {
					score += s
				}
			}
			phraseScores[phraseText] = prevScore + score
		} else {
			var score float64
			for _, w := range contentWords {
				if s, ok := wordScores[w]; ok {
					score += s
				}
			}
			phraseScores[phraseText] = score
		}
	}

	if len(phraseScores) == 0 {
		return nil
	}

	// ── 5. Ordenar por score decrescente ──
	var scored []scoredPhrase
	for phrase, score := range phraseScores {
		// Normaliza por número de palavras para evitar favorecer frases longas demais
		wordCount := len(strings.Fields(phrase))
		if wordCount > 0 {
			score = score / math.Sqrt(float64(wordCount))
		}
		scored = append(scored, scoredPhrase{phrase: phrase, score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		if math.Abs(scored[i].score-scored[j].score) < 0.0001 {
			return scored[i].phrase < scored[j].phrase
		}
		return scored[i].score > scored[j].score
	})

	// ── 6. Pegar top N (deduplicado) ──
	if topN <= 0 {
		topN = 3
	}
	seenPhrases := make(map[string]bool)
	var result []string
	for _, sp := range scored {
		if seenPhrases[sp.phrase] {
			continue
		}
		seenPhrases[sp.phrase] = true
		result = append(result, sp.phrase)
		if len(result) >= topN {
			break
		}
	}

	return result
}

// splitCandidates divide o texto em frases candidatas usando stopwords
// e delimitadores como separadores.
func splitCandidates(text string) []string {
	text = strings.ToLower(text)
	var candidates []string
	var current strings.Builder
	skipUntilDelim := false

	for _, r := range text {
		if isPhraseDelimiter(r) {
			if current.Len() > 0 {
				candidates = append(candidates, current.String())
				current.Reset()
			}
			skipUntilDelim = false
			continue
		}

		if unicode.IsSpace(r) {
			// Verifica se a palavra atual (até o espaço) é stopword
			// cleanWord normaliza acentos para que "não"→"nao" bata no mapa
			word := current.String()
			if word != "" && isStopword(cleanWord(word)) {
				// Stopword → quebra a frase candidata aqui
				candidates = append(candidates, word)
				current.Reset()
				skipUntilDelim = true
				continue
			}
			if skipUntilDelim {
				current.Reset()
				continue
			}
			current.WriteRune(r)
			continue
		}

		if skipUntilDelim {
			continue
		}
		current.WriteRune(r)
	}

	if current.Len() > 0 {
		candidates = append(candidates, current.String())
	}

	// Filtra candidatos vazios ou que são só stopwords
	var filtered []string
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		// Se a frase inteira é uma única stopword ou número, pula
		words := strings.Fields(c)
		allStopwords := true
		for _, w := range words {
			w = cleanWord(w)
			if w != "" && !isStopword(w) && !isNumeric(w) {
				allStopwords = false
				break
			}
		}
		if allStopwords {
			continue
		}
		filtered = append(filtered, c)
	}

	return filtered
}

// cleanWord remove pontuação aderente e normaliza a palavra.
func cleanWord(w string) string {
	w = strings.Trim(w, ".,;:!?\"'()[]{}<>«»-–—#@$%&*+±=§|\\/~`^")
	w = strings.ToLower(strings.TrimSpace(w))
	// Remove acentos comuns para normalização básica
	w = strings.NewReplacer(
		"á", "a", "à", "a", "â", "a", "ã", "a",
		"é", "e", "ê", "e", "è", "e",
		"í", "i", "ì", "i", "î", "i",
		"ó", "o", "ô", "o", "õ", "o", "ò", "o",
		"ú", "u", "ù", "u", "û", "u",
		"ç", "c",
		"ñ", "n",
	).Replace(w)
	return w
}

// isNumeric verifica se a string contém apenas dígitos (0-9).
// Usada para filtrar anos, números avulsos etc. que não são keywords úteis.
func isNumeric(w string) bool {
	for _, r := range w {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
