package layout

import (
	"fmt"
	"html"
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// GetTagColor generates a deterministic HSL color based on the tag string
func GetTagColor(tag string) (string, string) {
	var hash int32 = 0
	clean := strings.TrimSpace(strings.ReplaceAll(strings.ToLower(tag), "#", ""))
	for i := 0; i < len(clean); i++ {
		hash = int32(clean[i]) + ((hash << 5) - hash)
	}
	hue := int(math.Abs(float64(hash))) % 360
	base := fmt.Sprintf("hsl(%d, 80%%, 65%%)", hue)
	alpha := fmt.Sprintf("hsla(%d, 80%%, 65%%, 0.2)", hue)
	return base, alpha
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
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return dateStr
	}
	t = t.Local()
	weekday := getWeekdayPt(t.Weekday())
	now := time.Now()
	if t.Year() == now.Year() {
		return fmt.Sprintf("%s, %s", weekday, t.Format("02/01 - 15:04"))
	}
	return fmt.Sprintf("%s, %s", weekday, t.Format("02/01/2006 - 15:04"))
}
