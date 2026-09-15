package icons

import (
	"bytes"
	"context"
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
