package dayplan

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"ton618/core/internal/core/domain"
)

const (
	minutesPerDay = 24 * 60
	dateLayout    = "2006-01-02"
	timeLayout    = "15:04"
)

// timeInterval é um intervalo válido em minutos desde 00:00.
type timeInterval struct{ start, end int }

// Summary resume o dia da Rotina: a UNIÃO dos intervalos (sem contar
// sobreposições duas vezes) dentro de 24h.
type Summary struct {
	OccupiedMinutes int
	FreeMinutes     int
	ActivityCount   int
	// Overlapping é o número de atividades que colidem com pelo menos outra.
	// O tempo continua sendo contado uma vez só — o número serve de aviso.
	Overlapping int
}

// Occupied formata o total ocupado, ex.: "6h 30min".
func (s Summary) Occupied() string { return formatMinutes(s.OccupiedMinutes) }

// Free formata o total livre dentro das 24h, ex.: "17h 30min".
func (s Summary) Free() string { return formatMinutes(s.FreeMinutes) }

// OccupiedPct devolve a porcentagem do dia ocupada (0–100), para a barra.
func (s Summary) OccupiedPct() int {
	total := s.OccupiedMinutes + s.FreeMinutes
	if total <= 0 {
		return 0
	}
	return int(math.Round(float64(s.OccupiedMinutes) * 100 / float64(total)))
}

// OverlapNote descreve as sobreposições, ou "" quando não há conflito.
func (s Summary) OverlapNote() string {
	switch s.Overlapping {
	case 0:
		return ""
	case 1:
		return "1 atividade se sobrepõe a outra no mesmo horário."
	default:
		return fmt.Sprintf("%d atividades se sobrepõem entre si no mesmo horário.", s.Overlapping)
	}
}

// ActivityCountLabel devolve "1 atividade" / "N atividades" / "nenhuma atividade".
func (s Summary) ActivityCountLabel() string {
	switch s.ActivityCount {
	case 0:
		return "nenhuma atividade"
	case 1:
		return "1 atividade"
	default:
		return fmt.Sprintf("%d atividades", s.ActivityCount)
	}
}

// OccupiedWidth devolve a largura da barra de ocupação (ex.: "42%").
func (s Summary) OccupiedWidth() string {
	return strconv.Itoa(s.OccupiedPct()) + "%"
}

// Summarize calcula o resumo a partir das atividades. Intervalos inválidos
// (horário fora de 00:00–23:59 ou fim <= início) são ignorados — o handler já
// os rejeita, mas o resumo nunca deve quebrar por causa de dado antigo.
func Summarize(activities []domain.DayActivity) Summary {
	valid := make([]timeInterval, 0, len(activities))

	for _, a := range activities {
		start, okStart := parseClockMinutes(a.StartTime)
		end, okEnd := parseClockMinutes(a.EndTime)
		if !okStart || !okEnd || end <= start {
			continue
		}
		if start < 0 {
			start = 0
		}
		if end > minutesPerDay {
			end = minutesPerDay
		}
		if end <= start {
			continue
		}
		valid = append(valid, timeInterval{start: start, end: end})
	}

	occupied := 0
	if len(valid) > 0 {
		sort.Slice(valid, func(i, j int) bool {
			if valid[i].start == valid[j].start {
				return valid[i].end < valid[j].end
			}
			return valid[i].start < valid[j].start
		})
		curStart, curEnd := valid[0].start, valid[0].end
		for _, iv := range valid[1:] {
			if iv.start <= curEnd {
				if iv.end > curEnd {
					curEnd = iv.end
				}
				continue
			}
			occupied += curEnd - curStart
			curStart, curEnd = iv.start, iv.end
		}
		occupied += curEnd - curStart
	}

	free := minutesPerDay - occupied
	if free < 0 {
		free = 0
	}

	return Summary{
		OccupiedMinutes: occupied,
		FreeMinutes:     free,
		ActivityCount:   len(activities),
		Overlapping:     countOverlapping(valid),
	}
}

// countOverlapping conta quantas atividades colidem com pelo menos outra.
func countOverlapping(intervals []timeInterval) int {
	count := 0
	for i := range intervals {
		for j := range intervals {
			if i == j {
				continue
			}
			if intervals[i].start < intervals[j].end && intervals[j].start < intervals[i].end {
				count++
				break
			}
		}
	}
	return count
}

// parseClockMinutes converte "HH:MM" (ou "H:MM") em minutos desde 00:00.
func parseClockMinutes(hhmm string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(hhmm), ":")
	if len(parts) != 2 {
		return 0, false
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, false
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, false
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

// formatMinutes imprime minutos como "6h 30min", "6h" ou "30min".
func formatMinutes(total int) string {
	if total < 0 {
		total = 0
	}
	h := total / 60
	m := total % 60
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%dh %02dmin", h, m)
	case h > 0:
		return fmt.Sprintf("%dh", h)
	default:
		return fmt.Sprintf("%dmin", m)
	}
}

// DateLabel formata "YYYY-MM-DD" como "25/10/2026". Em datas inválidas devolve
// a entrada original (não esconder o problema do usuário).
func DateLabel(day string) string {
	t, err := parseDay(day)
	if err != nil {
		return day
	}
	return t.Format("02/01/2006")
}

// WeekdayLabel devolve o dia da semana curto em português (ex.: "dom").
func WeekdayLabel(day string) string {
	t, err := parseDay(day)
	if err != nil {
		return ""
	}
	return []string{"dom", "seg", "ter", "qua", "qui", "sex", "sáb"}[int(t.Weekday())]
}

// parseDay valida uma data no formato YYYY-MM-DD.
func parseDay(day string) (time.Time, error) {
	return time.Parse(dateLayout, strings.TrimSpace(day))
}
