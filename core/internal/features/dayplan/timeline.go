package dayplan

import (
	"sort"
	"strconv"
	"strings"
	"unicode"

	"ton618/core/internal/core/domain"
)

// activityColor é a cor única da Rotina (sky-500, o mesmo azul da Agenda).
// Vai embutida no id de cada barra (c0ea5e9-1) para o JS pintar o retângulo.
const activityColor = "0ea5e9"

// Gantt monta o código-fonte Mermaid (gantt) da Rotina do dia.
//
// Uma única seção ("Rotina") — o TON-618 é de uso pessoal, não há pessoas para
// agrupar. A "tarefa-fantasma" `edge` força o eixo a cobrir 00:00–23:59; o JS
// esconde a barra e usa a geometria para desenhar as horas no rodapé.
func Gantt(day string, activities []domain.DayActivity) string {
	if len(activities) == 0 {
		return ""
	}

	sorted := make([]domain.DayActivity, len(activities))
	copy(sorted, activities)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].StartTime == sorted[j].StartTime {
			if sorted[i].EndTime == sorted[j].EndTime {
				return sorted[i].ID < sorted[j].ID
			}
			return sorted[i].EndTime < sorted[j].EndTime
		}
		return sorted[i].StartTime < sorted[j].StartTime
	})

	var b strings.Builder
	b.WriteString("gantt\n")
	// O dateFormat precisa incluir a hora: sem HH:mm o Mermaid não entende o
	// fim da tarefa e desenha barras de largura zero.
	b.WriteString("    dateFormat YYYY-MM-DD HH:mm\n")
	b.WriteString("    axisFormat %Hh\n")
	b.WriteString("    section Rotina\n")

	for i, a := range sorted {
		label := sanitizeMermaid(a.Description)
		b.WriteString("        " + label +
			" :c" + activityColor + "-" + strconv.Itoa(i+1) +
			", " + day + " " + a.StartTime +
			", " + day + " " + a.EndTime + "\n")
	}

	// Tarefa-fantasma: cobre o dia inteiro para o eixo não "inflar" barras
	// curtas. O JS (web/src/rotina.js) esconde esta linha do SVG.
	b.WriteString("    edge :edge0, " + day + " 00:00, " + day + " 23:59\n")
	return b.String()
}

// sanitizeMermaid limpa o texto para uso seguro dentro do gantt: remove
// caracteres que quebram a sintaxe (vírgula, dois-pontos, aspas, quebras de
// linha), colapsa espaços e limita a 80 caracteres.
func sanitizeMermaid(s string) string {
	var b strings.Builder
	lastSpace := false
	for _, r := range s {
		keep := unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) ||
			strings.ContainsRune("áéíóúâêôãõçüÁÉÍÓÚÂÊÔÃÕÇÜ!?/()&+_'-", r)
		if keep {
			if unicode.IsSpace(r) {
				if !lastSpace {
					b.WriteRune(' ')
					lastSpace = true
				}
			} else {
				b.WriteRune(r)
				lastSpace = false
			}
		} else if !lastSpace {
			b.WriteRune(' ')
			lastSpace = true
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		out = "Atividade"
	}
	runes := []rune(out)
	if len(runes) > 80 {
		out = string(runes[:80])
	}
	return out
}
