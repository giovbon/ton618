package processor

import (
	"crypto/rand"
	"math/big"
)

var (
	adjectives = []string{
		"agil", "alegre", "alto", "amarelo", "amplo", "antigo", "azul",
		"belo", "bom", "brando", "bravo", "breve", "brilhante", "calmo",
		"castanho", "claro", "conciso", "coral", "certo", "curto",
		"doce", "direto", "diverso", "digno", "duro", "eficaz", "exato",
		"feliz", "fiel", "fino", "firme", "fixo", "forte", "fresco",
		"genuino", "gigante", "grande", "gracioso", "habil", "honesto",
		"igual", "impar", "intenso", "jovem", "justo", "largo", "leal",
		"leve", "limpo", "linear", "liso", "livre", "logico", "longo",
		"lento", "magno", "manso", "marrom", "mestre", "minimo", "misto",
		"multiplo", "nativo", "natural", "negro", "nobre", "nulo",
		"novo", "otimo", "paralelo", "perfeito", "pleno", "positivo",
		"pragmatico", "pratico", "preciso", "primo", "prioritario",
		"produtivo", "profundo", "proprio", "proximo", "puro", "rapido",
		"raro", "regular", "reto", "rigido", "robusto", "rustico",
		"sabio", "sagaz", "santo", "seguro", "sereno", "simples",
		"solido", "sutil", "tardio", "tenaz", "terso", "tipico", "tranquilo",
		"utimo", "unico", "urgente", "valido", "valoroso", "veloz",
		"verde", "versatil", "vigoroso", "vital", "vivo", "volumoso",
	}

	nouns = []string{
		"alva", "anjo", "arbusto", "areia", "astro", "atalho",
		"ave", "banco", "barco", "brisa", "caminho", "campo",
		"canto", "casa", "cavalo", "cedro", "ceu", "cidade",
		"corda", "colina", "concha", "cristal", "curva",
		"duna", "eco", "elefante", "elmo", "encosta", "espaco",
		"espelho", "estrela", "facho", "fala", "favo", "feno",
		"fio", "fogo", "folha", "fonte", "foz", "fronte", "fruto",
		"gato", "gleba", "gomo", "gota", "grilo", "gume", "haste",
		"ilha", "jato", "juba", "lago", "lagoa", "lareira", "lapis",
		"lasca", "lenda", "lince", "linha", "lobo", "lodo", "lua",
		"lume", "lustro", "magna", "mala", "manto", "mar", "mastro",
		"meta", "monte", "muro", "ninho", "nuvem", "onda", "osso",
		"palco", "passaro", "pato", "pena", "penhasco", "pilar",
		"pinho", "pista", "pluma", "porta", "porto", "prata", "praia",
		"prisma", "pulso", "puma", "quota", "raiz", "rama", "rede",
		"rifa", "rio", "rocha", "roda", "rosa", "rumo", "sabre",
		"seara", "selo", "selva", "senda", "serra", "sino", "solo",
		"sonho", "sopro", "taça", "tarde", "teto", "tesouro", "tigre",
		"tinta", "toalha", "topo", "torre", "touro", "trecho", "tribo",
		"trilha", "trono", "tulipa", "turquesa", "urze", "vale", "vela",
		"vento", "vereda", "vespa", "vidro", "vinha", "viola", "vista",
		"vulcao", "zebra",
	}
)

// GenerateCUID2 generates a human-readable note name using the diceware approach:
// adjetivo-substantivo-nn (e.g., "veloz-tigre-42").
// This is far more memorable than random character strings while still providing
// ~100 * 194 * 99 ≈ 1.9M unique combinations.
func GenerateCUID2() string {
	adj := randomChoice(adjectives)
	noun := randomChoice(nouns)
	num := randomInt(10, 99)

	return adj + "-" + noun + "-" + num
}

func randomChoice(list []string) string {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(list))))
	if err != nil {
		return list[0]
	}
	return list[n.Int64()]
}

func randomInt(min, max int) string {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	if err != nil {
		return "00"
	}
	val := min + int(n.Int64())
	if val < 10 {
		return "0" + itoa(val)
	}
	return itoa(val)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [3]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
