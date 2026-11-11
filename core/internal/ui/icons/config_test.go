package icons

import (
	"bytes"
	"context"
	"os"
	"regexp"
	"strings"
	"testing"
)

// calculatorMarker é um path exclusivo do ícone Lucide "calculator"
// (a primeira tecla da fileira de cima). Serve para provar que o SVG
// renderizado é realmente o ícone do tipo cálculo e não o fallback genérico.
const calculatorMarker = "M8 10h.01"

func TestConfig_Calculo(t *testing.T) {
	if got := GetIcon("calculo"); got != "calculator" {
		t.Errorf("GetIcon(calculo) = %q, want %q", got, "calculator")
	}
	if got := GetColor("calculo"); got != "#F97316" {
		t.Errorf("GetColor(calculo) = %q, want %q", got, "#F97316")
	}
	// A cor deve ser exclusiva do tipo cálculo (não colidir com outros tipos).
	if got := GetColor("agenda"); got == GetColor("calculo") {
		t.Errorf("cor da nota de cálculo não deveria ser igual à da agenda (%q)", got)
	}
}

// TestCalculoIcon_RendersInBothRenderers garante que o ícone do tipo cálculo
// existe nos DOIS renderizadores de SVG: o templ (server-side, usado pelo
// navbar/sidebar) e o SVGString (usado por IconSVG no JSON da busca/banco).
// Esquecer um deles faz a mesma nota aparecer com ícones diferentes por tela.
func TestCalculoIcon_RendersInBothRenderers(t *testing.T) {
	raw := SVGString("calculo", "w-4 h-4")
	if !strings.Contains(raw, calculatorMarker) {
		t.Errorf("SVGString(calculo) não rendeu o ícone calculator (fallback?): %s", raw)
	}

	var buf bytes.Buffer
	// Mesmo uso dos chamadores reais (navbar/sidebar): classe + cor da spec.
	if err := Icon("calculo", "w-4 h-4 "+GetColor("calculo")).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render do templ Icon: %v", err)
	}
	rendered := buf.String()
	if !strings.Contains(rendered, calculatorMarker) {
		t.Errorf("templ Icon(calculo) não rendeu o ícone calculator (fallback?): %s", rendered)
	}
	if !strings.Contains(rendered, `class="w-4 h-4"`) || !strings.Contains(rendered, "color:#F97316") {
		t.Errorf("classe/cor não aplicadas no Icon(calculo): %s", rendered)
	}
}

// TestIconSVG_CalculoAplicaCor garante que o caminho IconSVG (usado no JSON de
// busca/banco de dados) injeta a cor da spec como style inline.
func TestIconSVG_CalculoAplicaCor(t *testing.T) {
	svg := IconSVG("calculo", "w-3 h-3")
	if !strings.Contains(svg, calculatorMarker) {
		t.Errorf("IconSVG(calculo) não rendeu o ícone calculator: %s", svg)
	}
	if !strings.Contains(svg, "color:#F97316") {
		t.Errorf("IconSVG(calculo) não aplicou a cor esperada: %s", svg)
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
