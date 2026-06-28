package appointments

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"
	"ton618/internal/core/timeutil"
)

// GetTagColor generates a distinct color from a predefined aesthetic palette based on the tag string
func GetTagColor(tag string) (string, string) {
	colors := []struct {
		Base  string
		Alpha string
	}{
		{"#f43f5e", "rgba(244, 63, 94, 0.2)"},
		{"#ec4899", "rgba(236, 72, 153, 0.2)"},
		{"#d946ef", "rgba(217, 70, 239, 0.2)"},
		{"#a855f7", "rgba(168, 85, 247, 0.2)"},
		{"#8b5cf6", "rgba(139, 92, 246, 0.2)"},
		{"#6366f1", "rgba(99, 102, 241, 0.2)"},
		{"#14b8a6", "rgba(20, 184, 166, 0.2)"},
		{"#10b981", "rgba(16, 185, 129, 0.2)"},
		{"#22c55e", "rgba(34, 197, 94, 0.2)"},
		{"#84cc16", "rgba(132, 204, 22, 0.2)"},
		{"#eab308", "rgba(234, 179, 8, 0.2)"},
		{"#f59e0b", "rgba(245, 158, 11, 0.2)"},
		{"#f97316", "rgba(249, 115, 22, 0.2)"},
	}

	var hash int32 = 0
	clean := strings.TrimSpace(strings.ReplaceAll(strings.ToLower(tag), "#", ""))
	for i := 0; i < len(clean); i++ {
		hash = int32(clean[i]) + ((hash << 5) - hash)
	}
	
	if hash < 0 {
		hash = -hash
	}
	idx := int(hash) % len(colors)
	return colors[idx].Base, colors[idx].Alpha
}

var wikiLinkRegex = regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)
var tagRegex = regexp.MustCompile(`(^|\s)(#[\w\-]+)`)

// FormatDescription formats description text, escaping HTML and converting wikilinks and tags
func FormatDescription(desc string) string {
	if desc == "" {
		return ""
	}
	
	// Escape HTML
	h := html.EscapeString(desc)
	
	// Process wikilinks
	h = wikiLinkRegex.ReplaceAllStringFunc(h, func(match string) string {
		submatches := wikiLinkRegex.FindStringSubmatch(match)
		noteTarget := submatches[1]
		alias := submatches[2]
		
		displayName := strings.TrimSpace(alias)
		if displayName == "" {
			displayName = strings.TrimSpace(noteTarget)
		}
		
		targetFile := strings.TrimSpace(noteTarget)
		if !strings.HasSuffix(targetFile, ".md") && !strings.HasSuffix(targetFile, ".typ") && !strings.HasSuffix(targetFile, ".sheet") {
			targetFile += ".md"
		}
		if !strings.HasPrefix(targetFile, "notes/") {
			targetFile = "notes/" + targetFile
		}
		
		urlPath := fmt.Sprintf("/editor?file=%s", url.QueryEscape(targetFile))
		return fmt.Sprintf(`<a href="%s" class="text-sky-400 hover:text-sky-300 underline font-medium transition-colors cursor-pointer" onclick="event.stopPropagation();">%s</a>`, urlPath, displayName)
	})
	
	// Process tags
	h = tagRegex.ReplaceAllStringFunc(h, func(match string) string {
		submatches := tagRegex.FindStringSubmatch(match)
		space := submatches[1]
		tag := submatches[2]
		
		baseColor, alphaColor := GetTagColor(tag)
		return fmt.Sprintf(`%s<span class="inline-block px-1.5 py-0.5 rounded text-[10px] font-bold mx-0.5 transition-all select-none" style="background-color: %s; color: %s; border: 1px solid %s">%s</span>`, space, alphaColor, baseColor, alphaColor, tag)
	})
	
	return h
}

func getWeekdayPt(wd time.Weekday) string {
	switch wd {
	case time.Sunday:    return "DOM"
	case time.Monday:    return "SEG"
	case time.Tuesday:   return "TER"
	case time.Wednesday: return "QUA"
	case time.Thursday:  return "QUI"
	case time.Friday:    return "SEX"
	case time.Saturday:  return "SÁB"
	}
	return ""
}

// FormatAgendaDate formats the date as DD/MM/YYYY - HH:MM or DD/MM - HH:MM if current year, prefixed with Portuguese weekday abbreviation
func FormatAgendaDate(dateStr string) string {
	t, err := timeutil.ParseFloatingTime(dateStr)
	if err != nil {
		return dateStr
	}
	
	weekday := getWeekdayPt(t.Weekday())
	now := time.Now()
	if t.Year() == now.Year() {
		return fmt.Sprintf("%s, %s", weekday, t.Format("02/01 - 15:04"))
	}
	return fmt.Sprintf("%s, %s", weekday, t.Format("02/01/2006 - 15:04"))
}

// StripTags removes hashtag patterns from the description text
func StripTags(desc string) string {
	stripTagRegex := regexp.MustCompile(`\s*#[\w\-]+`)
	res := stripTagRegex.ReplaceAllString(desc, "")
	return strings.TrimSpace(res)
}

// FormatWeekLabel returns the week label string in the format "Semana DD mmm", e.g. "Semana 29 jun"
func FormatWeekLabel(year, weekNumber int) string {
	// ISO week starts on Monday
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	wd := jan4.Weekday()
	daysToSubtract := int(wd) - 1
	if wd == time.Sunday {
		daysToSubtract = 6
	}
	isoWeek1Start := jan4.AddDate(0, 0, -daysToSubtract)
	mondayOfTargetWeek := isoWeek1Start.AddDate(0, 0, (weekNumber-1)*7)

	monthsPtShort := []string{"jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"}
	monthIndex := int(mondayOfTargetWeek.Month()) - 1
	monthStr := "des"
	if monthIndex >= 0 && monthIndex < 12 {
		monthStr = monthsPtShort[monthIndex]
	}

	return fmt.Sprintf("Semana %d %s", mondayOfTargetWeek.Day(), monthStr)
}
