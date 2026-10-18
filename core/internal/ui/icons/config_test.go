package icons

import (
	"bytes"
	"context"
	"os"
	"regexp"
	"strings"
	"testing"
)

// calendarDaysMarker é um path exclusivo do ícone Lucide "calendar-days"
// (a fileira de pontinhos do meio). Serve para provar que o SVG renderizado
// é realmente o ícone semanal e não o fallback genérico.
const calendarDaysMarker = "M12 14h.01"

func TestConfig_Semanal(t *testing.T) {
	if got := GetIcon("semanal"); got != "calendar-days" {
		t.Errorf("GetIcon(semanal) = %q, want %q", got, "calendar-days")
	}
	if got := GetColor("semanal"); got != "#14B8A6" {
		t.Errorf("GetColor(semanal) = %q, want %q", got, "#14B8A6")
	}
	// A cor deve ser exclusiva do tipo semanal (não colidir com outros tipos).
	if got := GetColor("agenda"); got == GetColor("semanal") {
		t.Errorf("cor da nota semanal não deveria ser igual à da agenda (%q)", got)
	}
}

// TestWeeklyIcon_RendersInBothRenderers garante que o ícone do tipo semanal
// existe nos DOIS renderizadores de SVG: o templ (server-side, usado pelo
// navbar/sidebar) e o SVGString (usado por IconSVG no JSON da busca/banco).
// Esquecer um deles faz a mesma nota aparecer com ícones diferentes por tela.
func TestWeeklyIcon_RendersInBothRenderers(t *testing.T) {
	raw := SVGString("semanal", "w-4 h-4")
	if !strings.Contains(raw, calendarDaysMarker) {
		t.Errorf("SVGString(semanal) não rendeu o ícone calendar-days (fallback?): %s", raw)
	}

	var buf bytes.Buffer
	// Mesmo uso dos chamadores reais (navbar/sidebar): classe + cor da spec.
	if err := Icon("semanal", "w-4 h-4 "+GetColor("semanal")).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render do templ Icon: %v", err)
	}
	rendered := buf.String()
	if !strings.Contains(rendered, calendarDaysMarker) {
		t.Errorf("templ Icon(semanal) não rendeu o ícone calendar-days (fallback?): %s", rendered)
	}
	if !strings.Contains(rendered, `class="w-4 h-4"`) || !strings.Contains(rendered, "color:#14B8A6") {
		t.Errorf("classe/cor não aplicadas no Icon(semanal): %s", rendered)
	}
}

// TestIconSVG_SemanalAplicaCor garante que o caminho IconSVG (usado no JSON de
// busca/banco de dados) injeta a cor da spec como style inline.
func TestIconSVG_SemanalAplicaCor(t *testing.T) {
	svg := IconSVG("semanal", "w-3 h-3")
	if !strings.Contains(svg, calendarDaysMarker) {
		t.Errorf("IconSVG(semanal) não rendeu o ícone calendar-days: %s", svg)
	}
	if !strings.Contains(svg, "color:#14B8A6") {
		t.Errorf("IconSVG(semanal) não aplicou a cor esperada: %s", svg)
	}
}

// caseLabels casa os rótulos de `case` dos dois renderizadores
// (`case "a", "b":`) para extrair os NOMES de ícone suportados.
var caseLabels = regexp.MustCompile(`case\s+((?:"[^"]+"\s*(?:,\s*)?)+)\s*:`)
var caseNames = regexp.MustCompile(`"([^"]+)"`)

// iconNamesFromSource lê os nomes de ícone do switch de um arquivo-fonte.
func iconNamesFromSource(t *testing.T, path string) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ler %s: %v", path, err)
	}
	names := make(map[string]bool)
	for _, m := range caseLabels.FindAllStringSubmatch(string(data), -1) {
		for _, n := range caseNames.FindAllStringSubmatch(m[1], -1) {
			names[n[1]] = true
		}
	}
	if len(names) == 0 {
		t.Fatalf("nenhum case de ícone encontrado em %s", path)
	}
	return names
}

// TestIconNames_ParidadeEntreRenderizadores garante que os DOIS renderizadores
// conhecem exatamente os mesmos nomes de ícone: `icons.templ` (usado por navbar,
// sidebar e /database) e `icons.go`/SVGString (usado pelo JSON do banco/busca,
// via IconSVG). Um case presente em apenas um dos lados faz a mesma nota aparecer
// com o ícone genérico de arquivo (fallback do switch) em uma das telas: foi
// assim que o tipo `tabulator` (ícone "container") ficou sem ícone próprio na
// tabela do /database — o case existia só no templ.
func TestIconNames_ParidadeEntreRenderizadores(t *testing.T) {
	viaString := iconNamesFromSource(t, "icons.go")   // SVGString
	viaTempl := iconNamesFromSource(t, "icons.templ") // Icon

	for n := range viaTempl {
		if !viaString[n] {
			t.Errorf("ícone %q só existe no icons.templ — falta o case em SVGString (icons.go)", n)
		}
	}
	for n := range viaString {
		if !viaTempl[n] {
			t.Errorf("ícone %q só existe em SVGString (icons.go) — falta o case no icons.templ", n)
		}
	}
}

// TestConfig_IconesExistemNosRenderizadores impede que uma chave do Config aponte
// para um ícone inexistente (GetIcon/GetColor cairiam no fallback em silêncio).
// Cobre todos os tipos de nota e chaves de UI.
func TestConfig_IconesExistemNosRenderizadores(t *testing.T) {
	viaString := iconNamesFromSource(t, "icons.go")
	viaTempl := iconNamesFromSource(t, "icons.templ")

	for key, spec := range Config {
		if spec.Icon == "" {
			t.Errorf("chave %q sem ícone definido", key)
			continue
		}
		if !viaString[spec.Icon] || !viaTempl[spec.Icon] {
			t.Errorf("chave %q aponta para o ícone %q, inexistente (SVGString: %v, templ: %v)",
				key, spec.Icon, viaString[spec.Icon], viaTempl[spec.Icon])
		}
	}
}
