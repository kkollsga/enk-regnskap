package apptest

import (
	"strings"
	"testing"
)

// AssertStatus feiler hvis statuskoden ikke stemmer.
func AssertStatus(t *testing.T, d *Doc, want int) {
	t.Helper()
	if d.Status != want {
		t.Errorf("status = %d, forventet %d\nbody: %s", d.Status, want, truncate(d.Body, 400))
	}
}

// AssertBodyContains feiler hvis body ikke inneholder delstrengen.
func AssertBodyContains(t *testing.T, d *Doc, substr string) {
	t.Helper()
	if !strings.Contains(d.Body, substr) {
		t.Errorf("body inneholder ikke %q\nbody: %s", substr, truncate(d.Body, 600))
	}
}

// AssertBodyNotContains feiler hvis body inneholder delstrengen.
func AssertBodyNotContains(t *testing.T, d *Doc, substr string) {
	t.Helper()
	if strings.Contains(d.Body, substr) {
		t.Errorf("body inneholder %q, men skulle ikke det\nbody: %s", substr, truncate(d.Body, 600))
	}
}

// AssertHas feiler hvis ingen node matcher selektoren.
func AssertHas(t *testing.T, d *Doc, selector string) {
	t.Helper()
	if !d.Has(selector) {
		t.Errorf("fant ingen element for selektor %q", selector)
	}
}

// AssertHTMLContains feiler hvis elementet (selektor) ikke inneholder teksten.
func AssertHTMLContains(t *testing.T, d *Doc, selector, expected string) {
	t.Helper()
	for _, n := range d.Find(selector) {
		if strings.Contains(NodeText(n), expected) {
			return
		}
	}
	t.Errorf("ingen element %q inneholder teksten %q", selector, expected)
}

// AssertEqual er en generisk likhetssjekk.
func AssertEqual[T comparable](t *testing.T, got, want T, msg string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: fikk %v, forventet %v", msg, got, want)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
