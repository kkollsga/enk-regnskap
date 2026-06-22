package core

import (
	"context"

	"github.com/kkollsga/enk-regnskap/internal/db"
	"github.com/kkollsga/enk-regnskap/internal/tax"
)

// CategorySum er en kategori med sumtotaler.
type CategorySum struct {
	Category   string
	Total      float64
	Deductible float64
}

// Report er all data som trengs for årsrapport, næringsspesifikasjon og
// transaksjonslogg for ett inntektsår.
type Report struct {
	Year              int
	BusinessName      string
	OrgNr             string
	Income            []db.Income
	Expenses          []db.Expense
	IncomeByCategory  []CategorySum
	ExpenseByCategory []CategorySum
	ForeignCredits    []db.ForeignTaxCredit
	TotalIncome       float64
	TotalExpenses     float64
	TotalDeductible   float64
	Result            float64 // TotalIncome - TotalDeductible
	Tax               *tax.TaxEstimate
}

// BuildReport samler alle tall for et inntektsår.
func (a *App) BuildReport(ctx context.Context, year int) (Report, error) {
	rep := Report{Year: year}
	rep.BusinessName = a.GetConfig(ctx, ConfigBusinessName, "")
	rep.OrgNr = a.GetConfig(ctx, ConfigOrgNr, "")

	income, err := a.Q.ListIncomeByYear(ctx, int64(year))
	if err != nil {
		return rep, err
	}
	rep.Income = income

	expenses, err := a.Q.ListExpensesByYear(ctx, int64(year))
	if err != nil {
		return rep, err
	}
	rep.Expenses = expenses

	incCat, err := a.Q.SumIncomeByCategory(ctx, int64(year))
	if err != nil {
		return rep, err
	}
	for _, c := range incCat {
		t := toFloat(c.Total)
		rep.IncomeByCategory = append(rep.IncomeByCategory, CategorySum{Category: c.Category, Total: t})
		rep.TotalIncome += t
	}

	expCat, err := a.Q.SumExpensesByCategory(ctx, int64(year))
	if err != nil {
		return rep, err
	}
	for _, c := range expCat {
		total := toFloat(c.Total)
		ded := toFloat(c.Deductible)
		rep.ExpenseByCategory = append(rep.ExpenseByCategory, CategorySum{Category: c.Category, Total: total, Deductible: ded})
		rep.TotalExpenses += total
		rep.TotalDeductible += ded
	}

	rep.TotalIncome = tax.Round2(rep.TotalIncome)
	rep.TotalExpenses = tax.Round2(rep.TotalExpenses)
	rep.TotalDeductible = tax.Round2(rep.TotalDeductible)
	rep.Result = tax.Round2(rep.TotalIncome - rep.TotalDeductible)

	credits, err := a.Q.ListForeignTaxCreditsByYear(ctx, int64(year))
	if err != nil {
		return rep, err
	}
	rep.ForeignCredits = credits

	if rules, err := tax.Load(year); err == nil {
		est := rules.Estimate(rep.Result, rep.Result)
		rep.Tax = &est
	}
	return rep, nil
}

// CategoryDisplayName gir et lesbart navn for en kategorinøkkel for et år.
func (a *App) CategoryDisplayName(year int, key string) string {
	for _, c := range a.ExpenseCategories(year) {
		if c.Key == key {
			return c.Name
		}
	}
	for _, c := range IncomeCategories() {
		if c.Key == key {
			return c.Name
		}
	}
	return key
}
