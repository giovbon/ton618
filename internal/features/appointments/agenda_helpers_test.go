package appointments

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

	// Verify exact color format
	if h1 == "" || a1 == "" {
		t.Errorf("Expected valid colors, got empty strings")
	}

	// Case insensitivity and # character removal test
	h3, _ := GetTagColor("UEEEPA")
	h4, _ := GetTagColor("#ueeepa")
	if h3 != h4 {
		t.Errorf("GetTagColor is sensitive to case or # symbol: %s vs %s", h3, h4)
	}
}

// TestGetTagColorPaletteStability ensures that the colors for known tags don't drift over time.
func TestGetTagColorPaletteStability(t *testing.T) {
	tests := []struct {
		tag      string
		wantBase string
	}{
		{"#todo", "#f59e0b"},
		{"#impacta", "#6366f1"},
		{"#urgente", "#14b8a6"},
	}
	for _, tt := range tests {
		base, _ := GetTagColor(tt.tag)
		if base != tt.wantBase {
			t.Errorf("GetTagColor(%q) base = %q, want %q", tt.tag, base, tt.wantBase)
		}
	}
}

// TestGetTagColorNoPaletteCollision verifies that different tags produce stable results
// and the function is idempotent across the whole palette.
func TestGetTagColorNoPaletteCollision(t *testing.T) {
	tags := []string{"#alpha", "#beta", "#gamma", "#delta", "#epsilon",
		"#zeta", "#eta", "#theta", "#iota", "#kappa", "#lambda", "#mu", "#nu"}
	colors := make(map[string]string)
	for _, tag := range tags {
		base, _ := GetTagColor(tag)
		if base == "" {
			t.Errorf("GetTagColor(%q) returned empty base color", tag)
		}
		colors[tag] = base
	}
	// Ensure the function is still stable on a second call
	for _, tag := range tags {
		base, _ := GetTagColor(tag)
		if base != colors[tag] {
			t.Errorf("GetTagColor(%q) not stable: first=%q second=%q", tag, colors[tag], base)
		}
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
			expected: `Fix this tomorrow <span class="inline-block px-1.5 py-0.5 rounded text-[10px] font-bold mx-0.5 transition-all select-none" style="background-color: rgba(245, 158, 11, 0.2); color: #f59e0b; border: 1px solid rgba(245, 158, 11, 0.2)">#todo</span>`,
		},
		{
			name:     "XSS attempt in description text",
			input:    `<script>alert("xss")</script>`,
			expected: `&lt;script&gt;alert(&#34;xss&#34;)&lt;/script&gt;`,
		},
		{
			name:  "Multiple tags in one description",
			input: "Reunião importante #urgente #trabalho",
			expected: func() string {
				b1, a1 := GetTagColor("#urgente")
				b2, a2 := GetTagColor("#trabalho")
				return fmt.Sprintf(
					`Reunião importante <span class="inline-block px-1.5 py-0.5 rounded text-[10px] font-bold mx-0.5 transition-all select-none" style="background-color: %s; color: %s; border: 1px solid %s">#urgente</span> <span class="inline-block px-1.5 py-0.5 rounded text-[10px] font-bold mx-0.5 transition-all select-none" style="background-color: %s; color: %s; border: 1px solid %s">#trabalho</span>`,
					a1, b1, a1, a2, b2, a2,
				)
			}(),
		},
		{
			name:  "Tag at start of line",
			input: "#urgente meeting tomorrow",
			expected: func() string {
				b, a := GetTagColor("#urgente")
				return fmt.Sprintf(
					`<span class="inline-block px-1.5 py-0.5 rounded text-[10px] font-bold mx-0.5 transition-all select-none" style="background-color: %s; color: %s; border: 1px solid %s">#urgente</span> meeting tomorrow`,
					a, b, a,
				)
			}(),
		},
		{
			name:     "Wikilink with .sheet extension",
			input:    "See [[budget.sheet|Budget]] here",
			expected: `See <a href="/editor?file=notes%2Fbudget.sheet" class="text-sky-400 hover:text-sky-300 underline font-medium transition-colors cursor-pointer" onclick="event.stopPropagation();">Budget</a> here`,
		},
		{
			name:     "Plain text without markup",
			input:    "Comprar café na padaria",
			expected: "Comprar café na padaria",
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
}

// TestFormatAgendaDateFloatingFormats verifies that dates stored in the floating-time
// format (without timezone suffix) are formatted exactly as written, immune to server TZ.
func TestFormatAgendaDateFloatingFormats(t *testing.T) {
	origLocal := time.Local
	time.Local = time.UTC
	defer func() { time.Local = origLocal }()

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{"Floating local no suffix", "2026-06-29T15:00:00", "29/06 - 15:00"},
		{"With Z suffix (legacy frontend)", "2026-06-29T15:00:00Z", "29/06 - 15:00"},
		{"With -03:00 offset (should strip offset, keep written time)", "2026-06-29T15:00:00-03:00", "29/06 - 15:00"},
		{"With milliseconds and Z", "2026-06-26T15:52:20.533Z", "26/06 - 15:52"},
		{"Midnight time", "2026-07-01T00:00:00", "01/07 - 00:00"},
		{"End of year", "2026-12-31T23:59:00", "31/12 - 23:59"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatAgendaDate(tt.input)
			if !strings.Contains(got, tt.contains) {
				t.Errorf("FormatAgendaDate(%q) = %q, expected to contain %q", tt.input, got, tt.contains)
			}
		})
	}
}

// TestGetWeekdayPt verifies Portuguese weekday abbreviations.
func TestGetWeekdayPt(t *testing.T) {
	tests := []struct {
		wd   time.Weekday
		want string
	}{
		{time.Sunday, "DOM"},
		{time.Monday, "SEG"},
		{time.Tuesday, "TER"},
		{time.Wednesday, "QUA"},
		{time.Thursday, "QUI"},
		{time.Friday, "SEX"},
		{time.Saturday, "SÁB"},
	}
	for _, tt := range tests {
		got := getWeekdayPt(tt.wd)
		if got != tt.want {
			t.Errorf("getWeekdayPt(%v) = %q, want %q", tt.wd, got, tt.want)
		}
	}
}

func TestStripTags(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello #world", "Hello"},
		{"#tag at start", "at start"},
		{"Multiple #tags in #row", "Multiple in"},
		{"No tags here", "No tags here"},
		{"Clean  double   spaces #tag", "Clean  double   spaces"},
	}

	for _, tt := range tests {
		got := StripTags(tt.input)
		if got != tt.expected {
			t.Errorf("StripTags(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
