package icons

import "strings"

// IconSpec define o par de ícone (Lucide) e cor (Tailwind CSS) de um elemento.
type IconSpec struct {
	Icon  string // Nome do ícone Lucide (ex: "sticky-note", "pencil-ruler", "container")
	Color string // Classe CSS de cor Tailwind (ex: "text-amber-400", "text-zinc-400")
}

// Config é o arquivo centralizador de configuração de ícones e cores do TON-618.
// Edite os ícones (Lucide) e as cores (classes CSS Tailwind) abaixo para alterar o sistema todo:
var Config = map[string]IconSpec{
	// ── TIPOS DE NOTAS E ARQUIVOS ──
	"nota":        {Icon: "sticky-note", Color: "text-amber-400"},
	"markdown":    {Icon: "sticky-note", Color: "text-amber-400"},
	"desenho":     {Icon: "pencil", Color: "text-pink-400"},
	"drawing":     {Icon: "pencil", Color: "text-pink-400"},
	"typst":       {Icon: "book-down", Color: "text-cyan-400"},
	"mermaid":     {Icon: "vector-square", Color: "text-purple-400"},
	"mindmap":     {Icon: "chart-no-axes-gantt", Color: "text-emerald-400"},
	"markmap":     {Icon: "chart-no-axes-gantt", Color: "text-emerald-400"},
	"mapa":        {Icon: "pin", Color: "text-orange-400"},
	"map":         {Icon: "pin", Color: "text-orange-400"},
	"planilha":    {Icon: "table", Color: "text-sky-400"},
	"spreadsheet": {Icon: "table", Color: "text-sky-400"},
	"pdf":         {Icon: "book-text", Color: "text-red-500"},
	"epub":        {Icon: "book-open", Color: "text-indigo-400"},
	"anexo":       {Icon: "package-plus", Color: "text-amber-200"},
	"attachment":  {Icon: "package-plus", Color: "text-amber-200"},
	"arquivo":     {Icon: "archive", Color: "text-zinc-400"},
	"archive":     {Icon: "archive", Color: "text-zinc-400"},
	"youtube":     {Icon: "video", Color: "text-red-500"},
	"artigo":      {Icon: "newspaper", Color: "text-blue-400"},
	"article":     {Icon: "newspaper", Color: "text-blue-400"},

	// ── NAVEGAÇÃO & HEADER ──
	"agenda":    {Icon: "calendar", Color: "text-sky-400"},
	"task":      {Icon: "list-todo", Color: "text-amber-400"},
	"todos":     {Icon: "list-todo", Color: "text-amber-400"},
	"tabulator": {Icon: "layers", Color: "text-sky-400"},
	"captura":   {Icon: "link", Color: "text-emerald-400"},

	// ── UTILIDADES & MENUS ──
	"config":   {Icon: "settings", Color: "text-zinc-400"},
	"settings": {Icon: "settings", Color: "text-zinc-400"},
	"sair":     {Icon: "log-out", Color: "text-zinc-400"},
	"logout":   {Icon: "log-out", Color: "text-zinc-400"},
	"ajuda":    {Icon: "help", Color: "text-zinc-400"},
	"help":     {Icon: "help", Color: "text-zinc-400"},

	// ── AÇÕES E BOTÕES DENTRO DAS NOTAS ──
	"salvar":   {Icon: "save", Color: "text-emerald-400"},
	"save":     {Icon: "save", Color: "text-emerald-400"},
	"arquivar": {Icon: "archive", Color: "text-zinc-500"},
	"duplicar": {Icon: "copy", Color: "text-zinc-500"},
	"excluir":  {Icon: "trash", Color: "text-zinc-500"},
	"delete":   {Icon: "trash", Color: "text-zinc-500"},
}

// GetSpec retorna a especificação (ícone e cor) para uma chave.
func GetSpec(key string) IconSpec {
	k := strings.ToLower(strings.TrimSpace(key))
	if spec, exists := Config[k]; exists {
		return spec
	}
	return IconSpec{Icon: "sticky-note", Color: "text-amber-400"}
}

// GetIcon retorna o nome do ícone para uma chave.
func GetIcon(key string) string {
	return GetSpec(key).Icon
}

// GetColor retorna a classe Tailwind de cor para uma chave ou nome de ícone.
func GetColor(keyOrIcon string) string {
	k := strings.ToLower(strings.TrimSpace(keyOrIcon))
	if spec, exists := Config[k]; exists {
		return spec.Color
	}
	// Busca por nome do ícone caso a chave passada seja o nome do ícone em si
	for _, spec := range Config {
		if spec.Icon == k {
			return spec.Color
		}
	}
	return "text-zinc-400"
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
		sb.WriteString(`"` + k + `":{"Icon":"` + v.Icon + `","Color":"` + v.Color + `"}`)
		first = false
	}
	sb.WriteString("}")
	return sb.String()
}
