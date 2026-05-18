package api

import (
	"html/template"
	"net/http"
)

// SetTemplates configura os templates compilados (chamado de main.go).
func (ctx *HandlerContext) SetTemplates(tpl *template.Template) {
	ctx.Templates = tpl
}

func (ctx *HandlerContext) render(w http.ResponseWriter, name string, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if ctx.Templates == nil {
		http.Error(w, "templates not loaded", http.StatusInternalServerError)
		return
	}
	if err := ctx.Templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (ctx *HandlerContext) renderPartial(w http.ResponseWriter, name string, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if ctx.Templates == nil {
		http.Error(w, "templates not loaded", http.StatusInternalServerError)
		return
	}
	tpl := ctx.Templates.Lookup(name)
	if tpl == nil {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	if err := tpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (ctx *HandlerContext) renderLogin(w http.ResponseWriter, name string, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if ctx.Templates == nil {
		http.Error(w, "templates not loaded", http.StatusInternalServerError)
		return
	}
	if err := ctx.Templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
