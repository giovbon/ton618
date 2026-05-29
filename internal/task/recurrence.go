package task

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// ExpandRecurrences gera instâncias virtuais de tarefas recorrentes para um período.
func ExpandRecurrences(tasks []Task, recurrences map[string]*TaskRecurrence, from, to time.Time) []TaskInstance {
	var instances []TaskInstance

	for _, t := range tasks {
		instances = append(instances, TaskInstance{Task: t, IsRecurrenceInstance: false, OriginalID: t.ID})
	}

	for _, t := range tasks {
		if t.RecurrenceID == "" {
			continue
		}
		rec, ok := recurrences[t.RecurrenceID]
		if !ok {
			continue
		}
		expanded := expandTemplate(t, rec, from, to)
		instances = append(instances, expanded...)
	}

	sort.Slice(instances, func(i, j int) bool {
		return instances[i].StartTime.Before(instances[j].StartTime)
	})

	return instances
}

func expandTemplate(template Task, rec *TaskRecurrence, from, to time.Time) []TaskInstance {
	rangeStart := template.StartTime
	if from.After(rangeStart) {
		rangeStart = from
	}
	rangeEnd := rec.EndDate
	if to.Before(rangeEnd) {
		rangeEnd = to
	}
	if rangeEnd.Before(rangeStart) {
		return nil
	}

	duration := template.EndTime.Sub(template.StartTime)

	switch rec.Rule {
	case RecurrenceWeekly:
		return expandWeekly(template, rec, rangeStart, rangeEnd, duration)
	case RecurrenceMonthly:
		return expandMonthly(template, rec, rangeStart, rangeEnd, duration)
	}
	return nil
}

func expandWeekly(template Task, rec *TaskRecurrence, from, to time.Time, duration time.Duration) []TaskInstance {
	days := parseDaysOfWeek(rec.DaysOfWeek)
	if len(days) == 0 {
		days = []time.Weekday{template.StartTime.Weekday()}
	}
	daySet := make(map[time.Weekday]bool)
	for _, d := range days {
		daySet[d] = true
	}

	var instances []TaskInstance
	cursor := from
	for !cursor.After(to) && !cursor.After(rec.EndDate) {
		if daySet[cursor.Weekday()] {
			weeksSinceStart := int(cursor.Sub(template.StartTime).Hours() / (24 * 7))
			if weeksSinceStart >= 0 && weeksSinceStart%rec.Interval == 0 && !sameDay(cursor, template.StartTime) {
				newTask := template
				newTask.ID = template.ID + "-" + cursor.Format("2006-01-02")
				newTask.StartTime = time.Date(cursor.Year(), cursor.Month(), cursor.Day(),
					template.StartTime.Hour(), template.StartTime.Minute(), 0, 0, template.StartTime.Location())
				newTask.EndTime = newTask.StartTime.Add(duration)
				instances = append(instances, TaskInstance{
					Task:                 newTask,
					IsRecurrenceInstance: true,
					OriginalID:           template.ID,
				})
			}
		}
		cursor = cursor.AddDate(0, 0, 1)
	}
	return instances
}

func expandMonthly(template Task, rec *TaskRecurrence, from, to time.Time, duration time.Duration) []TaskInstance {
	days := parseDaysOfMonth(rec.DaysOfMonth)
	if len(days) == 0 {
		days = []int{template.StartTime.Day()}
	}

	var instances []TaskInstance
	// Começa no mês do template e avança rec.Interval meses por vez
	cursorMonth := template.StartTime
	for !cursorMonth.After(rec.EndDate) && !cursorMonth.After(to) {
		for _, day := range days {
			year, month := cursorMonth.Year(), cursorMonth.Month()
			lastDay := daysInMonth(year, month)
			targetDay := day
			if targetDay > lastDay {
				targetDay = lastDay
			}
			instanceDate := time.Date(year, month, targetDay,
				template.StartTime.Hour(), template.StartTime.Minute(), 0, 0, template.StartTime.Location())

			if instanceDate.Before(from) || instanceDate.After(to) || instanceDate.After(rec.EndDate) {
				continue
			}
			if sameDay(instanceDate, template.StartTime) {
				continue
			}

			newTask := template
			newTask.ID = template.ID + "-" + instanceDate.Format("2006-01-02")
			newTask.StartTime = instanceDate
			newTask.EndTime = instanceDate.Add(duration)
			instances = append(instances, TaskInstance{
				Task:                 newTask,
				IsRecurrenceInstance: true,
				OriginalID:           template.ID,
			})
		}
		cursorMonth = cursorMonth.AddDate(0, rec.Interval, 0)
	}
	return instances
}

// ── Helpers ──

func parseDaysOfWeek(s string) []time.Weekday {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	days := make([]time.Weekday, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 0 || n > 6 {
			continue
		}
		days = append(days, time.Weekday(n))
	}
	return days
}

func parseDaysOfMonth(s string) []int {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	days := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 1 || n > 31 {
			continue
		}
		days = append(days, n)
	}
	return days
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
