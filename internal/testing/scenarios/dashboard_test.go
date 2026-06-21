package scenarios

import (
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/core"
	apptest "github.com/kkollsga/enk-regnskap/internal/testing"
)

// Fase 7: dashboard.

func TestDashboardReflectsTransactions(t *testing.T) {
	h := apptest.Start(t)
	h.App.SetConfig(h.Context(), core.ConfigActiveYear, "2025")
	ctx := h.Context()

	h.App.AddIncome(ctx, core.ActorWeb, core.IncomeInput{
		Date: "2025-02-01", Description: "Inntekt A", Currency: "NOK",
		CountryCode: "NO", AmountOrig: 100000, Category: "tjenesteinntekt",
	})
	h.App.AddIncome(ctx, core.ActorWeb, core.IncomeInput{
		Date: "2025-03-01", Description: "Inntekt B", Currency: "NOK",
		CountryCode: "NO", AmountOrig: 50000, Category: "honorar",
	})
	h.App.AddExpense(ctx, core.ActorWeb, core.ExpenseInput{
		Date: "2025-02-15", Description: "Utstyr", Category: "smaa_driftsmidler",
		AmountNOK: 20000,
	})

	doc := h.Browser().Get("/")
	apptest.AssertStatus(t, doc, 200)
	// Inntekt 150 000, fradrag 20 000, resultat 130 000.
	apptest.AssertHTMLContains(t, doc, "[data-live=income]", "150 000")
	apptest.AssertHTMLContains(t, doc, "[data-live=expenses]", "20 000")
	apptest.AssertBodyContains(t, doc, "130 000")
}

func TestDashboardQuickActionsNavigate(t *testing.T) {
	h := apptest.Start(t)
	doc := h.Browser().Get("/")
	hrefs := map[string]bool{}
	for _, a := range doc.Find("a") {
		hrefs[apptest.Attr(a, "href")] = true
	}
	for _, want := range []string{"/income/new", "/expenses/new"} {
		if !hrefs[want] {
			t.Errorf("dashboard mangler snarvei til %s", want)
		}
	}
}

func TestDashboardEstimatedTax(t *testing.T) {
	h := apptest.Start(t)
	h.App.SetConfig(h.Context(), core.ConfigActiveYear, "2025")
	// Resultat 700 000 -> estimert skatt vises.
	h.App.AddIncome(h.Context(), core.ActorWeb, core.IncomeInput{
		Date: "2025-01-10", Description: "Stor inntekt", Currency: "NOK",
		CountryCode: "NO", AmountOrig: 700000, Category: "tjenesteinntekt",
	})
	doc := h.Browser().Get("/")
	apptest.AssertBodyContains(t, doc, "Estimert skatt")
	// Trygdeavgift for 700k i 2025 = 76 300.
	apptest.AssertBodyContains(t, doc, "76 300")
}

func TestYearSwitcherPersists(t *testing.T) {
	h := apptest.Start(t)
	b := h.Browser()
	b.Get("/set-year?year=2024")
	if got := h.App.ActiveYear(h.Context()); got != 2024 {
		t.Errorf("aktivt aar = %d, forventet 2024 etter bytte", got)
	}
}
