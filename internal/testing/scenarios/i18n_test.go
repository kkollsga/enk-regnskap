package scenarios

import (
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/core"
	apptest "github.com/kkollsga/enk-regnskap/internal/testing"
)

// Fase 11: flerspråklig støtte.

func TestDefaultLanguageNorwegian(t *testing.T) {
	h := apptest.Start(t)
	doc := h.Browser().Get("/")
	apptest.AssertHTMLContains(t, doc, "nav", "Oversikt")
}

func TestSwitchToEnglish(t *testing.T) {
	h := apptest.Start(t)
	h.App.SetConfig(h.Context(), core.ConfigLanguage, "en")
	doc := h.Browser().Get("/")
	apptest.AssertHTMLContains(t, doc, "nav", "Overview")
	apptest.AssertHTMLContains(t, doc, "nav", "Expenses")
}

func TestSwitchToPortuguese(t *testing.T) {
	h := apptest.Start(t)
	h.App.SetConfig(h.Context(), core.ConfigLanguage, "pt")
	doc := h.Browser().Get("/")
	apptest.AssertHTMLContains(t, doc, "nav", "Despesas")
}

func TestSetLangEndpointPersists(t *testing.T) {
	h := apptest.Start(t)
	b := h.Browser()
	b.Get("/set-lang?lang=en")
	if got := h.App.Language(h.Context()); got != "en" {
		t.Errorf("språk = %q, forventet en", got)
	}
	// Ugyldig språk ignoreres.
	b.Get("/set-lang?lang=zz")
	if got := h.App.Language(h.Context()); got != "en" {
		t.Errorf("ugyldig språk endret tilstand: %q", got)
	}
}
