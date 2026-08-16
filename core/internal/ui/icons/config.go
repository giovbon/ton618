package icons

import (
	"sort"
	"strings"
	"sync"
)

// IconSpec define o par de ícone (Lucide) e cor (Tailwind CSS) de um elemento.
type IconSpec struct {
	Icon  string // Nome do ícone Lucide (ex: "sticky-note", "pencil-ruler", "container")
	Color string // Classe CSS de cor Tailwind (ex: "text-amber-400", "text-zinc-400")
}

// Config é o arquivo centralizador de configuração de ícones e cores do TON-618.
// Edite os ícones (Lucide) e as cores (classes CSS Tailwind) abaixo para alterar o sistema todo:
var Config = map[string]IconSpec{
	// ── TIPOS DE NOTAS E ARQUIVOS ──
	"nota":        {Icon: "sticky-note", Color: "#F54927"},
	"markdown":    {Icon: "sticky-note", Color: "#F54927"},
	"desenho":     {Icon: "pencil", Color: "#9C63F8"},
	"drawing":     {Icon: "pencil", Color: "#9C63F8"},
	"markmap":     {Icon: "chart-no-axes-gantt", Color: "#22C55E"},
	"mindmap":     {Icon: "chart-no-axes-gantt", Color: "#22C55E"},
	"pdf":         {Icon: "book-text", Color: "text-red-500"},
	"epub":        {Icon: "book-open", Color: "#F5F227"},
	"anexo":       {Icon: "package-plus", Color: "#0B61F4"},
	"attachment":  {Icon: "package-plus", Color: "#0B61F4"},
	"arquivo":     {Icon: "archive", Color: "text-zinc-400"},
	"archive":     {Icon: "archive", Color: "text-zinc-400"},
	"youtube":     {Icon: "video", Color: "text-red-500"},
	"artigo":      {Icon: "newspaper", Color: "text-blue-400"},
	"article":     {Icon: "newspaper", Color: "text-blue-400"},

	// ── NAVEGAÇÃO & HEADER ──
	"agenda":         {Icon: "calendar", Color: "text-sky-400"},
	"task":           {Icon: "construction", Color: "text-amber-400"},
	"todos":          {Icon: "construction", Color: "text-amber-400"},
	"tabulator":      {Icon: "container", Color: "#10C809"},
	"captura":        {Icon: "link", Color: "text-zinc-400"},
	"mapa-semantico": {Icon: "map", Color: "text-violet-400"},
	"semantic-map":   {Icon: "map", Color: "text-violet-400"},
	"compass":        {Icon: "map", Color: "text-violet-400"},
	"galaxy":         {Icon: "map", Color: "text-violet-400"},

	// ── BUSCA E FILTROS ──
	"busca-notas":     {Icon: "type-outline", Color: "text-sky-400"},
	"search-notes":    {Icon: "type-outline", Color: "text-sky-400"},
	"busca-global":    {Icon: "search", Color: "text-emerald-400"},
	"search-global":   {Icon: "search", Color: "text-emerald-400"},
	"busca-semantica": {Icon: "bot", Color: "text-violet-400"},
	"search-semantic": {Icon: "bot", Color: "text-violet-400"},
	"busca-hybrid":    {Icon: "sparkles", Color: "text-teal-400"},
	"search-hybrid":   {Icon: "sparkles", Color: "text-teal-400"},

	// ── UTILIDADES & MENUS ──
	"config":   {Icon: "settings", Color: "#696969"},
	"settings": {Icon: "settings", Color: "#696969"},
	"sair":     {Icon: "log-out", Color: "#696969"},
	"logout":   {Icon: "log-out", Color: "#696969"},
	"ajuda":    {Icon: "help", Color: "#696969"},
	"help":     {Icon: "help", Color: "#696969"},

	// ── AÇÕES E BOTÕES DENTRO DAS NOTAS ──
	"salvar":   {Icon: "save", Color: "text-emerald-400"},
	"save":     {Icon: "save", Color: "text-emerald-400"},
	"arquivar": {Icon: "archive", Color: "text-zinc-500"},
	"duplicar": {Icon: "copy", Color: "text-zinc-500"},
	"excluir":  {Icon: "trash", Color: "text-zinc-500"},
	"delete":   {Icon: "trash", Color: "text-zinc-500"},

	// ── ABAS DO MODAL DE CONFIGURAÇÕES ──
	"arquivamento":          {Icon: "archive", Color: "text-amber-400"},
	"settings-arquivamento": {Icon: "archive", Color: "text-amber-400"},
	"restaurar":             {Icon: "package", Color: "text-sky-400"},
	"settings-restaurar":    {Icon: "package", Color: "text-sky-400"},
	"backup":                {Icon: "database", Color: "text-emerald-400"},
	"settings-backup":       {Icon: "database", Color: "text-emerald-400"},
	"marcadores":            {Icon: "pin", Color: "text-purple-400"},
	"settings-marcadores":   {Icon: "pin", Color: "text-purple-400"},
	"settings-agenda":       {Icon: "calendar", Color: "text-cyan-400"},
	"ntfy":                  {Icon: "bell", Color: "text-rose-400"},
	"settings-ntfy":         {Icon: "bell", Color: "text-rose-400"},
	"semantica":             {Icon: "sparkles", Color: "text-violet-400"},
	"settings-semantica":    {Icon: "sparkles", Color: "text-violet-400"},
}

// isRawColor detecta se uma string é um valor de cor raw (hex, rgb, hsl).
func isRawColor(c string) bool {
	return strings.HasPrefix(c, "#") || strings.HasPrefix(c, "rgb(") || strings.HasPrefix(c, "rgba(") || strings.HasPrefix(c, "hsl(") || strings.HasPrefix(c, "hsla(")
}

// FormatColor trata valores de cor para aceitar tanto classes Tailwind ("text-indigo-800")
// quanto códigos Hex, RGB ou HSL (ex: "#F54927", "#000", "rgb(...)").
// Cores raw são retornadas como estão — o consumidor (Icon, JS) decide como aplicar.
func FormatColor(c string) string {
	c = strings.TrimSpace(c)
	if c == "" {
		return "text-zinc-400"
	}
	return c
}

// ParseClassAndStyle recebe uma string contendo classes CSS e opcionalmente uma cor raw
// (hex, rgb, hsl) e separa os dois: classes puras vão para cleanClass, cores vão para styleAttr.
// Ex: "w-4 h-4 #F54927" → cleanClass="w-4 h-4", styleAttr="color:#F54927"
// Ex: "w-4 h-4 text-pink-400" → cleanClass="w-4 h-4 text-pink-400", styleAttr=""
func ParseClassAndStyle(input string) (cleanClass string, styleAttr string) {
	parts := strings.Fields(input)
	var classes []string
	var styleParts []string

	for _, p := range parts {
		if isRawColor(p) {
			styleParts = append(styleParts, "color:"+p)
		} else {
			classes = append(classes, p)
		}
	}

	style := ""
	if len(styleParts) > 0 {
		style = strings.Join(styleParts, ";")
	}
	return strings.Join(classes, " "), style
}

// ColorToStyle converte um valor de cor para string de estilo CSS inline.
// Se for uma classe Tailwind, retorna vazio (deve ser usada como classe).
// Se for cor raw (hex/rgb/hsl), retorna "color:#F54927".
func ColorToStyle(c string) string {
	c = strings.TrimSpace(c)
	if c == "" || !isRawColor(c) {
		return ""
	}
	return "color:" + c
}

// GetSpec retorna a especificação (ícone e cor) para uma chave.
func GetSpec(key string) IconSpec {
	k := strings.ToLower(strings.TrimSpace(key))
	if k != "" {
		if spec, exists := Config[k]; exists {
			spec.Color = FormatColor(spec.Color)
			return spec
		}
	}
	if spec, exists := Config["nota"]; exists {
		spec.Color = FormatColor(spec.Color)
		return spec
	}
	return IconSpec{Icon: "sticky-note", Color: "#F54927"}
}

// iconColorByIcon é o mapa reverso ícone (lowercased) → cor, construído uma
// única vez de forma determinística (chaves ordenadas) para evitar que
// GetColor dependa da ordem de iteração de mapas em Go (que é aleatória).
var (
	iconColorOnce   sync.Once
	iconColorByIcon map[string]string
)

func buildIconColorMap() {
	iconColorByIcon = make(map[string]string, len(Config))
	keys := make([]string, 0, len(Config))
	for k := range Config {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		icon := strings.ToLower(Config[k].Icon)
		if _, exists := iconColorByIcon[icon]; !exists {
			iconColorByIcon[icon] = FormatColor(Config[k].Color)
		}
	}
}

// GetIcon retorna o nome do ícone para uma chave.
func GetIcon(key string) string {
	return GetSpec(key).Icon
}

// IconSVG retorna o SVG completo de um ícone (nome ou chave de tipo) com a classe
// fornecida e a cor da especificação aplicada — cores raw (hex/rgb/hsl) viram
// style inline, classes Tailwind ficam no atributo class. Paridade com o templ Icon.
func IconSVG(name, class string) string {
	spec := GetSpec(name)
	cleanClass, styleAttr := ParseClassAndStyle(class + " " + spec.Color)
	svg := SVGString(spec.Icon, cleanClass)
	if styleAttr != "" {
		svg = strings.Replace(svg, "<svg ", `<svg style="`+styleAttr+`" `, 1)
	}
	return svg
}

// GetColor retorna a classe ou estilo de cor para uma chave ou nome de ícone.
func GetColor(keyOrIcon string) string {
	k := strings.ToLower(strings.TrimSpace(keyOrIcon))
	if k == "" {
		if spec, exists := Config["nota"]; exists {
			return FormatColor(spec.Color)
		}
		return "#F54927"
	}

	// Chave direta primeiro (mais específica).
	if spec, exists := Config[k]; exists {
		return FormatColor(spec.Color)
	}

	// Nome de ícone → cor (determinístico).
	iconColorOnce.Do(buildIconColorMap)
	if c, exists := iconColorByIcon[k]; exists {
		return c
	}

	if spec, exists := Config["nota"]; exists {
		return FormatColor(spec.Color)
	}
	return "#F54927"
}

// ConfigJSON exporta a configuração de ícones como JSON string para consumo pelo JavaScript frontend.
func ConfigJSON() string {
	var sb strings.Builder
	sb.WriteString("{")
	first := true
	for k, v := range Config {
		if !first {
			sb.WriteString(",")
		}
		sb.WriteString(`"` + k + `":{"Icon":"` + v.Icon + `","Color":"` + FormatColor(v.Color) + `"}`)
		first = false
	}
	sb.WriteString("}")
	return sb.String()
}
