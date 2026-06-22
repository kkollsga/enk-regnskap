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
	v.Data = data
	s.renderer.Render(w, "tax_info", v)
}
