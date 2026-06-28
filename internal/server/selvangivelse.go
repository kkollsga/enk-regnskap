package server

import (
	"net/http"

	"github.com/kkollsga/enk-regnskap/internal/core"
)

// selvangivelseData er en veiledet visning av skattemeldingen/næringsspesifikasjonen
// for et inntektsår, organisert etter RF-skjemaene med tall fra appens data.
type selvangivelseData struct {
	Report           core.Report
	CatNames         map[string]string
	Foreign          []core.ForeignTaxOverview
	ForeignCreditEst float64
	NetTax           float64
	HasForeign       bool
}

func (s *Server) handleSelvangivelse(w http.ResponseWriter, r *http.Request) {
	v := s.view(r, "selvangivelse", s.tr(r, "nav_selvangivelse"))
	rep, err := s.app().BuildReport(r.Context(), v.Year)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	catNames := map[string]string{}
	for _, c := range rep.IncomeByCategory {
		catNames[c.Category] = s.app().CategoryDisplayName(v.Year, c.Category)
	}
	for _, c := range rep.ExpenseByCategory {
		catNames[c.Category] = s.app().CategoryDisplayName(v.Year, c.Category)
	}
	foreign, _ := s.app().ForeignTaxForYear(r.Context(), v.Year)
	creditEst, _ := s.app().EstimatedForeignTaxCredit(r.Context(), v.Year)
	netTax := 0.0
	if rep.Tax != nil {
		netTax = rep.Tax.SumSkatt - creditEst
		if netTax < 0 {
			netTax = 0
		}
	}
	v.Data = selvangivelseData{
		Report:           rep,
		CatNames:         catNames,
		Foreign:          foreign,
		ForeignCreditEst: creditEst,
		NetTax:           netTax,
		HasForeign:       len(foreign) > 0,
	}
	s.renderer.Render(w, "selvangivelse", v)
}
