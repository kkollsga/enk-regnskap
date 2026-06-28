package core

import "context"

// ForeignTaxExpenseCategories er utgiftskategorier for brasiliansk skatt etter
// skatteavtalen Norge–Brasil. IRRF (kildeskatt) er kreditberettiget (gir
// kreditfradrag mot norsk skatt); de øvrige er normalt ikke krediterbare og
// regnes som overskytende skatt betalt til Brasil.
//
// Disse kategoriene har 0 % fradrag i næringsresultatet: utenlandsk skatt gir
// KREDITFRADRAG (mot norsk skatt), ikke kostnadsfradrag i inntekten.
func ForeignTaxExpenseCategories() []ExpenseCategory {
	g := GroupSkattBrasil
	cats := []ExpenseCategory{
		{
			Key: "skatt_irrf", Group: g, Kind: TaxKindCreditable, DefaultPct: 0,
			Name:        "IRRF – brasiliansk kildeskatt (kreditberettiget)",
			Description: "Imposto de Renda Retido na Fonte. Brasiliansk inntektsskatt som gir kreditfradrag mot norsk skatt (sktl. § 16-20 / skatteavtalen art. 25).",
			Note:        "Krediteres mot norsk skatt – ikke kostnadsfradrag i resultatet. Krever dokumentasjon.",
		},
		{
			Key: "skatt_iss", Group: g, Kind: TaxKindNonCreditable, DefaultPct: 0,
			Name:        "ISS – kommunal tjenesteskatt (normalt ikke krediterbar)",
			Description: "Imposto Sobre Serviços. Indirekte kommunal skatt; gir normalt ikke kreditfradrag i Norge.",
			Note:        "Overskytende skatt til Brasil – normalt ikke krediterbar.",
		},
		{
			Key: "skatt_csll", Group: g, Kind: TaxKindNonCreditable, DefaultPct: 0,
			Name:        "CSLL – sosial bidragsskatt (ikke krediterbar)",
			Description: "Contribuição Social sobre o Lucro Líquido. Normalt ikke krediterbar for et norsk ENK.",
			Note:        "Overskytende skatt til Brasil.",
		},
		{
			Key: "skatt_pis", Group: g, Kind: TaxKindNonCreditable, DefaultPct: 0,
			Name:        "PIS – bidragsskatt på omsetning (ikke krediterbar)",
			Description: "Programa de Integração Social. Omsetningsbasert; ikke en inntektsskatt og normalt ikke krediterbar.",
			Note:        "Overskytende skatt til Brasil.",
		},
		{
			Key: "skatt_cofins", Group: g, Kind: TaxKindNonCreditable, DefaultPct: 0,
			Name:        "COFINS – bidragsskatt (ikke krediterbar)",
			Description: "Contribuição para o Financiamento da Seguridade Social. Omsetningsbasert; normalt ikke krediterbar.",
			Note:        "Overskytende skatt til Brasil.",
		},
		{
			Key: "skatt_overskytende", Group: g, Kind: TaxKindNonCreditable, DefaultPct: 0,
			Name:        "Overskytende brasiliansk skatt (over kredittak)",
			Description: "Brasiliansk skatt utover maksimalt kreditfradrag (sktl. § 16-21). Kan fremføres i inntil 5 år (§ 16-22).",
			Note:        "Ikke benyttet kredit – vurder fremføring.",
		},
	}
	for i := range cats {
		cats[i].Country = "BR" // brasilianske skattekategorier vises kun for Brasil
	}
	return cats
}

// TaxExpenseSummary oppsummerer utenlandsk skatt registrert som utgifter.
type TaxExpenseSummary struct {
	Creditable    float64 // kreditberettiget brasiliansk skatt (NOK)
	Overpaid      float64 // overskytende / ikke krediterbar (NOK)
	HasTaxExpense bool
}

// TaxExpenseSummaryForYear summerer skatteutgifter per kind for et inntektsår.
func (a *App) TaxExpenseSummaryForYear(ctx context.Context, year int) (TaxExpenseSummary, error) {
	expenses, err := a.ListExpenses(ctx, year)
	if err != nil {
		return TaxExpenseSummary{}, err
	}
	kind := map[string]string{}
	for _, c := range a.ExpenseCategories(year) {
		kind[c.Key] = c.Kind
	}
	var sum TaxExpenseSummary
	for _, e := range expenses {
		switch kind[e.Category] {
		case TaxKindCreditable:
			sum.Creditable += e.AmountNok
			sum.HasTaxExpense = true
		case TaxKindNonCreditable:
			sum.Overpaid += e.AmountNok
			sum.HasTaxExpense = true
		}
	}
	sum.Creditable = roundMoney(sum.Creditable)
	sum.Overpaid = roundMoney(sum.Overpaid)
	return sum, nil
}

func roundMoney(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
