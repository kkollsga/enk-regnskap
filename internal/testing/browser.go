package apptest

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// Browser er en tynn wrapper rundt net/http som holder session-state
// (cookies), folger redirects og parser HTML-responsen.
type Browser struct {
	t      *testing.T
	base   string
	client *http.Client
}

func newBrowser(t *testing.T, base string) *Browser {
	jar, _ := cookiejar.New(nil)
	return &Browser{
		t:      t,
		base:   base,
		client: &http.Client{Jar: jar},
	}
}

// Doc er en parset HTML-respons med enkle sporringer.
type Doc struct {
	t      *testing.T
	b      *Browser
	Status int
	Body   string
	Header http.Header
	root   *html.Node
}

// Get henter en side.
func (b *Browser) Get(path string) *Doc {
	b.t.Helper()
	resp, err := b.client.Get(b.base + path)
	if err != nil {
		b.t.Fatalf("GET %s: %v", path, err)
	}
	return b.readDoc(resp)
}

// GetRaw henter en side og returnerer status + body uten aa kreve HTML.
func (b *Browser) GetRaw(path string) (int, string, http.Header) {
	b.t.Helper()
	resp, err := b.client.Get(b.base + path)
	if err != nil {
		b.t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body), resp.Header
}

// PostForm sender et POST-skjema (application/x-www-form-urlencoded).
func (b *Browser) PostForm(path string, values url.Values) *Doc {
	b.t.Helper()
	resp, err := b.client.PostForm(b.base+path, values)
	if err != nil {
		b.t.Fatalf("POST %s: %v", path, err)
	}
	return b.readDoc(resp)
}

func (b *Browser) readDoc(resp *http.Response) *Doc {
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	d := &Doc{t: b.t, b: b, Status: resp.StatusCode, Body: string(body), Header: resp.Header}
	if root, err := html.Parse(strings.NewReader(d.Body)); err == nil {
		d.root = root
	}
	return d
}

// --- Enkle CSS-lignende sporringer ---

// Find returnerer alle noder som matcher en enkel selektor: "#id", ".class",
// "tag", eller "tag.class".
func (d *Doc) Find(selector string) []*html.Node {
	if d.root == nil {
		return nil
	}
	var tag, id, class string
	switch {
	case strings.HasPrefix(selector, "#"):
		id = selector[1:]
	case strings.HasPrefix(selector, "."):
		class = selector[1:]
	default:
		if i := strings.IndexByte(selector, '.'); i >= 0 {
			tag, class = selector[:i], selector[i+1:]
		} else {
			tag = selector
		}
	}
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && nodeMatches(n, tag, id, class) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(d.root)
	return out
}

// First returnerer forste node som matcher selektoren, eller nil.
func (d *Doc) First(selector string) *html.Node {
	nodes := d.Find(selector)
	if len(nodes) == 0 {
		return nil
	}
	return nodes[0]
}

// Has sjekker om minst en node matcher selektoren.
func (d *Doc) Has(selector string) bool {
	return len(d.Find(selector)) > 0
}

// Text returnerer den sammenslaatte teksten i forste node som matcher.
func (d *Doc) Text(selector string) string {
	n := d.First(selector)
	if n == nil {
		return ""
	}
	return NodeText(n)
}

func nodeMatches(n *html.Node, tag, id, class string) bool {
	if tag != "" && n.Data != tag {
		return false
	}
	if id != "" && attr(n, "id") != id {
		return false
	}
	if class != "" && !hasClass(n, class) {
		return false
	}
	return true
}

// Attr returnerer verdien til en attributt paa en node (eksportert hjelper).
func Attr(n *html.Node, key string) string { return attr(n, key) }

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, class string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}

// NodeText henter all tekst under en node.
func NodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(b.String())
}
