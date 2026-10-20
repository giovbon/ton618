package processor

import (
	"crypto/rand"
	"math/big"
	"strings"
)

var (
	adjectives = []string{
		"absurdo", "acumulado", "adormecido", "agoniado", "aleatorio",
		"amador", "amargurado", "ambiguo", "ansioso", "apressado",
		"arrogante", "atrasado", "caotico", "cetico", "chato",
		"confuso", "controverso", "complicado", "convencido", "coitado",
		"caido", "cinico", "desesperado", "desatento", "desajeitado",
		"desconfiado", "desmotivado", "dramatico", "duvidoso", "eccentrico",
		"empolgado", "enrolado", "esquisito", "estressado", "exagerado",
		"exaustivo", "falido", "falso", "fantasioso", "ficticio",
		"esquizo", "frustrado", "gambiarra", "hipocrita", "historico",
		"iludido", "impaciente", "imprevisivel", "improvisado", "incoerente",
		"incompetente", "indeciso", "ineficiente", "inexplicavel", "instavel",
		"ironico", "inutil", "irritado", "julgador", "lagado",
		"lento", "limitado", "louco", "malandro", "manipulador",
		"miseravel", "mistico", "maluco", "desgracado", "necrotico",
		"nervoso", "obsessivo", "obsoleto", "ostentador", "otario",
		"paranoico", "pessimo", "preguicoso", "pretensioso", "problematico",
		"procrastinador", "questionavel", "ranzinza", "rebelde", "redundante",
		"sarcastico", "surtado", "suspeito", "teimoso", "tragico",
		"traumatizado", "turbulento", "vago", "vazio", "viciado",
	}
	nouns = []string{
		"agiota", "algoritmo", "alien", "artropode", "boato",
		"boleto", "bug", "burocrata", "cafeteria", "calvo",
		"cancela", "capivara", "chaos", "charlata", "chefe",
		"clique", "clone", "palhaco", "code", "cogumelo",
		"coincidencia", "combustivel", "cometa", "conselheiro", "cpf",
		"crachá", "crise", "critico", "cupim", "debounce",
		"debt", "deploravel", "desculpas", "detetive", "dino",
		"diplomata", "diva", "dramaturgo", "duende", "embarcacao",
		"embuste", "estagiario", "fake", "fantasma", "fiasco",
		"fofoca", "fossil", "fratricida", "furo", "gambiarra",
		"ganancioso", "bebe", "mamata", "viado", "golem",
		"golpe", "gorila", "governante", "hacker", "herdeiro",
		"hiena", "hipster", "hoax", "horda", "idolo",
		"impostor", "incendiario", "infinito", "invertebrado", "janela",
		"karma", "kernel", "lagarto", "leak", "legado",
		"limbo", "lobby", "loop", "loser", "mago", "marmita",
		"meleca", "meme", "mentor", "microbio", "miragem",
		"mumia", "mutante", "aristocrata", "ninja", "noob",
		"ostentacao", "pancada", "panfleteiro", "papagaio", "paparazzi",
		"parasita", "parede", "patrao", "pato", "pavao",
		"pedagio", "pinguim", "pirata", "piramide", "pix",
		"playboy", "pombo", "pretensioso", "procrastinacao", "pseudonimo",
		"puddle", "rancor", "refem", "remendo", "robot",
		"panaca", "script", "seguidor", "sobrinho", "sorrisinho",
		"sombra", "spammer", "stalker", "susto", "tampa",
		"tapearia", "teclado", "troll", "trouxa", "unicornio",
		"vampiro", "varredor", "vendedor", "vovo", "zumbi",
	}
)

var (
	adjMap  map[string]bool
	nounMap map[string]bool
)

func init() {
	adjMap = make(map[string]bool, len(adjectives))
	for _, a := range adjectives {
		adjMap[a] = true
	}
	nounMap = make(map[string]bool, len(nouns))
	for _, n := range nouns {
		nounMap[n] = true
	}
}

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

// IsCUID2 verifica se o nome segue o padrão gerado automaticamente adjetivo-substantivo-nn.
func IsCUID2(name string) bool {
	base := strings.TrimPrefix(name, "notes/")
	base = strings.TrimSuffix(base, ".md")
	parts := strings.Split(base, "-")
	if len(parts) != 3 {
		return false
	}
	if !adjMap[strings.ToLower(parts[0])] {
		return false
	}
	if !nounMap[strings.ToLower(parts[1])] {
		return false
	}
	if len(parts[2]) != 2 || parts[2][0] < '0' || parts[2][0] > '9' || parts[2][1] < '0' || parts[2][1] > '9' {
		return false
	}
	return true
}

// IsDraftName verifica se um nome de arquivo é um rascunho recém-criado / gerado automaticamente.
func IsDraftName(name string) bool {
	if IsCUID2(name) {
		return true
	}
	base := strings.TrimPrefix(name, "notes/")
	base = strings.TrimSuffix(base, ".md")
	baseLower := strings.ToLower(base)
	return strings.HasPrefix(baseLower, "captura-") ||
		strings.HasPrefix(baseLower, "draft-") ||
		strings.HasPrefix(baseLower, "rascunho-") ||
		strings.HasPrefix(baseLower, "untitled") ||
		strings.HasPrefix(baseLower, "nova-nota")
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
