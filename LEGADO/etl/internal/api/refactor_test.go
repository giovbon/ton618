package api

import (
	"regexp"
	"testing"
)

func TestSemanticLinkReplacementRegex(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		oldTopic string
		newTopic string
		want     string
	}{
		{
			name:     "Simple replacement",
			content:  "Note with @[bunda] link.",
			oldTopic: "bunda",
			newTopic: "bundaz",
			want:     "Note with @[bundaz] link.",
		},
		{
			name:     "Hierarchical child replacement",
			content:  "Note with @[Namespace/bunda] link.",
			oldTopic: "bunda",
			newTopic: "bundaz",
			want:     "Note with @[Namespace/bundaz] link.",
		},
		{
			name:     "Hierarchical parent replacement",
			content:  "Note with @[bunda/Child] link.",
			oldTopic: "bunda",
			newTopic: "bundaz",
			want:     "Note with @[bundaz/Child] link.",
		},
		{
			name:     "Middle segment replacement",
			content:  "Note with @[Grand/bunda/Child] link.",
			oldTopic: "bunda",
			newTopic: "bundaz",
			want:     "Note with @[Grand/bundaz/Child] link.",
		},
		{
			name:     "Should not replace partial match (plural)",
			content:  "Note with @[bundas] link.",
			oldTopic: "bunda",
			newTopic: "bundaz",
			want:     "Note with @[bundas] link.",
		},
		{
			name:     "Should not replace partial match (prefix)",
			content:  "Note with @[kabunda] link.",
			oldTopic: "bunda",
			newTopic: "bundaz",
			want:     "Note with @[kabunda] link.",
		},
		{
			name:     "Multiple occurrences",
			content:  "@[bunda] and @[Other/bunda].",
			oldTopic: "bunda",
			newTopic: "bundaz",
			want:     "@[bundaz] and @[Other/bundaz].",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re := regexp.MustCompile(`(@\[|/)(` + regexp.QuoteMeta(tt.oldTopic) + `)(/|\])`)
			got := re.ReplaceAllStringFunc(tt.content, func(match string) string {
				subMatches := re.FindStringSubmatch(match)
				if len(subMatches) < 4 {
					return match
				}
				prefix := subMatches[1]
				suffix := subMatches[3]
				return prefix + tt.newTopic + suffix
			})
			if got != tt.want {
				t.Errorf("Replacement failed.\nGot:  %s\nWant: %s", got, tt.want)
			}
		})
	}
}
