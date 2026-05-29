package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"ton618/internal/task"
)

// ── Página do Dashboard ──

func (ctx *HandlerContext) HandleTasksPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":        "Agenda - TON-618",
		"ContentBlock": "tasksContent",
		"Year":         time.Now().Year(),
	}
	ctx.render(w, "tasks.html", data)
}

func (ctx *HandlerContext) HandleTaskListPage(w http.ResponseWriter, r *http.Request) {
	ctx.render(w, "tasks.html", map[string]interface{}{
		"Title":        "Tarefas - TON-618",
		"ContentBlock": "taskListContent",
	})
}

func (ctx *HandlerContext) HandleAllTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := ctx.Tasks.GetAllTasks()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks": tasks,
	})
}

// ── API CRUD ──

func (ctx *HandlerContext) HandleListTasks(w http.ResponseWriter, r *http.Request) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	now := time.Now()
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	to := from.AddDate(0, 0, 7)

	if fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = t
		}
	}
	if toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = t
		}
	}

	tasks, err := ctx.Tasks.ListTasks(from, to)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	recurrences := loadRecurrences(ctx.Tasks, tasks)
	instances := task.ExpandRecurrences(tasks, recurrences, from, to)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks": instances,
		"from":  from.Format(time.RFC3339),
		"to":    to.Format(time.RFC3339),
	})
}

func (ctx *HandlerContext) HandleCreateTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var input struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    string `json:"priority"`
		Category    string `json:"category"`
		StartTime   string `json:"start_time"`
		EndTime     string `json:"end_time"`
		AllDay      bool   `json:"all_day"`
		Recurrence  *struct {
			Rule        string `json:"rule"`
			DaysOfWeek  string `json:"days_of_week"`
			DaysOfMonth string `json:"days_of_month"`
			Interval    int    `json:"interval"`
			EndDate     string `json:"end_date"`
		} `json:"recurrence"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if input.Title == "" || input.StartTime == "" || input.EndTime == "" {
		http.Error(w, "title, start_time, end_time required", http.StatusBadRequest)
		return
	}

	startTime, err := time.Parse(time.RFC3339, input.StartTime)
	if err != nil {
		http.Error(w, "invalid start_time", http.StatusBadRequest)
		return
	}
	endTime, err := time.Parse(time.RFC3339, input.EndTime)
	if err != nil {
		http.Error(w, "invalid end_time", http.StatusBadRequest)
		return
	}

	t := task.Task{
		ID:          generateID(),
		Title:       input.Title,
		Description: input.Description,
		Status:      task.StatusPending,
		Priority:    task.Priority(input.Priority),
		Category:    input.Category,
		StartTime:   startTime,
		EndTime:     endTime,
		AllDay:      input.AllDay,
		Color:       task.CategoryColor(input.Category),
	}

	if t.Priority == "" {
		t.Priority = task.PriorityNormal
	}

	if input.Recurrence != nil && (input.Recurrence.Rule == "weekly" || input.Recurrence.Rule == "monthly") {
		recID := generateID()
		endDate := time.Now().AddDate(1, 0, 0)
		if input.Recurrence.EndDate != "" {
			if dt, err := time.Parse(time.RFC3339, input.Recurrence.EndDate); err == nil {
				endDate = dt
			}
		}
		interval := input.Recurrence.Interval
		if interval < 1 {
			interval = 1
		}
		rec := task.TaskRecurrence{
			ID:          recID,
			Rule:        task.RecurrenceRule(input.Recurrence.Rule),
			DaysOfWeek:  input.Recurrence.DaysOfWeek,
			DaysOfMonth: input.Recurrence.DaysOfMonth,
			Interval:    interval,
			EndDate:     endDate,
		}
		if err := ctx.Tasks.CreateRecurrence(rec); err != nil {
			slog.Error("create recurrence", "error", err)
			http.Error(w, "failed to create recurrence", http.StatusInternalServerError)
			return
		}
		t.RecurrenceID = recID
	}

	if err := ctx.Tasks.CreateTask(t); err != nil {
		slog.Error("create task", "error", err)
		http.Error(w, "failed to create task", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(t)
}

func (ctx *HandlerContext) HandleUpdateTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	var input struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Status      string `json:"status"`
		Priority    string `json:"priority"`
		Category    string `json:"category"`
		StartTime   string `json:"start_time"`
		EndTime     string `json:"end_time"`
		AllDay      bool   `json:"all_day"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	t, err := ctx.Tasks.GetTask(id)
	if err != nil || t == nil {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	// Update common fields
	if input.Title != "" {
		t.Title = input.Title
	}
	if input.Description != "" {
		t.Description = input.Description
	}
	if input.Status != "" {
		t.Status = task.Status(input.Status)
	}
	if input.Priority != "" {
		t.Priority = task.Priority(input.Priority)
	}
	if input.Category != "" {
		t.Category = input.Category
		t.Color = task.CategoryColor(input.Category)
	}
	if input.StartTime != "" {
		if st, err := time.Parse(time.RFC3339, input.StartTime); err == nil {
			t.StartTime = st
		}
	}
	if input.EndTime != "" {
		if et, err := time.Parse(time.RFC3339, input.EndTime); err == nil {
			t.EndTime = et
		}
	}
	t.AllDay = input.AllDay

	if err := ctx.Tasks.UpdateTask(*t); err != nil {
		slog.Error("update task", "error", err)
		http.Error(w, "failed to update task", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

func (ctx *HandlerContext) HandleDeleteTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	if err := ctx.Tasks.DeleteTask(id); err != nil {
		slog.Error("delete task", "error", err)
		http.Error(w, "failed to delete task", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (ctx *HandlerContext) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	now := time.Now()
	now = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var from, to time.Time
	switch period {
	case "day":
		from = now
		to = now.AddDate(0, 0, 1)
	case "week":
		weekday := now.Weekday()
		if weekday == time.Sunday {
			weekday = 7
		}
		from = now.AddDate(0, 0, -int(weekday)+1)
		to = from.AddDate(0, 0, 7)
	case "month":
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		to = from.AddDate(0, 1, 0)
	default:
		from = now
		to = now.AddDate(0, 0, 7)
	}

	// O dashboard Dashboard() usa o db.Store diretamente
	data, err := task.Dashboard(ctx.Store, from, to, period)
	if err != nil {
		slog.Error("dashboard", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// ── Helpers ──

func generateID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func loadRecurrences(store *task.Store, tasks []task.Task) map[string]*task.TaskRecurrence {
	result := make(map[string]*task.TaskRecurrence)
	for _, t := range tasks {
		if t.RecurrenceID == "" {
			continue
		}
		if _, ok := result[t.RecurrenceID]; ok {
			continue
		}
		rec, err := store.GetRecurrence(t.RecurrenceID)
		if err != nil || rec == nil {
			continue
		}
		result[t.RecurrenceID] = rec
	}
	return result
}

func (ctx *HandlerContext) HandleTaskCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := ctx.Tasks.GetDistinctCategories()
	if err != nil {
		categories = nil
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"categories": categories,
	})
}
