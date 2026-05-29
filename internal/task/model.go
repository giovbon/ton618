package task

import (
	"fmt"
	"time"
)

// ── Tipos ──

type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
	StatusCancelled  Status = "cancelled"
)

type Priority string

const (
	PriorityImportant   Priority = "important"
	PriorityNormal      Priority = "normal"
	PriorityDispensable Priority = "dispensable"
)

type RecurrenceRule string

const (
	RecurrenceWeekly  RecurrenceRule = "weekly"
	RecurrenceMonthly RecurrenceRule = "monthly"
)

// ── Estruturas ──

// Task representa uma tarefa no banco.
type Task struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Status       Status    `json:"status"`
	Priority     Priority  `json:"priority"`
	Category     string    `json:"category"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	AllDay       bool      `json:"all_day"`
	Color        string    `json:"color"`
	RecurrenceID string    `json:"recurrence_id,omitempty"`
	NoteLink     string    `json:"note_link,omitempty"` // ex: "notes/reuniao.md"
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TaskRecurrence define a regra de recorrência de uma tarefa.
type TaskRecurrence struct {
	ID          string         `json:"id"`
	Rule        RecurrenceRule `json:"rule"`
	DaysOfWeek  string         `json:"days_of_week,omitempty"`  // "1,3,5" (seg, qua, sex)
	DaysOfMonth string         `json:"days_of_month,omitempty"` // "1,15"
	Interval    int            `json:"interval"`                // a cada N períodos (1 = toda semana/mês)
	EndDate     time.Time      `json:"end_date"`
}

// TaskInstance é uma ocorrência concreta (real ou expandida) de uma tarefa.
type TaskInstance struct {
	Task
	IsRecurrenceInstance bool   `json:"is_recurrence_instance"`
	OriginalID           string `json:"original_id,omitempty"` // ID da task template
}

// ── Dashboard ──

type CategoryBreakdown struct {
	Category string  `json:"category"`
	Hours    float64 `json:"hours"`
	Color    string  `json:"color"`
}

type PriorityBreakdown struct {
	Priority Priority `json:"priority"`
	Hours    float64  `json:"hours"`
}

type DailyOccupancy struct {
	Date       string  `json:"date"`
	BusyHours  float64 `json:"busy_hours"`
	FreeHours  float64 `json:"free_hours"`
	TotalHours float64 `json:"total_hours"`
}

type DashboardData struct {
	PeriodLabel       string              `json:"period_label"` // "Diário", "Semanal", "Mensal"
	TotalHours        float64             `json:"total_hours"`
	BusyHours         float64             `json:"busy_hours"`
	FreeHours         float64             `json:"free_hours"`
	SleepHours        float64             `json:"sleep_hours"`
	PendingCount      int                 `json:"pending_count"`
	DoneCount         int                 `json:"done_count"`
	ByCategory        []CategoryBreakdown `json:"by_category"`
	ByPriority        []PriorityBreakdown `json:"by_priority"`
	OccupancyHistory  []DailyOccupancy    `json:"occupancy_history"` // histórico desde 01/jan
	UpcomingTasks     []Task              `json:"upcoming_tasks"`    // próximas 5 tarefas
	OverloadedDays    []string            `json:"overloaded_days"`   // dias com >8h ocupadas
}

// ── Cores por categoria (hash determinístico) ──

// CategoryColor gera uma cor HSL determinística a partir do nome da categoria.
func CategoryColor(category string) string {
	if category == "" {
		return "#38bdf8" // sky-400 default
	}
	h := 0
	for _, c := range category {
		h = (h*31 + int(c)) % 360
	}
	return hslToHex(h, 60, 55)
}

// PaletteIndex retorna um índice de cor (0–7) baseado no nome da categoria,
// para usar classes CSS tailwind como `accent0`–`accent7` em gráficos.
func PaletteIndex(category string) int {
	if category == "" {
		return 0
	}
	var sum int
	for _, c := range category {
		sum += int(c)
	}
	return sum % 8
}

func hslToHex(h, s, l int) string {
	// Conversão simplificada — boa o suficiente para cores de categorias
	hf := float64(h%360) / 60.0
	sf := float64(s) / 100.0
	lf := float64(l) / 100.0

	c := (1.0 - abs(2.0*lf-1.0)) * sf
	x := c * (1.0 - abs(mod(hf, 2.0)-1.0))
	m := lf - c/2.0

	var r, g, b float64
	switch {
	case hf < 1:
		r, g, b = c, x, 0
	case hf < 2:
		r, g, b = x, c, 0
	case hf < 3:
		r, g, b = 0, c, x
	case hf < 4:
		r, g, b = 0, x, c
	case hf < 5:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}

	toHex := func(v float64) int {
		return int((v + m) * 255)
	}
	return fmt.Sprintf("#%02x%02x%02x", toHex(r), toHex(g), toHex(b))
}

func mod(a, b float64) float64 {
	return a - float64(int(a/b))*b
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
