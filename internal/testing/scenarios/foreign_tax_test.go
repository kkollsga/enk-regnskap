package scenarios

import (
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/core"
	apptest "github.com/kkollsga/enk-regnskap/internal/testing"
)

// Steg 4: utenlandsk skatt.

func seedBrazilIncome(t *testing.T, h *apptest.Harness, year int, date string, amount, tax float64) {
	t.Helper()
	h.Mock.AddRate("BRL", date, 2.00)
	_, err := h.App.AddIncome(h.Context(), core.ActorWeb, core.IncomeInput{
		Date: date, Description: "Brasiliansk inntekt", Currency: "BRL",
		CountryCode: "BR", AmountOrig: amount, Category: "tjenesteinntekt",
		TaxYear: year, ForeignTaxPaid: core.ForeignTaxYes, ForeignTaxOrig: tax,
		ForeignTaxCurrency: "BRL", ForeignTaxType: "IRRF",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestForeignTaxPageAggregatesBrazil(t *testing.T) {
	h := apptest.Start(t)
	h.App.SetConfig(h.Context(), core.ConfigActiveYear, "2025")
	// To brasilianske inntekter i 2025 -> aggregeres.
	seedBrazilIncome(t, h, 2025, "2025-03-10", 10000, 1500)
	seedBrazilIncome(t, h, 2025, "2025-06-20", 5000, 750)

	doc := h.Browser().Get("/foreign-tax")
	apptest.AssertStatus(t, doc, 200)
	// Samlet inntekt: (10000+5000)*2.00 = 30000 NOK
	apptest.AssertBodyContains(t, doc, "30 000")
	apptest.AssertBodyContains(t, doc, "Brasil")
}

func TestForeignTaxLegalBasisShownPerYear(t *testing.T) {
	h := apptest.Start(t)

	// 2025: skatteavtale (treaty)
	h.App.SetConfig(h.Context(), core.ConfigActiveYear, "2025")
	seedBrazilIncome(t, h, 2025, "2025-04-01", 10000, 1500)
	doc25 := h.Browser().Get("/foreign-tax")
	apptest.AssertBodyContains(t, doc25, "Skatteavtale")

	// 2024: intern rett
	h.App.SetConfig(h.Context(), core.ConfigActiveYear, "2024")
	seedBrazilIncome(t, h, 2024, "2024-04-01", 10000, 1500)
	doc24 := h.Browser().Get("/foreign-tax")
	apptest.AssertBodyContains(t, doc24, "§ 16-20")
}

func TestForeignTaxChecklistShowsTaxTypes(t *testing.T) {
	h := apptest.Start(t)
	h.App.SetConfig(h.Context(), core.ConfigActiveYear, "2025")
	seedBrazilIncome(t, h, 2025, "2025-04-01", 10000, 1500)
	doc := h.Browser().Get("/foreign-tax")
	// Sjekklisten skal vise IRRF (krediterbar) og COFINS (normalt nei).
	apptest.AssertBodyContains(t, doc, "IRRF")
	apptest.AssertBodyContains(t, doc, "COFINS")
	apptest.AssertBodyContains(t, doc, "RF-1147")
}

func TestForeignTaxNoTaxWarning(t *testing.T) {
	h := apptest.Start(t)
	h.App.SetConfig(h.Context(), core.ConfigActiveYear, "2025")
	h.Mock.AddRate("BRL", "2025-05-01", 2.00)
	// Inntekt uten dokumentert skatt.
	h.App.AddIncome(h.Context(), core.ActorWeb, core.IncomeInput{
		Date: "2025-05-01", Description: "Brasil uten skatt", Currency: "BRL",
		CountryCode: "BR", AmountOrig: 8000, Category: "tjenesteinntekt",
		TaxYear: 2025, ForeignTaxPaid: core.ForeignTaxNo,
	})
	doc := h.Browser().Get("/foreign-tax")
	apptest.AssertBodyContains(t, doc, "skattepliktig i Norge")
}

func TestForeignTaxStatusUpdate(t *testing.T) {
	h := apptest.Start(t)
	seedBrazilIncome(t, h, 2025, "2025-04-01", 10000, 1500)
	// Marker som klar for RF-1147 via core.
	err := h.App.UpdateForeignTaxStatus(h.Context(), core.ActorWeb, core.ForeignTaxStatusInput{
		Year: 2025, Country: "BR", DocumentationType: "Receita Federal-utskrift",
		TaxFinalized: true, RF1147Ready: true, Notes: "Klar",
	})
	if err != nil {
		t.Fatal(err)
	}
	credits, _ := h.App.ForeignTaxForYear(h.Context(), 2025)
	if len(credits) != 1 {
		t.Fatalf("forventet 1 kreditfradrag, fikk %d", len(credits))
	}
	c := credits[0].Credit
	if c.DocumentationType.String != "Receita Federal-utskrift" {
		t.Errorf("documentation_type = %q", c.DocumentationType.String)
	}
	if c.Rf1147Ready.Int64 != 1 {
		t.Errorf("rf1147_ready = %d, forventet 1", c.Rf1147Ready.Int64)
	}
	// Aggregerte tall skal være bevart (10000*2.00 = 20000).
	if c.IncomeNok != 20000 {
		t.Errorf("income_nok = %v, forventet 20000 (bevart)", c.IncomeNok)
	}
}
