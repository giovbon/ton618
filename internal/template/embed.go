package template

import "embed"

//go:embed *.html
var TemplatesFS embed.FS
