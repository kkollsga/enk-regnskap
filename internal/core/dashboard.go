package core

import (
	"context"
	"fmt"

	"github.com/kkollsga/enk-regnskap/internal/tax"
)

// Dashboard er nøkkeltallene på forsiden for et inntektsår.
type Dashboard struct {
	Year             int              `json:"year"`
	IncomeYTD        float64          `json:"income_ytd"`
	DeductibleYTD    float64          `json:"deductible_ytd"`
	Result           float64          `json:"result"` // inntekt - fradragsberettigede utgifter
	EstimatedTax     *tax.TaxEstimate `json:"estimated_tax"`
	ForeignTaxCredit float64          `json:"foreign_tax_credit"` // estimert kreditfradrag (§ 16-20 flg.)
	NetTax           float64          `json:"net_tax"`            // estimert skatt etter kreditfradrag
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

	incomeF := toFloat(income)
	deductibleF := toFloat(deductible)
	// Utenlandsk skatt behandlet som fradragsberettiget kostnad inngår i fradraget.
	if ftt, err := a.ForeignTaxTotalsForYear(ctx, year); err == nil {
		deductibleF += ftt.Deduct
	}
	deductibleF = tax.Round2(deductibleF)
	result := tax.Round2(incomeF - deductibleF)

	d := Dashboard{
		Year:          year,
		IncomeYTD:     incomeF,
		DeductibleYTD: deductibleF,
		Result:        result,
	}
	// Skatteestimat hvis vi har regler for året og positivt resultat.
	if rules, err := tax.Load(year); err == nil {
		est := rules.Estimate(result, result)
		d.EstimatedTax = &est
	}
	// Estimert kreditfradrag trekkes fra estimert skatt -> netto.
	if credit, err := a.EstimatedForeignTaxCredit(ctx, year); err == nil {
		d.ForeignTaxCredit = credit
	}
	if d.EstimatedTax != nil {
		net := d.EstimatedTax.SumSkatt - d.ForeignTaxCredit
		if net < 0 {
			net = 0
		}
		d.NetTax = tax.Round2(net)
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
