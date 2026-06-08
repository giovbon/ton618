package template

import "embed"

//go:embed help.md
var TemplatesFS embed.FS

// HelpMD is the raw content of the embedded help.md documentation file.
func HelpMD() ([]byte, error) {
	return TemplatesFS.ReadFile("help.md")
}
