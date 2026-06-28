package mcp

import (
	"context"

	"github.com/kkollsga/enk-regnskap/internal/core"
)

// selvOut speiler Selvangivelse-hjelpesiden: tallene en ENK fører hvor i
// skattemeldingen, organisert etter RF-skjema. Kompakt, ingen rader.
type selvOut struct {
	Year         int    `json:"year"`
	BusinessName string `json:"business_name,omitempty"`
	OrgNr        string `json:"org_nr,omitempty"`

	// RF-1175 Næringsspesifikasjon
	SumDriftsinntekter float64            `json:"sum_driftsinntekter"`
	SumFradrag         float64            `json:"sum_fradrag"`
	Naeringsresultat   float64            `json:"naeringsresultat"`
	IncomeByCategory   []core.CategorySum `json:"income_by_category"`
	ExpenseByCategory  []core.CategorySum `json:"expense_by_category"`

	// RF-1224 Personinntekt
	Personinntekt   float64 `json:"personinntekt"`
	TrygdeavgiftPct float64 `json:"trygdeavgift_pct"`
	Trygdeavgift    float64 `json:"trygdeavgift"`
	Trinnskatt      float64 `json:"trinnskatt"`

	// RF-1147 Kreditfradrag
	Foreign          []foreignOverviewOut `json:"foreign_credits,omitempty"`
	ForeignCreditEst float64              `json:"foreign_credit_est,omitempty"`

	// Estimert skatt
	AlminneligInntektsskatt float64 `json:"alminnelig_inntektsskatt"`
	SumSkatt                float64 `json:"sum_skatt"`
	NetTax                  float64 `json:"net_tax_etter_kredit"`
}

func selvangivelseRun(ctx context.Context, app *core.App, year int) (string, error) {
	rep, err := app.BuildReport(ctx, year)
	if err != nil {
		return "", err
	}
	foreign, _ := app.ForeignTaxForYear(ctx, year)
	creditEst, _ := app.EstimatedForeignTaxCredit(ctx, year)
	trygdePct := 0.0
	if rules, err := app.TaxRulesFor(year); err == nil {
		trygdePct = rules.TrygdeavgiftNaeringPct
	}
	out := selvOut{
		Year: year, BusinessName: rep.BusinessName, OrgNr: rep.OrgNr,
		SumDriftsinntekter: rep.TotalIncome, SumFradrag: rep.TotalDeductible,
		Naeringsresultat:  rep.Result,
		IncomeByCategory:  rep.IncomeByCategory,
		ExpenseByCategory: rep.ExpenseByCategory,
		Foreign:           foreignOverviewDTOs(foreign),
		ForeignCreditEst:  creditEst,
		TrygdeavgiftPct:   trygdePct,
	}
	if rep.Tax != nil {
		out.Personinntekt = rep.Tax.Personinntekt
		out.Trygdeavgift = rep.Tax.Trygdeavgift
		out.Trinnskatt = rep.Tax.Trinnskatt
		out.AlminneligInntektsskatt = rep.Tax.AlminneligInntektsskatt
		out.SumSkatt = rep.Tax.SumSkatt
		out.NetTax = rep.Tax.SumSkatt - creditEst
		if out.NetTax < 0 {
			out.NetTax = 0
		}
	}
	return toJSON(out), nil
}
