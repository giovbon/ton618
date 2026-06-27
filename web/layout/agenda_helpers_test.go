package layout

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestGetTagColor(t *testing.T) {
	// Determinism test
	h1, a1 := GetTagColor("#ueeepa")
	h2, a2 := GetTagColor("#ueeepa")
	if h1 != h2 || a1 != a2 {
		t.Errorf("GetTagColor is not deterministic: %s vs %s", h1, h2)
	}

	// Verify exact color matches (Hue 47 for #ueeepa)
	if !strings.Contains(h1, "hsl(47,") {
		t.Errorf("Expected #ueeepa to have HSL hue 47, got %s", h1)
	}

	// Case insensitivity and # character removal test
	h3, _ := GetTagColor("UEEEPA")
	h4, _ := GetTagColor("#ueeepa")
	if h3 != h4 {
		t.Errorf("GetTagColor is sensitive to case or # symbol: %s vs %s", h3, h4)
	}
}

func TestFormatDescription(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Plain text escaping",
			input:    "Hello <world> & friends",
			expected: "Hello &lt;world&gt; &amp; friends",
		},
		{
			name:     "Wikilink default",
			input:    "See [[my-note]] here",
			expected: `See <a href="/editor?file=notes%2Fmy-note.md" class="text-sky-400 hover:text-sky-300 underline font-medium transition-colors cursor-pointer" onclick="event.stopPropagation();">my-note</a> here`,
		},
		{
			name:     "Wikilink with alias",
			input:    "See [[my-note|Alternative Name]] here",
			expected: `See <a href="/editor?file=notes%2Fmy-note.md" class="text-sky-400 hover:text-sky-300 underline font-medium transition-colors cursor-pointer" onclick="event.stopPropagation();">Alternative Name</a> here`,
		},
		{
			name:     "Wikilink with custom folder or extension",
			input:    "See [[notes/another.typ|Another]] here",
			expected: `See <a href="/editor?file=notes%2Fanother.typ" class="text-sky-400 hover:text-sky-300 underline font-medium transition-colors cursor-pointer" onclick="event.stopPropagation();">Another</a> here`,
		},
		{
			name:     "Simple Tag matching",
			input:    "Fix this tomorrow #todo",
			expected: `Fix this tomorrow <span class="inline-block px-1.5 py-0.5 rounded text-[10px] font-bold mx-0.5 transition-all select-none" style="background-color: hsla(198, 80%, 65%, 0.2); color: hsl(198, 80%, 65%); border: 1px solid hsla(198, 80%, 65%, 0.2)">#todo</span>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDescription(tt.input)
			if got != tt.expected {
				t.Errorf("FormatDescription() =\n%s\nwant\n%s", got, tt.expected)
			}
		})
	}
}

func TestFormatAgendaDate(t *testing.T) {
	origLocal := time.Local
	time.Local = time.UTC
	defer func() { time.Local = origLocal }()

	now := time.Now()
	
	// Test current year
	currentYearTime := time.Date(now.Year(), time.January, 15, 14, 30, 0, 0, time.UTC)
	gotCurrent := FormatAgendaDate(currentYearTime.Format(time.RFC3339))
	expectedCurrent := fmt.Sprintf("%s, 15/01 - 14:30", getWeekdayPt(currentYearTime.Weekday()))
	if gotCurrent != expectedCurrent {
		t.Errorf("FormatAgendaDate for current year = %q, want %q", gotCurrent, expectedCurrent)
	}

	// Test different year
	otherYearTime := time.Date(2024, time.March, 10, 9, 15, 0, 0, time.UTC)
	gotOther := FormatAgendaDate(otherYearTime.Format(time.RFC3339))
	expectedOther := fmt.Sprintf("%s, 10/03/2024 - 09:15", getWeekdayPt(otherYearTime.Weekday()))
	if gotOther != expectedOther {
		t.Errorf("FormatAgendaDate for different year = %q, want %q", gotOther, expectedOther)
	}

	// Test invalid date format fallback
	gotInvalid := FormatAgendaDate("invalid-date-string")
	if gotInvalid != "invalid-date-string" {
		t.Errorf("FormatAgendaDate for invalid date = %q, want invalid-date-string", gotInvalid)
	}

	// Test timezone conversion: UTC date parsed and localized to UTC-3
	time.Local = time.FixedZone("UTC-3", -3*60*60)
	utcTime := time.Date(now.Year(), time.January, 15, 14, 30, 0, 0, time.UTC)
	gotConverted := FormatAgendaDate(utcTime.Format(time.RFC3339))
	expectedConverted := fmt.Sprintf("%s, 15/01 - 11:30", getWeekdayPt(utcTime.Local().Weekday()))
	if gotConverted != expectedConverted {
		t.Errorf("FormatAgendaDate timezone conversion = %q, want %q", gotConverted, expectedConverted)
	}
}
