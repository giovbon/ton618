package task

import (
	"fmt"
	"log/slog"
	"time"

	"ton618/internal/db"
)

const defaultSleepHours = 8.0
const overloadThreshold = 8.0 // horas

// Dashboard retorna os dados agregados para o painel de tarefas.
func Dashboard(store *db.Store, from, to time.Time, periodLabel string) (*DashboardData, error) {
	data := &DashboardData{
		PeriodLabel: periodLabel,
	}

	// Horas de sono (configurável nas settings)
	sleepHours := getSleepHours(store)

	// Calcula horas totais no período (total - sono)
	daysInPeriod := to.Sub(from).Hours() / 24
	if daysInPeriod < 1 {
		daysInPeriod = 1
	}
	data.TotalHours = daysInPeriod * 24
	data.SleepHours = daysInPeriod * sleepHours

	// Query de agregação: horas ocupadas por tarefas no período
	row := store.DB.QueryRow(`
		SELECT COALESCE(SUM(
			(julianday(MIN(end_time, ?)) - julianday(MAX(start_time, ?))) * 24
		), 0)
		FROM tasks
		WHERE start_time < ? AND end_time > ?
		  AND status != 'cancelled'
	`, to.Format(time.RFC3339), from.Format(time.RFC3339),
		to.Format(time.RFC3339), from.Format(time.RFC3339))
	row.Scan(&data.BusyHours)

	// Expande recorrências para contabilizar horas de instâncias virtuais
	taskStore := NewStore(store)
	periodTasks, err := taskStore.ListTasks(from, to)
	if err != nil {
		slog.Error("dashboard list tasks for recurrence", "error", err)
	} else {
		seen := make(map[string]bool)
		var recIDs []string
		for _, t := range periodTasks {
			if t.RecurrenceID != "" && !seen[t.RecurrenceID] {
				seen[t.RecurrenceID] = true
				recIDs = append(recIDs, t.RecurrenceID)
			}
		}
		if len(recIDs) > 0 {
			recurrences := make(map[string]*TaskRecurrence)
			for _, rid := range recIDs {
				rec, err := taskStore.GetRecurrence(rid)
				if err != nil {
					slog.Error("dashboard get recurrence", "id", rid, "error", err)
					continue
				}
				if rec != nil {
					recurrences[rid] = rec
				}
			}
			if len(recurrences) > 0 {
				instances := ExpandRecurrences(periodTasks, recurrences, from, to)
				var totalRecurrenceHours float64
				for _, inst := range instances {
					if inst.IsRecurrenceInstance {
						totalRecurrenceHours += inst.EndTime.Sub(inst.StartTime).Hours()
					}
				}
				data.BusyHours += totalRecurrenceHours
			}
		}
	}

	data.FreeHours = data.TotalHours - data.BusyHours - data.SleepHours
	if data.FreeHours < 0 {
		data.FreeHours = 0
	}

	// Contagem de pendentes e concluídas
	store.DB.QueryRow("SELECT COUNT(*) FROM tasks WHERE status = 'pending' OR status = 'in_progress'").Scan(&data.PendingCount)
	store.DB.QueryRow("SELECT COUNT(*) FROM tasks WHERE status = 'done'").Scan(&data.DoneCount)

	// Breakdown por categoria
	data.ByCategory = queryCategoryBreakdown(store, from, to)

	// Breakdown por prioridade
	data.ByPriority = queryPriorityBreakdown(store, from, to)

	// Histórico de ocupação desde o começo do ano
	data.OccupancyHistory = queryOccupancyHistory(store, sleepHours)

	// Próximas tarefas
	upcoming, err := taskStore.GetUpcomingTasks(5)
	if err != nil {
		slog.Error("dashboard upcoming", "error", err)
	} else {
		data.UpcomingTasks = upcoming
	}

	// Dias sobrecarregados
	data.OverloadedDays = queryOverloadedDays(store, from, to)

	return data, nil
}

func queryCategoryBreakdown(store *db.Store, from, to time.Time) []CategoryBreakdown {
	rows, err := store.DB.Query(`
		SELECT category, COALESCE(ROUND(SUM(
			(CASE WHEN all_day THEN 24 ELSE (julianday(MIN(end_time, ?)) - julianday(MAX(start_time, ?))) * 24 END)
		), 1), 0) AS hours
		FROM tasks
		WHERE start_time < ? AND end_time > ?
		  AND status != 'cancelled'
		GROUP BY category
		ORDER BY hours DESC
	`, to.Format(time.RFC3339), from.Format(time.RFC3339),
		to.Format(time.RFC3339), from.Format(time.RFC3339))
	if err != nil {
		slog.Error("category breakdown", "error", err)
		return nil
	}
	defer rows.Close()

	var result []CategoryBreakdown
	for rows.Next() {
		var cb CategoryBreakdown
		rows.Scan(&cb.Category, &cb.Hours)
		cb.Color = CategoryColor(cb.Category)
		result = append(result, cb)
	}
	return result
}

func queryPriorityBreakdown(store *db.Store, from, to time.Time) []PriorityBreakdown {
	rows, err := store.DB.Query(`
		SELECT priority, COALESCE(ROUND(SUM(
			(CASE WHEN all_day THEN 24 ELSE (julianday(MIN(end_time, ?)) - julianday(MAX(start_time, ?))) * 24 END)
		), 1), 0) AS hours
		FROM tasks
		WHERE start_time < ? AND end_time > ?
		  AND status != 'cancelled'
		GROUP BY priority
		ORDER BY hours DESC
	`, to.Format(time.RFC3339), from.Format(time.RFC3339),
		to.Format(time.RFC3339), from.Format(time.RFC3339))
	if err != nil {
		slog.Error("priority breakdown", "error", err)
		return nil
	}
	defer rows.Close()

	var result []PriorityBreakdown
	for rows.Next() {
		var pb PriorityBreakdown
		var p string
		rows.Scan(&p, &pb.Hours)
		pb.Priority = Priority(p)
		result = append(result, pb)
	}
	return result
}

// queryOccupancyHistory gera uma série diária de ocupação desde 01/jan do ano atual.
// Cada dia mostra busy_hours, free_hours (24 - busy_hours - sleep).
func queryOccupancyHistory(store *db.Store, sleepHours float64) []DailyOccupancy {
	now := time.Now()
	startOfYear := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())

	// Gera todos os dias do ano até hoje
	var history []DailyOccupancy
	cursor := startOfYear
	for !cursor.After(now) {
		dayStart := cursor
		dayEnd := cursor.AddDate(0, 0, 1)

		var busyHours float64
		store.DB.QueryRow(`
			SELECT COALESCE(ROUND(SUM(
				(julianday(MIN(end_time, ?)) - julianday(MAX(start_time, ?))) * 24
			), 1), 0)
			FROM tasks
			WHERE start_time < ? AND end_time > ?
			  AND status != 'cancelled'
		`, dayEnd.Format(time.RFC3339), dayStart.Format(time.RFC3339),
			dayEnd.Format(time.RFC3339), dayStart.Format(time.RFC3339)).Scan(&busyHours)

		totalHours := 24.0
		freeHours := totalHours - busyHours - sleepHours
		if freeHours < 0 {
			freeHours = 0
		}

		history = append(history, DailyOccupancy{
			Date:       cursor.Format("2006-01-02"),
			BusyHours:  busyHours,
			FreeHours:  freeHours,
			TotalHours: totalHours - sleepHours,
		})

		cursor = dayEnd
	}

	return history
}

func queryOverloadedDays(store *db.Store, from, to time.Time) []string {
	rows, err := store.DB.Query(`
		SELECT date(start_time) AS day
		FROM tasks
		WHERE start_time >= ? AND start_time <= ?
		  AND status != 'cancelled'
		GROUP BY day
		HAVING SUM((julianday(end_time) - julianday(start_time)) * 24) > ?
		ORDER BY day
	`, from.Format(time.RFC3339), to.Format(time.RFC3339), overloadThreshold)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var days []string
	for rows.Next() {
		var d string
		rows.Scan(&d)
		days = append(days, d)
	}
	return days
}

func getSleepHours(store *db.Store) float64 {
	val, err := store.GetSetting("sleep_hours")
	if err != nil || val == "" {
		return defaultSleepHours
	}
	var hours float64
	if _, err := fmt.Sscanf(val, "%f", &hours); err != nil {
		return defaultSleepHours
	}
	if hours < 0 || hours > 16 {
		return defaultSleepHours
	}
	return hours
}
