package clustering

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/blevesearch/bleve/v2"
)

var stopwordMap = map[string]bool{
	// Português (ISO)
	"a": true, "acerca": true, "adeus": true, "agora": true, "ainda": true, "alem": true, "algmas": true, "algo": true, "algumas": true, "alguns": true, "ali": true, "além": true, "ambas": true, "ambos": true, "ano": true, "anos": true, "antes": true, "ao": true, "aonde": true, "aos": true, "apenas": true, "apoio": true, "apontar": true, "apos": true, "após": true, "aquela": true, "aquelas": true, "aquele": true, "aqueles": true, "aqui": true, "aquilo": true, "as": true, "assim": true, "através": true, "atrás": true, "até": true, "aí": true, "baixo": true, "bastante": true, "bem": true, "boa": true, "boas": true, "bom": true, "bons": true, "breve": true, "cada": true, "caminho": true, "catorze": true, "cedo": true, "cento": true, "certamente": true, "certeza": true, "cima": true, "cinco": true, "coisa": true, "com": true, "como": true, "comprido": true, "conhecido": true, "conselho": true, "contra": true, "contudo": true, "corrente": true, "cuja": true, "cujas": true, "cujo": true, "cujos": true, "custa": true, "cá": true, "da": true, "daquela": true, "daquelas": true, "daquele": true, "daqueles": true, "dar": true, "das": true, "de": true, "debaixo": true, "dela": true, "delas": true, "dele": true, "deles": true, "demais": true, "dentro": true, "depois": true, "desde": true, "desligado": true, "dessa": true, "dessas": true, "desse": true, "desses": true, "desta": true, "destas": true, "deste": true, "destes": true, "deve": true, "devem": true, "deverá": true, "dez": true, "dezanove": true, "dezasseis": true, "dezassete": true, "dezoito": true, "dia": true, "diante": true, "direita": true, "dispoe": true, "dispoem": true, "diversa": true, "diversas": true, "diversos": true, "diz": true, "dizem": true, "dizer": true, "do": true, "dois": true, "dos": true, "doze": true, "duas": true, "durante": true, "dá": true, "dão": true, "dúvida": true, "e": true, "ela": true, "elas": true, "ele": true, "eles": true, "em": true, "embora": true, "enquanto": true, "entao": true, "entre": true, "então": true, "era": true, "eram": true, "essa": true, "essas": true, "esse": true, "esses": true, "esta": true, "estado": true, "estamos": true, "estar": true, "estará": true, "estas": true, "estava": true, "estavam": true, "este": true, "esteja": true, "estejam": true, "estejamos": true, "estes": true, "esteve": true, "estive": true, "estivemos": true, "estiver": true, "estivera": true, "estiveram": true, "estiverem": true, "estivermos": true, "estivesse": true, "estivessem": true, "estiveste": true, "estivestes": true, "estivéramos": true, "estivéssemos": true, "estou": true, "está": true, "estás": true, "estávamos": true, "estão": true, "eu": true, "exemplo": true, "falta": true, "fará": true, "favor": true, "faz": true, "fazeis": true, "fazem": true, "fazemos": true, "fazer": true, "fazes": true, "fazia": true, "faço": true, "fez": true, "fim": true, "final": true, "foi": true, "fomos": true, "for": true, "fora": true, "foram": true, "forem": true, "forma": true, "formos": true, "fosse": true, "fossem": true, "foste": true, "fostes": true, "fui": true, "fôramos": true, "fôssemos": true, "geral": true, "grande": true, "grandes": true, "grupo": true, "ha": true, "haja": true, "hajam": true, "hajamos": true, "havemos": true, "havia": true, "hei": true, "hoje": true, "hora": true, "horas": true, "houve": true, "houvemos": true, "houver": true, "houvera": true, "houveram": true, "houverei": true, "houverem": true, "houveremos": true, "houveria": true, "houveriam": true, "houvermos": true, "houverá": true, "houverão": true, "houveríamos": true, "houvesse": true, "houvessem": true, "houvéramos": true, "houvéssemos": true, "há": true, "hão": true, "http": true, "https": true, "html": true, "index": true, "iniciar": true, "inicio": true, "ir": true, "irá": true, "isso": true, "ista": true, "iste": true, "isto": true, "já": true, "lado": true, "lhe": true, "lhes": true, "ligado": true, "local": true, "logo": true, "longe": true, "lugar": true, "lá": true, "maior": true, "maioria": true, "maiorias": true, "mais": true, "mal": true, "mas": true, "me": true, "mediante": true, "meio": true, "menor": true, "menos": true, "meses": true, "mesma": true, "mesmas": true, "mesmo": true, "mesmos": true, "meu": true, "meus": true, "mil": true, "minha": true, "minhas": true, "momento": true, "muito": true, "muitos": true, "máximo": true, "mês": true, "na": true, "nada": true, "nao": true, "naquela": true, "naquelas": true, "naquele": true, "naqueles": true, "nas": true, "nem": true, "nenhuma": true, "nessa": true, "nessas": true, "nesse": true, "nesses": true, "nesta": true, "nestas": true, "neste": true, "nestes": true, "no": true, "noite": true, "nome": true, "nos": true, "nossa": true, "nossas": true, "nosso": true, "nossos": true, "nova": true, "novas": true, "nove": true, "novo": true, "novos": true, "num": true, "numa": true, "numas": true, "nunca": true, "nuns": true, "não": true, "nível": true, "nós": true, "número": true, "o": true, "obra": true, "obrigada": true, "obrigado": true, "oitava": true, "oitavo": true, "oito": true, "onde": true, "ontem": true, "onze": true, "os": true, "ou": true, "outra": true, "outras": true, "outro": true, "outros": true, "para": true, "parece": true, "parte": true, "partir": true, "paucas": true, "pegar": true, "pela": true, "pelas": true, "pelo": true, "pelos": true, "perante": true, "perto": true, "pessoas": true, "pode": true, "podem": true, "poder": true, "poderá": true, "podia": true, "pois": true, "ponto": true, "pontos": true, "por": true, "porque": true, "porquê": true, "portanto": true, "posição": true, "possivelmente": true, "posso": true, "possível": true, "pouca": true, "pouco": true, "poucos": true, "povo": true, "primeira": true, "primeiras": true, "primeiro": true, "primeiros": true, "promeiro": true, "propios": true, "proprio": true, "própria": true, "próprias": true, "próprio": true, "próprios": true, "próxima": true, "próximas": true, "próximo": true, "próximos": true, "puderam": true, "pôde": true, "põe": true, "põem": true, "quais": true, "qual": true, "qualquer": true, "quando": true, "quanto": true, "quarta": true, "quarto": true, "quatro": true, "que": true, "quem": true, "quer": true, "quereis": true, "querem": true, "queremas": true, "queres": true, "quero": true, "questão": true, "quieto": true, "quinta": true, "quinto": true, "quinze": true, "quáis": true, "quê": true, "relação": true, "sabe": true, "sabem": true, "saber": true, "se": true, "segunda": true, "segundo": true, "sei": true, "seis": true, "seja": true, "sejam": true, "sejamos": true, "sem": true, "sempre": true, "sendo": true, "ser": true, "serei": true, "seremos": true, "seria": true, "seriam": true, "será": true, "serão": true, "seríamos": true, "sete": true, "seu": true, "seus": true, "sexta": true, "sexto": true, "sim": true, "sistema": true, "sob": true, "sobre": true, "sois": true, "somente": true, "somos": true, "sou": true, "sua": true, "suas": true, "são": true, "sétima": true, "sétimo": true, "só": true, "tal": true, "talvez": true, "tambem": true, "também": true, "tanta": true, "tantas": true, "tanto": true, "tarde": true, "te": true, "tem": true, "temos": true, "tempo": true, "tendes": true, "tenha": true, "tenham": true, "tenhamos": true, "tenho": true, "tens": true, "tentar": true, "tentaram": true, "tente": true, "tentei": true, "ter": true, "terceira": true, "terceiro": true, "terei": true, "teremos": true, "teria": true, "teriam": true, "terá": true, "terão": true, "teríamos": true, "teu": true, "teus": true, "teve": true, "tinha": true, "tinham": true, "tipo": true, "tive": true, "tivemos": true, "tiver": true, "tivera": true, "tiveram": true, "tiverem": true, "tivermos": true, "tivesse": true, "tivessem": true, "tiveste": true, "tivestes": true, "tivéramos": true, "tivéssemos": true, "toda": true, "todas": true, "todo": true, "todos": true, "trabalhar": true, "trabalho": true, "treze": true, "três": true, "tu": true, "tua": true, "tuas": true, "tudo": true, "tão": true, "tém": true, "têm": true, "tínhamos": true, "um": true, "uma": true, "umas": true, "uns": true, "usa": true, "usar": true, "vai": true, "vais": true, "valor": true, "veja": true, "vem": true, "vens": true, "ver": true, "verdade": true, "verdadeiro": true, "vez": true, "vezes": true, "viagem": true, "vindo": true, "vinte": true, "você": true, "vocês": true, "vos": true, "vossa": true, "vossas": true, "vosso": true, "vossos": true, "vários": true, "vão": true, "vêm": true, "vós": true, "zero": true, "à": true, "às": true, "área": true, "é": true, "éramos": true, "és": true, "último": true,

	// Inglês (ISO)
	"about": true, "above": true, "across": true, "after": true, "afterwards": true, "again": true, "against": true, "all": true, "almost": true, "alone": true, "along": true, "already": true, "also": true, "although": true, "always": true, "am": true, "among": true, "amongst": true, "amoungst": true, "amount": true, "an": true, "and": true, "another": true, "any": true, "anyhow": true, "anyone": true, "anything": true, "anyway": true, "anywhere": true, "are": true, "around": true, "at": true, "back": true, "be": true, "became": true, "because": true, "become": true, "becomes": true, "becoming": true, "been": true, "before": true, "beforehand": true, "behind": true, "being": true, "below": true, "beside": true, "besides": true, "between": true, "beyond": true, "bill": true, "both": true, "bottom": true, "but": true, "by": true, "call": true, "can": true, "cannot": true, "cant": true, "co": true, "con": true, "could": true, "couldnt": true, "cry": true, "describe": true, "detail": true, "done": true, "down": true, "due": true, "during": true, "each": true, "eg": true, "eight": true, "either": true, "eleven": true, "else": true, "elsewhere": true, "empty": true, "enough": true, "etc": true, "even": true, "ever": true, "every": true, "everyone": true, "everything": true, "everywhere": true, "except": true, "few": true, "fifteen": true, "fify": true, "fill": true, "find": true, "fire": true, "first": true, "five": true, "former": true, "formerly": true, "forty": true, "found": true, "four": true, "from": true, "front": true, "full": true, "further": true, "get": true, "give": true, "go": true, "had": true, "has": true, "hasnt": true, "have": true, "he": true, "hence": true, "her": true, "here": true, "hereafter": true, "hereby": true, "herein": true, "hereupon": true, "hers": true, "herself": true, "him": true, "himself": true, "his": true, "how": true, "however": true, "hundred": true, "i": true, "ie": true, "if": true, "in": true, "inc": true, "indeed": true, "interest": true, "into": true, "is": true, "it": true, "its": true, "itself": true, "keep": true, "last": true, "latter": true, "latterly": true, "least": true, "less": true, "ltd": true, "made": true, "many": true, "may": true, "meanwhile": true, "might": true, "mill": true, "mine": true, "more": true, "moreover": true, "most": true, "mostly": true, "move": true, "much": true, "must": true, "my": true, "myself": true, "name": true, "namely": true, "neither": true, "never": true, "nevertheless": true, "next": true, "nine": true, "nobody": true, "none": true, "noone": true, "nor": true, "not": true, "nothing": true, "now": true, "nowhere": true, "of": true, "off": true, "often": true, "on": true, "once": true, "one": true, "only": true, "onto": true, "or": true, "other": true, "others": true, "otherwise": true, "our": true, "ours": true, "ourselves": true, "out": true, "over": true, "own": true, "part": true, "per": true, "perhaps": true, "please": true, "put": true, "rather": true, "re": true, "same": true, "see": true, "seem": true, "seemed": true, "seeming": true, "seems": true, "serious": true, "several": true, "she": true, "should": true, "show": true, "side": true, "since": true, "sincere": true, "six": true, "sixty": true, "so": true, "some": true, "somehow": true, "someone": true, "something": true, "sometime": true, "sometimes": true, "somewhere": true, "still": true, "such": true, "system": true, "take": true, "ten": true, "than": true, "that": true, "the": true, "their": true, "them": true, "themselves": true, "then": true, "thence": true, "there": true, "thereafter": true, "thereby": true, "therefore": true, "therein": true, "thereupon": true, "these": true, "they": true, "thick": true, "thin": true, "third": true, "this": true, "those": true, "though": true, "three": true, "through": true, "throughout": true, "thru": true, "thus": true, "to": true, "together": true, "too": true, "top": true, "toward": true, "towards": true, "twelve": true, "twenty": true, "two": true, "un": true, "under": true, "until": true, "up": true, "upon": true, "us": true, "very": true, "via": true, "was": true, "we": true, "well": true, "were": true, "what": true, "whatever": true, "when": true, "whence": true, "whenever": true, "where": true, "whereafter": true, "whereas": true, "whereby": true, "wherein": true, "whereupon": true, "wherever": true, "whether": true, "which": true, "while": true, "whither": true, "who": true, "whoever": true, "whole": true, "whom": true, "whose": true, "why": true, "will": true, "with": true, "within": true, "without": true, "would": true, "yet": true, "you": true, "your": true, "yours": true, "yourself": true, "yourselves": true,
}

// GenerateClusterLabel usa o Dicionário de Termos do Bleve (BM25 Lite) para batizar a ilha.
// Retorna o label principal (1-2 palavras) e a lista de keywords principais (top 5).
func GenerateClusterLabel(texts []string, index bleve.Index) (string, []string) {
	if len(texts) == 0 {
		return "Sem Nome", []string{}
	}

	// 1. Abrir Leitor do Índice via interface Advanced
	idxInternal, err := index.Advanced()
	if err != nil {
		return "Geral", []string{}
	}

	reader, err := idxInternal.Reader()
	if err != nil {
		return "Geral", []string{}
	}
	defer reader.Close()

	totalDocs, _ := reader.DocCount()
	if totalDocs == 0 {
		totalDocs = 1
	}

	// 2. Coletar Vocabulário da Ilha (TF Local)
	termClusterFreq := make(map[string]float64)
	for _, text := range texts {
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if len(line) == 0 {
				continue
			}

			weight := 1.0
			if strings.HasPrefix(line, "#") {
				weight = 5.0
			}

			words := tokenize(line)
			for i := 0; i < len(words); i++ {
				w1 := words[i]
				w1Valid := len(w1) >= 3 && !stopwordMap[w1] && !isNoise(w1)

				if w1Valid {
					termClusterFreq[w1] += weight

					// Extrair Bigrama se a próxima palavra também for válida
					if i < len(words)-1 {
						w2 := words[i+1]
						w2Valid := len(w2) >= 3 && !stopwordMap[w2] && !isNoise(w2)
						if w2Valid {
							bigram := w1 + " " + w2
							termClusterFreq[bigram] += weight
						}
					}
				}
			}
		}
	}

	// 3. Rankear palavras usando estatísticas globais do motor de busca
	type kv struct {
		Key   string
		Value float64
	}
	var candidates []kv
	for word, tf := range termClusterFreq {
		candidates = append(candidates, kv{word, tf})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Value > candidates[j].Value
	})

	// Analisar apenas os top 50 termos para evitar I/O excessivo no Bleve
	limit := 50
	if len(candidates) < limit {
		limit = len(candidates)
	}

	var ss []kv
	for i := 0; i < limit; i++ {
		word := candidates[i].Key
		tf := candidates[i].Value

		// Consultar a frequência do termo no dicionário global (IDF)
		termCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		tfr, err := reader.TermFieldReader(termCtx, []byte(word), "texto", false, false, false)
		cancel()
		if err != nil || tfr == nil {
			continue
		}
		df := float64(tfr.Count())
		tfr.Close()

		if df == 0 {
			df = 1
		}

		// Matemática BM25 Lite
		idf := math.Log((float64(totalDocs)-df+0.5)/(df+0.5) + 1.0)
		k1 := 1.2
		tfSaturado := (tf * (k1 + 1)) / (tf + k1)

		score := tfSaturado * idf
		// Boost semântico para bigramas
		if strings.Contains(word, " ") {
			score *= 2.5
		}

		ss = append(ss, kv{word, score})
	}

	if len(ss) == 0 {
		return "Geral", []string{}
	}

	sort.Slice(ss, func(i, j int) bool {
		if ss[i].Value == ss[j].Value {
			return ss[i].Key < ss[j].Key // Desempate estável
		}
		return ss[i].Value > ss[j].Value
	})

	// Coletar Top 5 Keywords
	var keywords []string
	for i := 0; i < len(ss) && i < 5; i++ {
		keywords = append(keywords, strings.ToUpper(ss[i].Key))
	}

	// Gerar Label Principal
	label := keywords[0]
	if len(keywords) > 1 && ss[1].Value > ss[0].Value*0.7 {
		// Só junta se nenhum dos dois for bigrama para evitar nomes muito longos
		if !strings.Contains(keywords[0], " ") && !strings.Contains(keywords[1], " ") {
			label = keywords[0] + " / " + keywords[1]
		}
	}

	return label, keywords
}

func isNoise(s string) bool {
	if len(s) == 0 {
		return false
	}
	// Tokenizer já removeu símbolos. Focamos em dígitos ou extensões comuns.
	for _, r := range s {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	// Divide por qualquer coisa que não seja letra ou número
	f := func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	}
	return strings.FieldsFunc(text, f)
}
