package core

import (
	"context"
	"fmt"

	"github.com/kkollsga/enk-regnskap/internal/tax"
)

// Dashboard er nøkkeltallene på forsiden for et inntektsår.
type Dashboard struct {
	Year             int
	IncomeYTD        float64
	DeductibleYTD    float64
	Result           float64 // inntekt - fradragsberettigede utgifter
	UnlinkedReceipts int64
	EstimatedTax     *tax.TaxEstimate
}

// Dashboard beregner nøkkeltall for et gitt inntektsår.
func (a *App) Dashboard(ctx context.Context, year int) (Dashboard, error) {
	income, err := a.Q.SumIncomeNOKByYear(ctx, int64(year))
	if err != nil {
		return Dashboard{}, fmt.Errorf("sum inntekt: %w", err)
	}
	deductible, err := a.Q.SumDeductibleByYear(ctx, int64(year))
	if err != nil {
		return Dashboard{}, fmt.Errorf("sum fradrag: %w", err)
	}
	unlinked, err := a.Q.CountUnlinkedReceipts(ctx)
	if err != nil {
		return Dashboard{}, fmt.Errorf("tell kvitteringer: %w", err)
	}

	incomeF := toFloat(income)
	deductibleF := toFloat(deductible)
	result := tax.Round2(incomeF - deductibleF)

	d := Dashboard{
		Year:             year,
		IncomeYTD:        incomeF,
		DeductibleYTD:    deductibleF,
		Result:           result,
		UnlinkedReceipts: unlinked,
	}
	// Skatteestimat hvis vi har regler for året og positivt resultat.
	if rules, err := tax.Load(year); err == nil {
		est := rules.Estimate(result, result)
		d.EstimatedTax = &est
	}
	return d, nil
}

// toFloat tolker sqlc sin interface{}-aggregatverdi (SUM/COALESCE) som float64.
func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case nil:
		return 0
	default:
		return 0
	}
}
