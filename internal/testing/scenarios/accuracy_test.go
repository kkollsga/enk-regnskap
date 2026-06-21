package scenarios

import (
	"math"
	"testing"

	apptest "github.com/kkollsga/enk-regnskap/internal/testing"
)

// Ende-til-ende noyaktighetstest mot det faste testdatasettet (fixtures).
// Alle forventede verdier er regnet ut for hand uavhengig av appen.

func approx(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func TestFixtureTotalsAreAccurate(t *testing.T) {
	h := apptest.Start(t)
	h.LoadFixtures(t)

	rep, err := h.App.BuildReport(h.Context(), apptest.FixtureYear)
	if err != nil {
		t.Fatal(err)
	}

	// Inntekt: 136 000 NOK (9 NOK) + 45 000 NOK (3 BRL @ 2,00) = 181 000.
	if !approx(rep.TotalIncome, apptest.FixtureIncome) {
		t.Errorf("TotalIncome = %.2f, forventet %.2f", rep.TotalIncome, apptest.FixtureIncome)
	}
	// Fradragsberettiget: 35 892 (telefon 6000 -> 3000 ved 50%).
	if !approx(rep.TotalDeductible, apptest.FixtureDeductible) {
		t.Errorf("TotalDeductible = %.2f, forventet %.2f", rep.TotalDeductible, apptest.FixtureDeductible)
	}
	// Resultat: 181 000 - 35 892 = 145 108.
	if !approx(rep.Result, apptest.FixtureResult) {
		t.Errorf("Result = %.2f, forventet %.2f", rep.Result, apptest.FixtureResult)
	}
}

func TestFixtureTaxEstimateIsAccurate(t *testing.T) {
	h := apptest.Start(t)
	h.LoadFixtures(t)
	rep, _ := h.App.BuildReport(h.Context(), apptest.FixtureYear)
	if rep.Tax == nil {
		t.Fatal("mangler skatteestimat")
	}
	// Personinntekt = resultat = 145 108 (2025-satser):
	//   alminnelig 22%        = 31 923,76
	//   trygdeavgift (10,9% / opptrapping 25% over 99 650):
	//     full = 145108*0.109 = 15 816,772
	//     opptrapping = (145108-99650)*0.25 = 11 364,50  -> min = 11 364,50
	//   trinnskatt (under 217 400) = 0
	//   sum = 43 288,26
	if !approx(rep.Tax.AlminneligInntektsskatt, 31923.76) {
		t.Errorf("alminnelig = %.2f, forventet 31923.76", rep.Tax.AlminneligInntektsskatt)
	}
	if !approx(rep.Tax.Trygdeavgift, 11364.50) {
		t.Errorf("trygdeavgift = %.2f, forventet 11364.50", rep.Tax.Trygdeavgift)
	}
	if !approx(rep.Tax.Trinnskatt, 0) {
		t.Errorf("trinnskatt = %.2f, forventet 0", rep.Tax.Trinnskatt)
	}
	if !approx(rep.Tax.SumSkatt, 43288.26) {
		t.Errorf("sum skatt = %.2f, forventet 43288.26", rep.Tax.SumSkatt)
	}
}

func TestFixtureForeignTaxAggregation(t *testing.T) {
	h := apptest.Start(t)
	h.LoadFixtures(t)
	credits, err := h.App.ForeignTaxForYear(h.Context(), apptest.FixtureYear)
	if err != nil {
		t.Fatal(err)
	}
	if len(credits) != 1 {
		t.Fatalf("forventet 1 land (Brasil), fikk %d", len(credits))
	}
	c := credits[0].Credit
	// Utenlandsinntekt: (10000+5000+7500)*2,00 = 45 000 NOK.
	if !approx(c.IncomeNok, apptest.FixtureForeign) {
		t.Errorf("utenlandsinntekt = %.2f, forventet %.2f", c.IncomeNok, apptest.FixtureForeign)
	}
	// Betalt brasiliansk skatt: (1500+750)*2,00 = 4 500 NOK (den tredje uten skatt).
	if !approx(c.ForeignTaxNok, apptest.FixtureForeignTaxNOK) {
		t.Errorf("utenlandsk skatt = %.2f, forventet %.2f", c.ForeignTaxNok, apptest.FixtureForeignTaxNOK)
	}
	if c.LegalBasis.String != "treaty" {
		t.Errorf("rettsgrunnlag = %q, forventet treaty (2025)", c.LegalBasis.String)
	}
}

func TestFixtureDashboardMatchesReport(t *testing.T) {
	h := apptest.Start(t)
	h.LoadFixtures(t)
	d, err := h.App.Dashboard(h.Context(), apptest.FixtureYear)
	if err != nil {
		t.Fatal(err)
	}
	if !approx(d.IncomeYTD, apptest.FixtureIncome) {
		t.Errorf("dashboard inntekt = %.2f, forventet %.2f", d.IncomeYTD, apptest.FixtureIncome)
	}
	if !approx(d.DeductibleYTD, apptest.FixtureDeductible) {
		t.Errorf("dashboard fradrag = %.2f, forventet %.2f", d.DeductibleYTD, apptest.FixtureDeductible)
	}
	if !approx(d.Result, apptest.FixtureResult) {
		t.Errorf("dashboard resultat = %.2f, forventet %.2f", d.Result, apptest.FixtureResult)
	}
}
