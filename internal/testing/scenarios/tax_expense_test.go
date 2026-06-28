package scenarios

import (
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/core"
	apptest "github.com/kkollsga/enk-regnskap/internal/testing"
)

// Skatteutgifter (skatteavtale Norge–Brasil): kreditberettiget vs overskytende.

func TestForeignTaxExpenseCategoriesAvailable(t *testing.T) {
	h := apptest.Start(t)
	cats := h.App.ExpenseCategories(2025)
	byKey := map[string]core.ExpenseCategory{}
	for _, c := range cats {
		byKey[c.Key] = c
	}
	// IRRF skal være kreditberettiget, ISS/CSLL/PIS/COFINS ikke.
	if byKey["skatt_irrf"].Kind != core.TaxKindCreditable {
		t.Errorf("IRRF kind = %q, forventet creditable", byKey["skatt_irrf"].Kind)
	}
	for _, k := range []string{"skatt_iss", "skatt_csll", "skatt_pis", "skatt_cofins", "skatt_overskytende"} {
		if byKey[k].Kind != core.TaxKindNonCreditable {
			t.Errorf("%s kind = %q, forventet noncreditable", k, byKey[k].Kind)
		}
	}
	// Skattekategorier gir 0 % kostnadsfradrag.
	if byKey["skatt_irrf"].DefaultPct != 0 {
		t.Errorf("IRRF DefaultPct = %v, forventet 0", byKey["skatt_irrf"].DefaultPct)
	}
}

func TestTaxExpenseDoesNotReduceResult(t *testing.T) {
	h := apptest.Start(t)
	ctx := h.Context()
	h.App.SetConfig(ctx, core.ConfigActiveYear, "2025")
	h.App.AddIncome(ctx, core.ActorWeb, core.IncomeInput{
		Date: "2025-02-01", Description: "Inntekt", Currency: "NOK",
		CountryCode: "NO", AmountOrig: 100000, Category: "tjenesteinntekt",
	})
	// IRRF (kreditberettiget) bokført som utgift – 0 % fradrag.
	exp, err := h.App.AddExpense(ctx, core.ActorWeb, core.ExpenseInput{
		Date: "2025-03-01", Description: "IRRF betalt til Brasil", Category: "skatt_irrf",
		AmountOrig: 5000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if exp.DeductibleNok != 0 {
		t.Errorf("skatteutgift gir %v fradrag, forventet 0", exp.DeductibleNok)
	}
	rep, _ := h.App.BuildReport(ctx, 2025)
	// Resultatet skal ikke reduseres av skatteutgiften (fradrag 0).
	if rep.Result != 100000 {
		t.Errorf("Result = %v, forventet 100000 (skatt reduserer ikke resultat)", rep.Result)
	}
}

func TestTaxExpenseSummary(t *testing.T) {
	h := apptest.Start(t)
	ctx := h.Context()
	h.App.SetConfig(ctx, core.ConfigActiveYear, "2025")
	h.App.AddExpense(ctx, core.ActorWeb, core.ExpenseInput{
		Date: "2025-03-01", Description: "IRRF", Category: "skatt_irrf", AmountOrig: 4000,
	})
	h.App.AddExpense(ctx, core.ActorWeb, core.ExpenseInput{
		Date: "2025-03-02", Description: "ISS", Category: "skatt_iss", AmountOrig: 1500,
	})
	h.App.AddExpense(ctx, core.ActorWeb, core.ExpenseInput{
		Date: "2025-03-03", Description: "PC", Category: "smaa_driftsmidler", AmountOrig: 9000,
	})

	sum, err := h.App.TaxExpenseSummaryForYear(ctx, 2025)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Creditable != 4000 {
		t.Errorf("Creditable = %v, forventet 4000 (IRRF)", sum.Creditable)
	}
	if sum.Overpaid != 1500 {
		t.Errorf("Overpaid = %v, forventet 1500 (ISS)", sum.Overpaid)
	}
	if !sum.HasTaxExpense {
		t.Error("HasTaxExpense skal være true")
	}

	// Siden skal vise begge nøkkeltallene.
	doc := h.Browser().Get("/expenses")
	apptest.AssertBodyContains(t, doc, "Kreditberettiget")
	apptest.AssertBodyContains(t, doc, "Overskytende")
}
