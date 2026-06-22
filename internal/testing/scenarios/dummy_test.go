package scenarios

import (
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/core"
	apptest "github.com/kkollsga/enk-regnskap/internal/testing"
)

// "Generer testdata"-knappen.

func TestGenerateDummyDataViaButton(t *testing.T) {
	h := apptest.Start(t)
	h.App.SetConfig(h.Context(), core.ConfigActiveYear, "2025")

	// Knappen i toppmenyen poster hit.
	res := h.Browser().PostForm("/dev/dummy-data", nil)
	apptest.AssertStatus(t, res, 200) // foelger redirect til /

	income, _ := h.App.ListIncome(h.Context(), 2025)
	if len(income) != 12 {
		t.Errorf("forventet 12 testinntekter, fikk %d", len(income))
	}
	expenses, _ := h.App.ListExpenses(h.Context(), 2025)
	if len(expenses) != 8 {
		t.Errorf("forventet 8 testutgifter, fikk %d", len(expenses))
	}

	// BRL-inntekt skal vaere konvertert til NOK uten nett (kurs fra cache).
	var brl int
	for _, in := range income {
		if in.Currency == "BRL" {
			brl++
			if in.AmountNok <= 0 {
				t.Errorf("BRL-inntekt %q mangler NOK-belop", in.Description)
			}
		}
	}
	if brl != 3 {
		t.Errorf("forventet 3 BRL-inntekter, fikk %d", brl)
	}

	// Utenlandsk skatt skal vaere aggregert (treaty for 2025).
	credits, _ := h.App.ForeignTaxForYear(h.Context(), 2025)
	if len(credits) != 1 || credits[0].Credit.LegalBasis.String != "treaty" {
		t.Errorf("forventet 1 BR-kreditfradrag (treaty), fikk %+v", credits)
	}
}

func TestDummyDataButtonInTopMenu(t *testing.T) {
	h := apptest.Start(t)
	doc := h.Browser().Get("/")
	// Toppmenyen skal inneholde testdata-knappen (form mot /dev/dummy-data).
	apptest.AssertHas(t, doc, "form[action=/dev/dummy-data]")
	apptest.AssertBodyContains(t, doc, "Testdata")
}

func TestDummyDataGeneratesValidCalculations(t *testing.T) {
	h := apptest.Start(t)
	h.App.SetConfig(h.Context(), core.ConfigActiveYear, "2025")
	if _, err := h.App.GenerateDummyData(h.Context(), core.ActorWeb); err != nil {
		t.Fatal(err)
	}
	rep, _ := h.App.BuildReport(h.Context(), 2025)
	// NOK-inntekt: 10000+15000+20000+12000+8000+25000+5000+30000+11000 = 136000
	// BRL @ 1,85: (10000+5000+7500)*1.85 = 41625
	// Sum = 177625
	if rep.TotalIncome != 177625 {
		t.Errorf("TotalIncome = %.2f, forventet 177625", rep.TotalIncome)
	}
}
