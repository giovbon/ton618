package template

import (
	"embed"
	"strings"
	"text/template"
)

//go:embed *.html
var TemplatesFS embed.FS

// LoadTemplates loads all embedded .html templates.
// Uses text/template to avoid Go 1.24+ html/template strict parsing issues.
func LoadTemplates(funcMap template.FuncMap) (*template.Template, error) {
	data, err := TemplatesFS.ReadFile("layout.html")
	if err != nil {
		return nil, err
	}

	combined := string(data)

	for _, name := range []string{"index.html", "editor.html",
		"login.html", "search_results.html"} {

		data, err := TemplatesFS.ReadFile(name)
		if err != nil {
			return nil, err
		}
		content := string(data)

		if strings.HasPrefix(content, "{{template") {
			// Has template call at start + defines inside
			// Split into filename wrapper + defines
			idx := strings.Index(content, "{{define")
			if idx > 0 {
				tmCall := content[:idx]
				defines := content[idx:]
				combined += "\n{{define \"" + name + "\"}}" + tmCall + "{{end}}\n"
				combined += defines + "\n"
			} else {
				combined += "\n{{define \"" + name + "\"}}\n" + content + "\n{{end}}\n"
			}
		} else if strings.HasPrefix(content, "{{define") {
			combined += "\n" + content + "\n"
		} else {
			combined += "\n{{define \"" + name + "\"}}\n" + content + "\n{{end}}\n"
		}
	}

	return template.New("").Funcs(funcMap).Parse(combined)
}
