package scenarios

import (
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/core"
	apptest "github.com/kkollsga/enk-regnskap/internal/testing"
)

// Steg 4: utenlandsk skatt.

// TestNonCreditableTaxExcludedFromCredit verifiserer at en ikke-krediterbar
// skattetype (f.eks. COFINS) lagres for referanse, men holdes utenfor
// kreditfradraget, mens en krediterbar type (IRRF) teller med.
func TestNonCreditableTaxExcludedFromCredit(t *testing.T) {
	h := apptest.Start(t)
	ctx := h.Context()
	h.App.SetConfig(ctx, core.ConfigActiveYear, "2025")
	h.Mock.AddRate("BRL", "2025-03-10", 2.00)

	res, err := h.App.AddIncome(ctx, core.ActorWeb, core.IncomeInput{
		Date: "2025-03-10", Description: "Brasil med to skattetyper", Currency: "BRL",
		CountryCode: "BR", AmountOrig: 10000, Category: "tjenesteinntekt", TaxYear: 2025,
		ForeignTaxPaid: core.ForeignTaxYes,
		ForeignTaxes: []core.ForeignTaxLine{
			{Type: "IRRF", AmountOrig: 1500, Currency: "BRL"},  // krediterbar
			{Type: "COFINS", AmountOrig: 500, Currency: "BRL"}, // ikke krediterbar
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	lines, _ := h.App.IncomeForeignTaxes(ctx, res.Income.ID)
	if len(lines) != 2 {
		t.Fatalf("forventet 2 skattelinjer, fikk %d", len(lines))
	}
	byType := map[string]string{}
	for _, l := range lines {
		byType[l.TaxType] = l.Treatment
	}
	// IRRF er krediterbar -> credit. COFINS er ikke krediterbar -> standard
	// behandling er fradragsberettiget kostnad (deduct).
	if byType["IRRF"] != core.TaxTreatmentCredit {
		t.Errorf("IRRF treatment = %q, forventet credit", byType["IRRF"])
	}
	if byType["COFINS"] != core.TaxTreatmentDeduct {
		t.Errorf("COFINS treatment = %q, forventet deduct", byType["COFINS"])
	}

	// Kreditfradraget skal kun inkludere IRRF: 1500 BRL * 2.00 = 3000 NOK.
	credit, err := h.App.ForeignTaxForYear(ctx, 2025)
	if err != nil {
		t.Fatal(err)
	}
	if len(credit) != 1 {
		t.Fatalf("forventet 1 kreditfradrag, fikk %d", len(credit))
	}
	if credit[0].Credit.ForeignTaxNok != 3000 {
		t.Errorf("kreditert utenlandsk skatt = %v, forventet 3000 (COFINS skal være ekskludert)",
			credit[0].Credit.ForeignTaxNok)
	}

	// COFINS (500 BRL * 2.00 = 1000 NOK) skal telle som fradragsberettiget kostnad.
	totals, err := h.App.ForeignTaxTotalsForYear(ctx, 2025)
	if err != nil {
		t.Fatal(err)
	}
	if totals.Credit != 3000 {
		t.Errorf("credit-total = %v, forventet 3000", totals.Credit)
	}
	if totals.Deduct != 1000 {
		t.Errorf("deduct-total = %v, forventet 1000 (COFINS som kostnad)", totals.Deduct)
	}
}

// TestDeductibleForeignTaxLinkedToExpenses verifiserer at en fradragsberettiget
// skattelinje (deduct) dukker opp som koblet utgiftspost med peker til inntekten,
// mens en kreditert linje (credit) ikke gjør det.
func TestDeductibleForeignTaxLinkedToExpenses(t *testing.T) {
	h := apptest.Start(t)
	ctx := h.Context()
	h.Mock.AddRate("BRL", "2025-03-10", 2.00)

	res, err := h.App.AddIncome(ctx, core.ActorWeb, core.IncomeInput{
		Date: "2025-03-10", Description: "Brasil", Currency: "BRL", CountryCode: "BR",
		AmountOrig: 10000, Category: "tjenesteinntekt", TaxYear: 2025,
		ForeignTaxPaid: core.ForeignTaxYes,
		ForeignTaxes: []core.ForeignTaxLine{
			{Type: "IRRF", AmountOrig: 1500, Currency: "BRL"},  // credit
			{Type: "COFINS", AmountOrig: 500, Currency: "BRL"}, // deduct
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	linked, err := h.App.DeductibleForeignTaxLines(ctx, 2025)
	if err != nil {
		t.Fatal(err)
	}
	if len(linked) != 1 {
		t.Fatalf("forventet 1 koblet fradragslinje, fikk %d", len(linked))
	}
	if linked[0].TaxType != "COFINS" {
		t.Errorf("koblet linje = %q, forventet COFINS (kun deduct)", linked[0].TaxType)
	}
	if linked[0].IncomeID != res.Income.ID {
		t.Errorf("koblet til inntekt %d, forventet %d", linked[0].IncomeID, res.Income.ID)
	}
	if linked[0].AmountNok != 1000 {
		t.Errorf("koblet beløp = %v, forventet 1000", linked[0].AmountNok)
	}
}

// TestCSLLCreditableFrom2025 låser fast at CSLL (bidragsskatt på netto overskudd)
// behandles som krediterbar inntektsskatt fra inntektsår 2025 (skatteavtalen
// Norge-Brasil art. 2), men ikke for tidligere år.
func TestCSLLCreditableFrom2025(t *testing.T) {
	h := apptest.Start(t)
	ctx := h.Context()
	h.Mock.AddRate("BRL", "2025-06-01", 2.00)
	h.Mock.AddRate("BRL", "2024-06-01", 2.00)

	res25, err := h.App.AddIncome(ctx, core.ActorWeb, core.IncomeInput{
		Date: "2025-06-01", Description: "BR CSLL 2025", Currency: "BRL", CountryCode: "BR",
		AmountOrig: 10000, Category: "tjenesteinntekt", TaxYear: 2025,
		ForeignTaxPaid: core.ForeignTaxYes,
		ForeignTaxes:   []core.ForeignTaxLine{{Type: "CSLL", AmountOrig: 900, Currency: "BRL"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	l25, _ := h.App.IncomeForeignTaxes(ctx, res25.Income.ID)
	if l25[0].Treatment != core.TaxTreatmentCredit {
		t.Errorf("CSLL 2025 treatment = %q, forventet credit (skatteavtale art. 2)", l25[0].Treatment)
	}

	res24, err := h.App.AddIncome(ctx, core.ActorWeb, core.IncomeInput{
		Date: "2024-06-01", Description: "BR CSLL 2024", Currency: "BRL", CountryCode: "BR",
		AmountOrig: 10000, Category: "tjenesteinntekt", TaxYear: 2024,
		ForeignTaxPaid: core.ForeignTaxYes,
		ForeignTaxes:   []core.ForeignTaxLine{{Type: "CSLL", AmountOrig: 900, Currency: "BRL"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	l24, _ := h.App.IncomeForeignTaxes(ctx, res24.Income.ID)
	if l24[0].Treatment != core.TaxTreatmentDeduct {
		t.Errorf("CSLL 2024 treatment = %q, forventet deduct (ikke krediterbar før 2025)", l24[0].Treatment)
	}
}

func seedBrazilIncome(t *testing.T, h *apptest.Harness, year int, date string, amount, tax float64) {
	t.Helper()
	h.Mock.AddRate("BRL", date, 2.00)
	_, err := h.App.AddIncome(h.Context(), core.ActorWeb, core.IncomeInput{
		Date: date, Description: "Brasiliansk inntekt", Currency: "BRL",
		CountryCode: "BR", AmountOrig: amount, Category: "tjenesteinntekt",
		TaxYear: year, ForeignTaxPaid: core.ForeignTaxYes,
		ForeignTaxes: []core.ForeignTaxLine{{Type: "IRRF", AmountOrig: tax, Currency: "BRL"}},
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
	// Ikrafttredelsesdato for skatteavtalen vises i lesbart format her (ikke på Skatteinfo).
	apptest.AssertBodyContains(t, doc, "30. desember 2024")
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

// TestINSSDefaultsToNone låser fast at INSS (trygdeavgift) får standard­behandling
// 'none' (verken kreditfradrag eller kostnadsfradrag), mens IRPF er krediterbar.
func TestINSSDefaultsToNone(t *testing.T) {
	h := apptest.Start(t)
	ctx := h.Context()
	h.App.SetConfig(ctx, core.ConfigActiveYear, "2025")
	h.Mock.AddRate("BRL", "2025-03-10", 2.00)
	res, err := h.App.AddIncome(ctx, core.ActorWeb, core.IncomeInput{
		Date: "2025-03-10", Description: "BR", Currency: "BRL", CountryCode: "BR",
		AmountOrig: 10000, Category: "tjenesteinntekt", TaxYear: 2025,
		ForeignTaxPaid: core.ForeignTaxYes,
		ForeignTaxes: []core.ForeignTaxLine{
			{Type: "INSS", AmountOrig: 300, Currency: "BRL"},
			{Type: "IRPF", AmountOrig: 800, Currency: "BRL"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	lines, _ := h.App.IncomeForeignTaxes(ctx, res.Income.ID)
	byType := map[string]string{}
	for _, l := range lines {
		byType[l.TaxType] = l.Treatment
	}
	if byType["INSS"] != core.TaxTreatmentNone {
		t.Errorf("INSS treatment = %q, forventet none", byType["INSS"])
	}
	if byType["IRPF"] != core.TaxTreatmentCredit {
		t.Errorf("IRPF treatment = %q, forventet credit", byType["IRPF"])
	}
	linked, _ := h.App.DeductibleForeignTaxLines(ctx, 2025)
	for _, l := range linked {
		if l.TaxType == "INSS" {
			t.Errorf("INSS skal ikke være en fradragspost (treatment none)")
		}
	}
}
