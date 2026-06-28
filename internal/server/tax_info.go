package server

import (
	"net/http"

	"github.com/kkollsga/enk-regnskap/internal/core"
	"github.com/kkollsga/enk-regnskap/internal/tax"
)

type taxInfoData struct {
	Rules           tax.Rules
	HasRules        bool
	Deductions      []core.DeductionUsage
	TotalDeductible float64
	Dash            core.Dashboard // foreløpig skatteberegning

	// Trinnvis skatteberegning (utvidbare linjer).
	IncomeLines       []calcLine        // brutto driftsinntekter per kategori
	IncomeTotal       float64           // = sum av IncomeLines (overskrift = sum av linjene)
	DeductibleTotal   float64           // = vanlige fradrag + ForeignDeductible
	ForeignDeductible float64           // utenlandsk skatt ført som fradragsberettiget kostnad
	ForeignLines      []foreignCalcLine // kreditfradrag per land
}

// calcLine er én linje i en utvidbar del av skatteberegningen.
type calcLine struct {
	Label string
	NOK   float64
}

// foreignCalcLine er kreditfradrag for ett land: betalt, estimert tak og anvendt.
type foreignCalcLine struct {
	Name    string
	Paid    float64
	MaxEst  float64
	Applied float64
}

func (s *Server) handleTaxInfo(w http.ResponseWriter, r *http.Request) {
	v := s.view(r, "tax-info", s.tr(r, "nav_tax_info"))
	data := taxInfoData{}
	if rules, err := s.app().TaxRulesFor(v.Year); err == nil {
		data.Rules = rules
		data.HasRules = true
	}
	deductions, total, err := s.app().DeductionUsageForYear(r.Context(), v.Year)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data.Deductions = deductions
	data.TotalDeductible = total
	data.Dash, _ = s.app().Dashboard(r.Context(), v.Year)

	// Brutto driftsinntekter per kategori (utvidbar linje).
	if incCat, err := s.app().IncomeByCategoryForYear(r.Context(), v.Year); err == nil {
		for _, c := range incCat {
			data.IncomeLines = append(data.IncomeLines, calcLine{
				Label: s.app().CategoryDisplayName(v.Year, c.Category),
				NOK:   c.Total,
			})
			data.IncomeTotal += c.Total
		}
		data.IncomeTotal = tax.Round2(data.IncomeTotal)
	}
	// Utenlandsk skatt: fradragsberettiget kostnad (inngår i fradraget) +
	// kreditfradrag per land (trekkes fra beregnet skatt).
	if ftt, err := s.app().ForeignTaxTotalsForYear(r.Context(), v.Year); err == nil {
		data.ForeignDeductible = ftt.Deduct
	}
	// Overskriftene = nøyaktig sum av linjene som vises (unngår øre-avvik).
	data.DeductibleTotal = tax.Round2(data.TotalDeductible + data.ForeignDeductible)
	if ovs, err := s.app().ForeignTaxForYear(r.Context(), v.Year); err == nil {
		for _, ov := range ovs {
			if ov.Credit.ForeignTaxNok == 0 {
				continue
			}
			applied := ov.Credit.ForeignTaxNok
			if applied > ov.MaxCreditEst {
				applied = ov.MaxCreditEst
			}
			data.ForeignLines = append(data.ForeignLines, foreignCalcLine{
				Name:    ov.Credit.CountryName,
				Paid:    ov.Credit.ForeignTaxNok,
				MaxEst:  ov.MaxCreditEst,
				Applied: applied,
			})
		}
	}

	v.Data = data
	s.renderer.Render(w, "tax_info", v)
}
