package dayplan

import "github.com/a-h/templ"

// styleAttr permite injetar um style dinâmico em templates (o templ não aceita
// `style={ expr }` para valores calculados em todos os contextos).
func styleAttr(css string) templ.Attributes {
	return templ.Attributes{"style": css}
}
