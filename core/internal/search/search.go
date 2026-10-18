package search

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"ton618/core/internal/core/db"
	"ton618/core/internal/core/domain"
)

type SearchHit struct {
	ID           string
	Score        float64
	Doc          db.Document
	FinalScore   float64
	ScoreDetails map[string]float64
	Highlight    map[string][]string
	// HasOriginalTerm indica se o documento contém um termo ORIGINAL da query
	// (não só um sinônimo). Governa a ordenação: docs com o termo exato sempre
	// vêm antes dos que casam apenas via expansão de sinônimos.
	HasOriginalTerm bool
}

type SearchResults struct {
	Hits  []SearchHit
	Total int
}

var (
	quoteRegex       = regexp.MustCompile(`"([^"]+)"`)
	singleQuoteRegex = regexp.MustCompile(`'([^']+)'`)
	tagFilterRegex   = regexp.MustCompile(`\+?tags:("[^"]+"|[^\s]+)`)
	cleanQueryRe     = regexp.MustCompile(`[\+\*"]`)
	spacesRe         = regexp.MustCompile(`\s+`)
	nativeHashtag    = regexp.MustCompile(`(?:\s|^)#([a-zA-Z0-9_À-ÿ\-]+)([?!]*)`)
	ftsUnsafeRe      = regexp.MustCompile(`[\^~()]`) // chars que interferem com sintaxe FTS5
	// quotedTermsRegex é compilado uma vez (antes era recompilado a cada
	// chamada de extractTerms, que roda em loop no gate da busca híbrida).
	quotedTermsRegex = regexp.MustCompile(`"([^"]+)"|'([^']+)'`)
)

// maxTagOnlyCandidates limita o pool de candidatos de uma busca "só por tag".
// O FTS5 já restringe o match pela tag, então não é preciso materializar o
// corpus inteiro: antes o LIMIT era 99999 e chegava a alocar ~180MB por busca
// em vaults grandes. O `total` continua exato (vem do COUNT), só reduzimos
// quantos documentos são carregados para re-ranking.
const maxTagOnlyCandidates = 500

// sanitizeFTS5Term removes characters that interfere with FTS5 query syntax.
func sanitizeFTS5Term(term string) string {
	term = ftsUnsafeRe.ReplaceAllString(term, "")
	if len(term) > 100 {
		term = term[:100]
	}
	return term
}

var stopwords = map[string]bool{
	"de": true, "da": true, "do": true, "em": true, "no": true, "na": true,
	"um": true, "uma": true, "os": true, "as": true, "com": true, "por": true,
	"para": true, "que": true, "seu": true, "sua": true, "dos": true, "das": true,
	"pelo": true, "pela": true, "nos": true, "nas": true, "o": true, "a": true,
	"e": true, "the": true, "and": true, "or": true, "of": true, "to": true, "in": true,
}

// synonyms contém sinônimos PT-BR em forma SEM acento e minúscula (casa com o
// tokenizer unicode61 do FTS5 e com o removeAccents do fallback LIKE). Expansão
// é aplicada SOMENTE a consultas de 1 termo de conteúdo (ver buildFTSQuery): em
// multi-termo cada palavra é unida por AND e expandir todas geraria o produto
// cartesiano dos sinônimos, despencando a precisão.
var synonyms = map[string][]string{
	"casa":       {"residencia", "moradia", "lar", "domicilio"},
	"carro":      {"automovel", "veiculo", "auto"},
	"automovel":  {"carro", "veiculo"},
	"veiculo":    {"carro", "automovel"},
	"trabalho":   {"emprego", "servico", "ocupacao", "labuta"},
	"emprego":    {"trabalho", "servico", "ocupacao"},
	"livro":      {"obra", "volume"},
	"escola":     {"colegio"},
	"pessoa":     {"individuo", "ser", "gente"},
	"dinheiro":   {"grana", "verba", "capital"},
	"comida":     {"alimento", "refeicao"},
	"problema":   {"questao", "obstaculo", "dificuldade"},
	"questao":    {"problema", "pergunta", "dificuldade"},
	"ideia":      {"conceito", "nocao"},
	"foto":       {"fotografia", "imagem"},
	"imagem":     {"foto", "figura"},
	"comeco":     {"inicio", "principio"},
	"inicio":     {"comeco", "principio"},
	"computador": {"pc", "maquina"},
	"celular":    {"smartphone", "telefone"},
	"telefone":   {"celular"},
	"teste":      {"prova", "ensaio"},
	"nota":       {"anotacao", "registro", "apontamento"},
	"anotacao":   {"nota", "registro"},
	"erro":       {"falha", "bug", "defeito"},
	"falha":      {"erro", "defeito"},
	"resposta":   {"solucao", "retorno"},
	"pergunta":   {"questao", "duvida"},
	"duvida":     {"pergunta", "incerteza"},
	"importante": {"relevante", "essencial"},
	"relevante":  {"importante", "essencial"},
	"rapido":     {"veloz"},
	"facil":      {"simples"},
	"dificil":    {"complicado"},
	"grande":     {"enorme", "vasto"},
	"pequeno":    {"diminuto", "reduzido"},
	"aprender":   {"estudar", "absorver"},
	"estudar":    {"aprender"},
	"escrever":   {"anotar", "redigir"},
	"buscar":     {"procurar", "pesquisar"},
	"pesquisar":  {"buscar", "procurar"},
	"procurar":   {"buscar", "pesquisar"},
	"encontrar":  {"achar", "localizar"},
	"achar":      {"encontrar", "localizar"},
	"comprar":    {"adquirir"},
	"usar":       {"utilizar", "empregar"},
	"utilizar":   {"usar", "empregar"},
	"mostrar":    {"exibir", "apresentar"},
	"criar":      {"produzir", "desenvolver"},
	"fazer":      {"realizar", "executar"},
	"dar":        {"entregar", "fornecer"},
	"receber":    {"obter"},
	"ter":        {"possuir"},
	"possuir":    {"ter"},
	"saber":      {"conhecer"},
	"conhecer":   {"saber", "dominar"},
	"pensar":     {"refletir"},
	"falar":      {"dizer", "comentar"},
	"dizer":      {"falar"},
	"ver":        {"olhar", "observar"},
	"olhar":      {"ver", "observar"},
	"ouvir":      {"escutar"},
	"escutar":    {"ouvir"},
	"lembrar":    {"recordar", "rememorar"},
	"recordar":   {"lembrar", "rememorar"},
	"precisao":   {"exatidao"},
	"exatidao":   {"precisao"},
	"rapidez":    {"velocidade"},
	"qualidade":  {"excelencia"},
	"saude":      {"bem-estar", "bemestar"},
	"feliz":      {"contente", "alegre"},
	"triste":     {"melancolico"},
	"bom":        {"otimo", "excelente"},
	"ruim":       {"pessimo"},
	"lugar":      {"local", "espaco"},
	"local":      {"lugar", "espaco"},
	"tempo":      {"periodo", "momento"},
	"momento":    {"instante"},
	"mundo":      {"globo"},
	"pais":       {"nacao"},
	"cidade":     {"municipio"},
	"noticia":    {"novidade", "reportagem"},
	"reportagem": {"noticia", "materia"},

	// ── Nomes / conceitos ──
	"estado":       {"condicao", "situacao", "circunstancia"},
	"forma":        {"modo", "maneira", "jeito"},
	"metodo":       {"tecnica", "procedimento", "processo"},
	"conhecimento": {"saber", "erudicao", "sabedoria"},
	"exemplo":      {"caso", "modelo", "instancia"},
	"resultado":    {"consequencia", "desfecho", "efeito"},
	"causa":        {"motivo", "razao", "origem"},
	"motivo":       {"causa", "razao", "justificativa"},
	"razao":        {"motivo", "causa", "justificativa"},
	"meta":         {"objetivo", "alvo", "proposito"},
	"objetivo":     {"meta", "finalidade", "proposito"},
	"plano":        {"projeto", "esquema", "estrategia"},
	"projeto":      {"plano", "programa", "iniciativa"},
	"processo":     {"procedimento", "fluxo", "metodo"},
	"ferramenta":   {"instrumento", "utensilio", "recurso"},
	"texto":        {"conteudo", "escrito", "documento"},
	"documento":    {"arquivo", "escrito", "registro"},
	"arquivo":      {"documento", "ficheiro", "pasta"},
	"resumo":       {"sumario", "sintese", "sinopse"},
	"capitulo":     {"secao", "parte", "divisao"},
	"secao":        {"parte", "divisao", "capitulo"},
	"lista":        {"relacao", "enumeracao", "elenco"},
	"tabela":       {"quadro", "matriz"},
	"diagrama":     {"esquema", "grafico", "mapa"},
	"mapa":         {"diagrama", "grafico"},
	"grafico":      {"diagrama", "carta"},
	"video":        {"filmagem", "gravacao"},
	"audio":        {"som", "gravacao"},
	"musica":       {"cancao", "melodia", "som"},
	"cancao":       {"musica", "melodia"},
	"artigo":       {"texto", "materia", "ensaio"},
	"ensaio":       {"artigo", "teste", "experimento"},
	"pesquisa":     {"estudo", "investigacao", "levantamento"},
	"estudo":       {"pesquisa", "investigacao", "aprendizado"},
	"investigacao": {"pesquisa", "apuracao"},
	"ambiente":     {"meio", "ecossistema", "entorno"},
	"meio":         {"ambiente", "ecossistema"},
	"historia":     {"narrativa", "cronica", "relato"},
	"relato":       {"historia", "narrativa", "descricao"},
	"descricao":    {"detalhe", "exposicao", "relato"},
	"detalhe":      {"pormenor", "particularidade"},
	"parte":        {"porcao", "elemento", "pedaco"},
	"pedaco":       {"parte", "fragmento", "porcao"},
	"elemento":     {"componente", "item", "parte"},
	"item":         {"elemento", "artigo", "ponto"},
	"ponto":        {"item", "assunto", "aspecto"},
	"topico":       {"assunto", "tema", "ponto"},
	"assunto":      {"tema", "topico", "materia"},
	"tema":         {"assunto", "topico", "motivo"},
	"materia":      {"assunto", "conteudo", "disciplina"},
	"foco":         {"centro", "destaque"},
	"destaque":     {"realce", "enfase"},
	"enfase":       {"destaque", "realce"},
	"valor":        {"preco", "custo", "importancia"},
	"custo":        {"preco", "despesa", "gasto"},
	"preco":        {"valor", "custo", "tarifa"},
	"gasto":        {"despesa", "custo"},
	"receita":      {"renda", "ganho", "lucro"},
	"lucro":        {"ganho", "beneficio"},
	"ganho":        {"lucro", "receita", "rendimento"},
	"rendimento":   {"ganho", "desempenho", "producao"},
	"desempenho":   {"rendimento", "resultado", "performance"},
	"quantidade":   {"volume", "numero", "grandeza"},
	"numero":       {"quantidade", "cifra", "contagem"},
	"familia":      {"parentes", "nucleo familiar"},
	"amigo":        {"companheiro", "colega", "camarada"},

	// ── Verbos ──
	"chegar":      {"alcancar", "atingir"},
	"alcancar":    {"atingir", "conseguir", "obter"},
	"atingir":     {"alcancar", "conseguir", "obter"},
	"precisar":    {"necessitar", "requerer"},
	"necessitar":  {"precisar", "requerer"},
	"ajudar":      {"auxiliar", "colaborar", "apoiar"},
	"auxiliar":    {"ajudar", "colaborar", "apoiar"},
	"apoiar":      {"ajudar", "sustentar", "colaborar"},
	"colaborar":   {"ajudar", "cooperar"},
	"tentar":      {"experimentar", "ensaiar", "buscar"},
	"ganhar":      {"obter", "conseguir", "vencer"},
	"conseguir":   {"obter", "alcancar", "lograr"},
	"obter":       {"conseguir", "alcancar", "adquirir"},
	"ler":         {"estudar", "percorrer"},
	"ensinar":     {"instruir", "educar", "lecionar"},
	"instruir":    {"ensinar", "educar"},
	"mudar":       {"alterar", "modificar", "transformar"},
	"alterar":     {"mudar", "modificar"},
	"modificar":   {"mudar", "alterar", "transformar"},
	"transformar": {"mudar", "converter"},
	"melhorar":    {"aprimorar", "aperfeicoar"},
	"aprimorar":   {"melhorar", "aperfeicoar"},
	"piorar":      {"degradar", "declinar"},
	"aumentar":    {"crescer", "ampliar", "elevar"},
	"crescer":     {"aumentar", "ampliar"},
	"reduzir":     {"diminuir", "cortar", "encurtar"},
	"diminuir":    {"reduzir", "baixar"},
	"comecar":     {"iniciar", "principiar"},
	"iniciar":     {"comecar", "principiar"},
	"terminar":    {"finalizar", "concluir", "acabar"},
	"finalizar":   {"terminar", "concluir"},
	"concluir":    {"finalizar", "terminar", "encerrar"},
	"parar":       {"cessar", "interromper"},
	"continuar":   {"prosseguir", "seguir"},
	"seguir":      {"continuar", "acompanhar"},
	"escolher":    {"selecionar", "optar", "preferir"},
	"selecionar":  {"escolher", "optar"},
	"optar":       {"escolher", "selecionar"},
	"preferir":    {"escolher", "optar"},
	"decidir":     {"resolver", "determinar", "definir"},
	"resolver":    {"solucionar", "decidir"},
	"definir":     {"especificar", "determinar"},
	"explicar":    {"esclarecer", "detalhar", "descrever"},
	"esclarecer":  {"explicar", "elucidar"},
	"entender":    {"compreender", "perceber"},
	"compreender": {"entender", "perceber"},
	"perceber":    {"notar", "observar", "entender"},
	"notar":       {"perceber", "observar"},
	"esquecer":    {"ignorar", "olvidar"},

	// ── Adjetivos ──
	"novo":         {"recente", "moderno", "inedito"},
	"antigo":       {"velho", "passado", "historico"},
	"velho":        {"antigo", "idoso"},
	"util":         {"proveitoso", "aproveitavel", "valioso"},
	"necessario":   {"preciso", "essencial", "indispensavel"},
	"essencial":    {"fundamental", "indispensavel", "necessario"},
	"fundamental":  {"essencial", "basico", "principal"},
	"principal":    {"fundamental", "primario", "basico"},
	"completo":     {"integral", "total", "inteiro"},
	"total":        {"completo", "integral"},
	"correto":      {"certo", "exato", "adequado"},
	"verdadeiro":   {"autentico", "real"},
	"real":         {"verdadeiro", "concreto", "atual"},
	"possivel":     {"viavel", "potencial"},
	"viavel":       {"possivel", "praticavel"},
	"provavel":     {"possivel", "plausivel"},
	"igual":        {"equivalente", "identico", "mesmo"},
	"diferente":    {"distinto", "variado", "diverso"},
	"diverso":      {"variado", "diferente"},
	"similar":      {"parecido", "semelhante", "analogo"},
	"semelhante":   {"parecido", "similar", "analogo"},
	"parecido":     {"semelhante", "similar"},
	"unico":        {"singular", "exclusivo", "so"},
	"singular":     {"unico", "particular", "inusitado"},
	"particular":   {"especifico", "singular", "especial"},
	"especial":     {"particular", "especifico", "diferenciado"},
	"especifico":   {"particular", "exato", "definido"},
	"geral":        {"global", "amplo", "comum"},
	"global":       {"geral", "amplo", "total"},
	"comum":        {"usual", "geral", "corriqueiro"},
	"usual":        {"comum", "habitual", "normal"},
	"normal":       {"comum", "usual", "padrao"},
	"incomum":      {"raro", "inusual", "diferente"},
	"raro":         {"incomum", "infrequente"},
	"frequente":    {"comum", "usual", "recorrente"},
	"continuo":     {"constante", "ininterrupto", "seguido"},
	"claro":        {"nitido", "evidente", "obvio", "luminoso"},
	"evidente":     {"claro", "obvio", "nitido"},
	"obvio":        {"evidente", "claro", "nitido"},
	"simples":      {"facil", "basico", "descomplicado"},
	"complexo":     {"complicado", "intrincado", "elaborado"},
	"complicado":   {"complexo", "dificil"},
	"interessante": {"atrativo", "envolvente", "curioso"},
	"curioso":      {"interessante", "intrigante"},
}

// normalizeSynonym normaliza um termo/sinônimo para a forma de lookup:
// minúsculas e sem acento (casa com o tokenizer unicode61 do FTS5 e com o
// removeAccents do fallback LIKE). É o mesmo formato das chaves/valores do mapa
// embarcado e das regras salvas na aba "Sinônimos".
func normalizeSynonym(s string) string {
	return removeAccents(strings.ToLower(strings.TrimSpace(s)))
}

// synonymsFor devolve os sinônimos (sem acento) de um termo de busca, ou nil se
// o termo não tiver entrada no mapa embarcado. Normaliza o termo para o lookup.
func synonymsFor(term string) []string {
	return synonyms[normalizeSynonym(term)]
}

// UserSynonymMap converte as regras salvas na aba "Sinônimos" (da tabela
// settings) em um mapa termo→sinônimos já normalizado. Descartada a si mesmo
// como sinônimo e entradas com termo vazio ou sem sinônimos.
func UserSynonymMap(rules []domain.SynonymRule) map[string][]string {
	out := make(map[string][]string)
	for _, r := range rules {
		t := normalizeSynonym(r.Termo)
		if t == "" {
			continue
		}
		var syns []string
		for _, s := range r.Sinonimos {
			ns := normalizeSynonym(s)
			if ns != "" && ns != t {
				syns = append(syns, ns)
			}
		}
		if len(syns) > 0 {
			out[t] = syns
		}
	}
	return out
}

// loadSynonymLookup monta a função de expansão de sinônimos usada na busca:
// primeiro consulta o mapa configurado pelo usuário (settings "synonyms_config")
// e, na falta de entrada, cai para o mapa embarcado padrão. Assim o usuário pode
// adicionar/sobrescrever sinônimos sem perder o conjunto básico embarcado.
func loadSynonymLookup(store *db.Store) func(string) []string {
	var user map[string][]string
	if store != nil {
		if raw, err := store.GetSetting("synonyms_config"); err == nil && raw != "" {
			var rules []domain.SynonymRule
			if json.Unmarshal([]byte(raw), &rules) == nil {
				user = UserSynonymMap(rules)
			}
		}
	}
	return func(term string) []string {
		t := normalizeSynonym(term)
		if user != nil {
			if syns, ok := user[t]; ok {
				return syns
			}
		}
		return synonyms[t]
	}
}

func Search(ctx context.Context, store *db.Store, rawQuery string, from, size int, getBacklinkCount func(string) int, getSynapticWeight func(string) float64) (*SearchResults, error) {
	if rawQuery == "" || rawQuery == "*" {
		return listAll(store, from, size)
	}

	// Extrai tags e a query restante
	queryTags, remainingQuery := extractTags(rawQuery)
	remainingQuery = strings.TrimSpace(remainingQuery)

	// 1. Try FTS5 first
	ftsQuery := buildFTSQueryWithSynonyms(rawQuery, loadSynonymLookup(store))
	// Se a busca textual é vazia mas temos tags, o FTS já filtra pela tag;
	// ampliamos o pool de candidatos para re-ranking, mas de forma limitada
	// (evita materializar o corpus inteiro — ver maxTagOnlyCandidates).
	limitSize := size * 3
	if remainingQuery == "" && len(queryTags) > 0 {
		limitSize = size * 10
		if limitSize > maxTagOnlyCandidates {
			limitSize = maxTagOnlyCandidates
		}
	}
	results, total, err := store.SearchFTSWithContext(ctx, ftsQuery, from, limitSize)
	ftsFailed := false
	if err != nil {
		log.Printf("[Search] FTS5 error: %v, falling back to LIKE\n", err)
		ftsFailed = true
		results, total, _ = store.SearchFTSLikeWithContext(ctx, remainingQuery, 0, limitSize)
	}

	// 2. Fallback fuzzy (LIKE) SOMENTE quando o FTS não achou nada. Antes
	// disparava com `total < 3`, forçando um full scan de `documents` (~80ms)
	// mesmo quando o FTS já tinha 1-2 resultados legítimos.
	if total == 0 && remainingQuery != "" && !ftsFailed {
		likeResults, likeTotal, _ := store.SearchFTSLikeWithContext(ctx, remainingQuery, 0, size*3)
		// total==0 ⇒ a página do FTS está vazia; o conjunto é todo do LIKE.
		results = likeResults
		total = likeTotal
	}

	// 3. Convert to SearchHits and re-rank
	heuristicTerms := extractTerms(rawQuery)
	cleanedQuery := cleanQuery(rawQuery)

	// ── Batch-load timestamps para evitar N+1 ──
	docIDs := make([]string, len(results))
	for i, r := range results {
		docIDs[i] = r.DocID
	}
	timestampMap, _ := store.BatchGetTimestamps(docIDs)

	// ── Cache para getSynapticWeight e getBacklinkCount ──
	weightCache := make(map[string]float64, len(results))
	backlinkCache := make(map[string]int, len(results))

	var hits []SearchHit
	for _, r := range results {
		// Converte FTSResult → Document diretamente, sem chamar GetDocument
		ts := timestampMap[r.DocID]
		doc := db.Document{
			ID:        r.DocID,
			Tipo:      r.Tipo,
			Arquivo:   r.Arquivo,
			Secao:     r.Secao,
			Texto:     r.Texto,
			Tags:      r.Tags,
			Timestamp: ts,
		}

		// Filtro de tags como segunda linha de defesa (garante exatidão)
		if len(queryTags) > 0 {
			docTags := db.TagsToSlice(doc.Tags)
			docTagMap := make(map[string]bool)
			for _, dt := range docTags {
				docTagMap[strings.ToLower(dt)] = true
			}
			hasAllTags := true
			for _, qt := range queryTags {
				if !docTagMap[strings.ToLower(qt)] {
					hasAllTags = false
					break
				}
			}
			if !hasAllTags {
				continue
			}
		}

		hit := SearchHit{
			ID:              r.DocID,
			Score:           r.Rank,
			Doc:             doc,
			HasOriginalTerm: HasContentEvidence(doc, rawQuery),
		}

		// Cache para peso sináptico
		weight, ok := weightCache[doc.Arquivo]
		if !ok {
			weight = getSynapticWeight(doc.Arquivo)
			weightCache[doc.Arquivo] = weight
		}

		// Cache para contagem de backlinks
		backlinkCount, ok := backlinkCache[doc.Arquivo]
		if !ok {
			backlinkCount = getBacklinkCount(strings.ToLower(doc.Arquivo))
			backlinkCache[doc.Arquivo] = backlinkCount
		}

		score, details := scoreFragment(&hit, heuristicTerms, cleanedQuery, weight, backlinkCount)
		hit.FinalScore = score
		hit.ScoreDetails = details
		if r.Snippet != "" {
			hit.Highlight = map[string][]string{"texto": {r.Snippet}}
		}

		hits = append(hits, hit)
	}

	// 4. Sort by original-term presence first, then by final score.
	// Docs que contêm o termo ORIGINAL da busca vêm sempre antes dos que casam
	// apenas via sinônimo (expansão), independentemente do BM25. Dentro de cada
	// grupo, a ordem segue o FinalScore e, no empate, a recência.
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].HasOriginalTerm != hits[j].HasOriginalTerm {
			return hits[i].HasOriginalTerm
		}
		if math.Abs(hits[i].FinalScore-hits[j].FinalScore) < 0.0001 {
			return hits[i].Doc.Timestamp > hits[j].Doc.Timestamp
		}
		return hits[i].FinalScore > hits[j].FinalScore
	})

	// 5. Trim to requested size
	if len(hits) > size {
		hits = hits[:size]
	}

	return &SearchResults{Hits: hits, Total: total}, nil
}

func listAll(store *db.Store, from, size int) (*SearchResults, error) {
	docs, total, err := store.GetDocumentsPaginated(from, size)
	if err != nil {
		return nil, err
	}

	var hits []SearchHit
	for _, doc := range docs {
		hits = append(hits, SearchHit{
			ID:         doc.ID,
			Doc:        doc,
			Score:      1.0,
			FinalScore: 1.0,
		})
	}
	return &SearchResults{Hits: hits, Total: total}, nil
}

// buildFTSQuery converte query do usuário para sintaxe FTS5 com pesos por coluna.
// Pesos: tags 50x, arquivo 20x, secao 10x, texto 1x. Usa o mapa embarcado padrão.
func buildFTSQuery(raw string) string {
	return buildFTSQueryWithSynonyms(raw, synonymsFor)
}

// buildFTSQueryWithSynonyms é buildFTSQuery com uma função de expansão de
// sinônimos fornecida (embarcada ou configurada pelo usuário).
func buildFTSQueryWithSynonyms(raw string, lookup func(string) []string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// Extrai tags e a query restante
	tags, remaining := extractTags(raw)
	remaining = strings.TrimSpace(remaining)

	var parts []string

	// Adiciona restrições de tags no formato FTS5 tags:"nome_da_tag"
	for _, tag := range tags {
		tag = sanitizeFTS5Term(tag)
		// Filtra apenas caracteres válidos para tag
		tag = strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
				return r
			}
			return -1
		}, tag)
		if tag != "" {
			parts = append(parts, `tags:"`+tag+`"`)
		}
	}

	if remaining != "" {
		// Extract exact quoted phrases — aplica a todas as colunas
		quotes := quoteRegex.FindAllStringSubmatch(remaining, -1)
		for _, q := range quotes {
			if len(q) == 2 && strings.TrimSpace(q[1]) != "" {
				phClean := sanitizeFTS5Term(strings.TrimSpace(q[1]))
				if phClean != "" {
					ph := `"` + phClean + `"`
					parts = append(parts, `(tags:`+ph+` OR arquivo:`+ph+` OR secao:`+ph+` OR texto:`+ph+`)`)
				}
			}
			remaining = strings.Replace(remaining, q[0], " ", 1)
		}

		// Extract proximity phrases in single quotes
		proximity := singleQuoteRegex.FindAllStringSubmatch(remaining, -1)
		for _, q := range proximity {
			if len(q) == 2 && strings.TrimSpace(q[1]) != "" {
				ph := buildProximityExpression(strings.TrimSpace(q[1]))
				if ph != "" {
					parts = append(parts, ph)
				}
			}
			remaining = strings.Replace(remaining, q[0], " ", 1)
		}

		// Substitui hífens e outros caracteres especiais por espaços na busca por palavras soltas,
		// permitindo que palavras como "e-mail" ou "go-lang" sejam divididas em termos FTS5 válidos.
		remainingCleaned := strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return r
			}
			return ' '
		}, remaining)

		// Process remaining words
		words := strings.Fields(remainingCleaned)

		// Reúne as palavras de conteúdo (não stopwords). A expansão por sinônimos
		// é aplicada SOMENTE quando há UMA única palavra de conteúdo: com uma
		// palavra é jogo de recall puro (o sentido não depende de combinação). Em
		// consultas de várias palavras cada termo é unido por AND e expandir todos
		// dispararia o produto cartesiano dos sinônimos — precisão despenca.
		var contentWords []string
		for _, w := range words {
			w = sanitizeFTS5Term(w)
			if w == "" || stopwords[strings.ToLower(w)] {
				continue
			}
			contentWords = append(contentWords, strings.ToLower(w))
		}
		expandSynonyms := len(contentWords) == 1

		for _, wLower := range contentWords {
			var variants []string
			if expandSynonyms {
				variants = lookup(wLower)
			}
			if len(variants) > 0 {
				var orParts []string
				for _, v := range append([]string{wLower}, variants...) {
					tv := v
					if len(tv) > 2 {
						tv += "*"
					}
					orParts = append(orParts, `(tags:`+tv+` OR arquivo:`+tv+` OR secao:`+tv+` OR texto:`+tv+`)`)
				}
				parts = append(parts, "("+strings.Join(orParts, " OR ")+")")
			} else {
				wStar := wLower
				if len(wStar) > 2 {
					wStar += "*"
				}
				parts = append(parts, `(tags:`+wStar+` OR arquivo:`+wStar+` OR secao:`+wStar+` OR texto:`+wStar+`)`)
			}
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " AND ")
}

func buildProximityExpression(phrase string) string {
	words := strings.Fields(phrase)
	if len(words) == 0 {
		return ""
	}
	if len(words) == 1 {
		w := strings.Trim(words[0], "?,;.:!+-")
		if w == "" || stopwords[strings.ToLower(w)] {
			return ""
		}
		return `(tags:` + w + ` OR arquivo:` + w + ` OR secao:` + w + ` OR texto:` + w + `)`
	}

	// FTS5 nao suporta NEAR. Usa AND simples para que o LIKE fallback
	// (no search.go) possa fazer a correspondencia de proximidade.
	var colQueries []string
	for _, col := range []string{"texto", "secao"} {
		var colTerms []string
		for _, w := range words {
			w = strings.Trim(w, "?,;.:!+-")
			if w == "" || stopwords[strings.ToLower(w)] {
				continue
			}
			colTerms = append(colTerms, col+":\""+w+"\"")
		}
		if len(colTerms) >= 2 {
			colQueries = append(colQueries, "("+strings.Join(colTerms, " AND ")+")")
		}
	}
	// Fallback: se apos filtrar stopwords sobrou 1 termo, retorna como termo unico
	if len(colQueries) == 0 {
		var single string
		for _, w := range words {
			w = strings.Trim(w, "?,;.:!+-")
			if w == "" || stopwords[strings.ToLower(w)] {
				continue
			}
			single = w
			break
		}
		if single != "" {
			return `(tags:` + single + ` OR arquivo:` + single + ` OR secao:` + single + ` OR texto:` + single + `)`
		}
	}
	if len(colQueries) > 0 {
		return strings.Join(colQueries, " OR ")
	}
	return ""
}

// extractTerms extrai termos relevantes da query (para re-ranking).
// Preserva frases entre aspas duplas ou simples como termo unico.
func extractTerms(raw string) []string {
	var terms []string
	remaining := raw

	for {
		m := quotedTermsRegex.FindStringSubmatch(remaining)
		if m == nil {
			break
		}
		phrase := strings.TrimSpace(m[1])
		if phrase == "" {
			phrase = strings.TrimSpace(m[2])
		}
		if len(phrase) > 1 && !stopwords[phrase] {
			terms = append(terms, phrase)
			// Adiciona palavras individuais como fallback
			for _, pw := range strings.Fields(phrase) {
				pl := strings.ToLower(strings.Trim(pw, "?,;.:!+-"))
				if pl != "" && !stopwords[pl] && len(pl) > 1 {
					terms = append(terms, pl)
				}
			}
		}
		remaining = strings.Replace(remaining, m[0], " ", 1)
	}

	clean := tagFilterRegex.ReplaceAllString(remaining, " ")
	clean = cleanQueryRe.ReplaceAllString(clean, " ")
	words := strings.Fields(strings.ToLower(clean))

	for _, w := range words {
		w = strings.Trim(w, "?,;.:!+-")
		if w != "" && !stopwords[w] && len(w) > 1 {
			terms = append(terms, w)
		}
	}
	return terms
}

func cleanQuery(raw string) string {
	clean := strings.ToLower(cleanQueryRe.ReplaceAllString(raw, " "))
	return strings.TrimSpace(spacesRe.ReplaceAllString(clean, " "))
}

// extractTags extrai tags formatadas (tags:xxx ou #hashtag) da query bruta.

func extractTags(raw string) (tags []string, remaining string) {
	remaining = raw
	matches := tagFilterRegex.FindAllStringSubmatch(remaining, -1)
	for _, m := range matches {
		if len(m) > 1 {
			tags = append(tags, strings.ToLower(strings.Trim(m[1], `"`)))
			remaining = strings.Replace(remaining, m[0], " ", 1)
		}
	}
	hashtags := nativeHashtag.FindAllStringSubmatch(remaining, -1)
	for _, m := range hashtags {
		if len(m) > 1 {
			tags = append(tags, strings.ToLower(m[1]))
			remaining = strings.Replace(remaining, m[0], " ", 1)
		}
	}
	return tags, remaining
}

// HasExplicitTagFilter indica se a query contém um filtro explícito de tag
// (`tags:xxx` ou `#hashtag`). Nesses casos o match por tag é intencional e não
// deve passar pelo gate de evidência da busca híbrida.
func HasExplicitTagFilter(rawQuery string) bool {
	tags, _ := extractTags(rawQuery)
	return len(tags) > 0
}

// HasContentEvidence indica se algum termo da query aparece no conteúdo da nota
// (texto/secao/arquivo), desconsiderando ocorrências apenas como hashtag
// (#termo é rótulo, não menção textual). Usado pela busca híbrida como gate de
// evidência: docs que casam no FTS só por tag/hashtag precisam de aval semântico.
// Retorna true quando a query não tem termos verificáveis (ex: só stopwords),
// para não filtrar indevidamente a busca vazia/lista completa.
func HasContentEvidence(doc db.Document, rawQuery string) bool {
	terms := extractTerms(rawQuery)
	if len(terms) == 0 {
		return true
	}
	// Remove hashtags do texto antes da verificação (rótulo ≠ menção real).
	texto := nativeHashtag.ReplaceAllString(removeAccents(strings.ToLower(doc.Texto)), " ")
	secao := removeAccents(strings.ToLower(doc.Secao))
	arquivo := removeAccents(strings.ToLower(doc.Arquivo))
	for _, term := range terms {
		t := removeAccents(strings.ToLower(term))
		if stopwords[t] || len(t) < 2 {
			continue
		}
		if strings.Contains(texto, t) || strings.Contains(secao, t) || strings.Contains(arquivo, t) {
			return true
		}
	}
	return false
}

var accentReplacer = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c",
	"Á", "a", "À", "a", "Â", "a", "Ã", "a", "Ä", "a",
	"É", "e", "È", "e", "Ê", "e", "Ë", "e",
	"Í", "i", "Ì", "i", "Î", "i", "Ï", "i",
	"Ó", "o", "Ò", "o", "Ô", "o", "õ", "o", "Ö", "o",
	"Ú", "u", "Ù", "u", "Û", "u", "Ü", "u",
	"Ç", "c",
)

// removeAccents remove acentos e diacríticos de uma string.
func removeAccents(s string) string {
	return accentReplacer.Replace(s)
}
