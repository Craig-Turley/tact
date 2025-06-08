package template

// Using this as a type alias for an template string
// possibly might move this later
type Template string

var ALLOWED_TAGS = map[string]bool{
	"html":   true,
	"head":   true,
	"body":   true,
	"style":  true,
	"table":  true,
	"tr":     true,
	"td":     true,
	"th":     true,
	"thead":  true,
	"tbody":  true,
	"tfoot":  true,
	"div":    true,
	"span":   true,
	"p":      true,
	"br":     true,
	"hr":     true,
	"h1":     true,
	"h2":     true,
	"h3":     true,
	"h4":     true,
	"h5":     true,
	"h6":     true,
	"a":      true,
	"img":    true,
	"strong": true,
	"em":     true,
	"b":      true,
	"i":      true,
	"u":      true,
	"ul":     true,
	"ol":     true,
	"li":     true,
	"font":   true,
	"center": true,
	"meta":   true,
}

// TODO implement this
func (t *Template) Sanitize() {
	return
}
