package scenarios

import (
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/core"
	apptest "github.com/kkollsga/enk-regnskap/internal/testing"
)

// Steg 6: skatteinfo og landoversikt.

func TestTaxInfoShowsDeductionCategories(t *testing.T) {
	h := apptest.Start(t)
	h.App.SetConfig(h.Context(), core.ConfigActiveYear, "2025")
	doc := h.Browser().Get("/tax-info")
	apptest.AssertStatus(t, doc, 200)
	// Fradragskategoriene vises i nedtrekksmenyen.
	apptest.AssertBodyContains(t, doc, "Kostnader hjemmekontor (standardfradrag)")
	apptest.AssertBodyContains(t, doc, "js-ded-select")
	// Utenlandsk skatt hører hjemme under «Utenlandsk skatt», ikke her.
	apptest.AssertBodyNotContains(t, doc, "Skatteavtale med Norge")
	apptest.AssertBodyNotContains(t, doc, "COFINS")
}

func TestTaxInfoSummarizesBookedDeductions(t *testing.T) {
	h := apptest.Start(t)
	h.App.SetConfig(h.Context(), core.ConfigActiveYear, "2025")
	if _, err := h.App.AddExpense(h.Context(), core.ActorWeb, core.ExpenseInput{
		Date: "2025-03-10", Description: "Kontorrekvisita", Category: "kontorrekvisita",
		AmountOrig: 1234, TaxYear: 2025,
	}); err != nil {
		t.Fatal(err)
	}
	doc := h.Browser().Get("/tax-info")
	// Bokført fradrag for året skal summeres i info-boksen.
	apptest.AssertBodyContains(t, doc, "1 234")
}

func TestTaxInfoShowsDeductionRates(t *testing.T) {
	h := apptest.Start(t)
	h.App.SetConfig(h.Context(), core.ConfigActiveYear, "2025")
	doc := h.Browser().Get("/tax-info")
	// Hjemmekontor sjablong 2025 = 2 192.
	apptest.AssertBodyContains(t, doc, "2 192")
	// Trygdeavgift næring 2025 = 10,9 %.
	apptest.AssertBodyContains(t, doc, "10,9")
}

func TestTaxInfoRatesPerYear(t *testing.T) {
	h := apptest.Start(t)
	h.App.SetConfig(h.Context(), core.ConfigActiveYear, "2024")
	doc := h.Browser().Get("/tax-info")
	// Hjemmekontor sjablong 2024 = 2 128.
	apptest.AssertBodyContains(t, doc, "2 128")
}
