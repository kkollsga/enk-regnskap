package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"

	"github.com/kkollsga/enk-regnskap/web"
)

// View er datamodellen som sendes til hver mal.
type View struct {
	Lang         string
	T            map[string]string // oversettelser for valgt sprak
	Active       string            // aktiv navigasjonsside
	Title        string
	Year         int
	Years        []int
	MirrorDir    string
	ProjectName  string
	MultiProject bool
	Data         any // sidespesifikk data
}

// Tr slaar opp en oversettelse, med nokkelen som fallback.
func (v View) Tr(key string) string {
	if s, ok := v.T[key]; ok {
		return s
	}
	return key
}

// Renderer parser embeddede maler og holder oversettelser.
type Renderer struct {
	pages map[string]*template.Template
	i18n  map[string]map[string]string
}

// NewRenderer laster i18n og parser alle sidemaler sammen med layouten.
func NewRenderer() (*Renderer, error) {
	r := &Renderer{
		pages: map[string]*template.Template{},
		i18n:  map[string]map[string]string{},
	}
	if err := r.loadI18n(); err != nil {
		return nil, err
	}
	if err := r.loadTemplates(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Renderer) loadI18n() error {
	entries, err := fs.ReadDir(web.I18n, "i18n")
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := web.I18n.ReadFile("i18n/" + e.Name())
		if err != nil {
			return err
		}
		var m map[string]string
		if err := json.Unmarshal(b, &m); err != nil {
			return fmt.Errorf("tolk %s: %w", e.Name(), err)
		}
		lang := strings.TrimSuffix(e.Name(), ".json")
		r.i18n[lang] = m
	}
	if _, ok := r.i18n["nb"]; !ok {
		return fmt.Errorf("mangler i18n/nb.json")
	}
	return nil
}

func (r *Renderer) loadTemplates() error {
	layout, err := web.Templates.ReadFile("templates/layout.html")
	if err != nil {
		return err
	}
	entries, err := fs.ReadDir(web.Templates, "templates")
	if err != nil {
		return err
	}
	funcs := template.FuncMap{
		"nok": formatNOK,
		"pct": formatPct,
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || name == "layout.html" || !strings.HasSuffix(name, ".html") {
			continue
		}
		pageBytes, err := web.Templates.ReadFile("templates/" + name)
		if err != nil {
			return err
		}
		t := template.New("layout").Funcs(funcs)
		if _, err := t.Parse(string(layout)); err != nil {
			return fmt.Errorf("parse layout for %s: %w", name, err)
		}
		if _, err := t.Parse(string(pageBytes)); err != nil {
			return fmt.Errorf("parse %s: %w", name, err)
		}
		page := strings.TrimSuffix(name, ".html")
		r.pages[page] = t
	}
	return nil
}

// translations returnerer oversettelser for et sprak, med fallback til nb.
func (r *Renderer) translations(lang string) map[string]string {
	if m, ok := r.i18n[lang]; ok {
		return m
	}
	return r.i18n["nb"]
}

// Render skriver siden page til w. Feiler malen, sendes 500.
func (r *Renderer) Render(w http.ResponseWriter, page string, v View) {
	t, ok := r.pages[page]
	if !ok {
		http.Error(w, "ukjent side: "+page, http.StatusInternalServerError)
		return
	}
	if v.T == nil {
		v.T = r.translations(v.Lang)
	}
	// Render til buffer forst slik at en feil ikke gir halv-skrevet respons.
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", v); err != nil {
		http.Error(w, "malfeil: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}
