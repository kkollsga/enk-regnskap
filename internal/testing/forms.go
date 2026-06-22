package apptest

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// Form representerer et HTML-skjema funnet på en side, med standardverdier
// hentet fra feltene.
type Form struct {
	Action string
	Method string
	Values url.Values
	doc    *Doc
}

// Form finner et skjema på siden. selector kan være "" (første skjema),
// "#id", ".class", eller en delstreng av action-attributtet.
func (d *Doc) Form(selector string) *Form {
	d.t.Helper()
	node := d.findFormNode(selector)
	if node == nil {
		d.t.Fatalf("fant ikke skjema for selektor %q", selector)
	}
	f := &Form{
		Action: attr(node, "action"),
		Method: strings.ToUpper(attr(node, "method")),
		Values: url.Values{},
		doc:    d,
	}
	if f.Method == "" {
		f.Method = "POST"
	}
	collectFields(node, f.Values)
	return f
}

func (d *Doc) findFormNode(selector string) *html.Node {
	forms := d.Find("form")
	for _, fn := range forms {
		switch {
		case selector == "":
			return fn
		case strings.HasPrefix(selector, "#"):
			if attr(fn, "id") == selector[1:] {
				return fn
			}
		case strings.HasPrefix(selector, "."):
			if hasClass(fn, selector[1:]) {
				return fn
			}
		default:
			if strings.Contains(attr(fn, "action"), selector) {
				return fn
			}
		}
	}
	return nil
}

// collectFields leser standardverdier fra input/select/textarea.
func collectFields(form *html.Node, vals url.Values) {
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "input":
				name := attr(n, "name")
				typ := strings.ToLower(attr(n, "type"))
				if name != "" && typ != "submit" && typ != "button" {
					if (typ == "checkbox" || typ == "radio") && attr(n, "checked") == "" {
						// uavkrysset - hopp over
					} else {
						vals.Set(name, attr(n, "value"))
					}
				}
			case "textarea":
				if name := attr(n, "name"); name != "" {
					vals.Set(name, NodeText(n))
				}
			case "select":
				if name := attr(n, "name"); name != "" {
					vals.Set(name, selectedOption(n))
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(form)
}

// selectedOption returnerer verdien til valgt option, ellers første option.
func selectedOption(sel *html.Node) string {
	var first, selected string
	got := false
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "option" {
			val := attr(n, "value")
			if val == "" {
				val = NodeText(n)
			}
			if first == "" && !got {
				first = val
			}
			if attr(n, "selected") != "" {
				selected = val
				got = true
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(sel)
	if got {
		return selected
	}
	return first
}

// Set overstyrer en feltverdi.
func (f *Form) Set(name, value string) *Form {
	f.Values.Set(name, value)
	return f
}

// Submit sender skjemaet og returnerer responssiden.
func (f *Form) Submit() *Doc {
	action := f.Action
	if action == "" {
		action = "/"
	}
	return f.doc.b.PostForm(action, f.Values)
}
