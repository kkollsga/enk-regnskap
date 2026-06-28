package server

import (
	"net/http"

	"github.com/kkollsga/enk-regnskap/internal/core"
	"github.com/kkollsga/enk-regnskap/internal/db"
)

// foreignCountryView samler oversikt + sjekkliste for ett land.
type foreignCountryView struct {
	Overview   core.ForeignTaxOverview
	TaxTypes   []db.CountryTaxType
	Summary    core.CountryTaxSummary // skatteavtale + krediterbare skatter
	HasSummary bool
	Status     string // "Dokumentasjon mangler" | "Klar for RF-1147"
	NoTaxPaid  bool   // inntekt finnes, men ingen dokumentert skatt
	NotFinal   bool   // skatt ikke endelig fastsatt i utlandet
}

type foreignTaxData struct {
	Countries  []foreignCountryView
	HasData    bool
	Deductible float64 // utenlandsk skatt ført som fradragsberettiget kostnad
	Reference  float64 // utenlandsk skatt uten lettelse (kun referanse)
}

func (s *Server) handleForeignTax(w http.ResponseWriter, r *http.Request) {
	v := s.view(r, "foreign-tax", s.tr(r, "nav_foreign_tax"))
	year := v.Year

	overviews, err := s.app().ForeignTaxForYear(r.Context(), year)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	summaries, _ := s.app().ForeignCountrySummaries(r.Context())
	summaryByCode := map[string]core.CountryTaxSummary{}
	for _, sm := range summaries {
		summaryByCode[sm.Code] = sm
	}

	data := foreignTaxData{}
	for _, ov := range overviews {
		types, _ := s.app().CountryTaxTypes(r.Context(), ov.Credit.CountryCode, year)
		cv := foreignCountryView{
			Overview:  ov,
			TaxTypes:  types,
			NoTaxPaid: ov.Credit.ForeignTaxNok == 0 && ov.Credit.IncomeNok > 0,
			NotFinal:  !(ov.Credit.TaxFinalizedAbroad.Valid && ov.Credit.TaxFinalizedAbroad.Int64 == 1),
		}
		if sm, ok := summaryByCode[ov.Credit.CountryCode]; ok {
			cv.Summary = sm
			cv.HasSummary = true
		}
		if ov.Credit.Rf1147Ready.Valid && ov.Credit.Rf1147Ready.Int64 == 1 {
			cv.Status = "Klar for RF-1147"
		} else {
			cv.Status = "Dokumentasjon mangler"
		}
		data.Countries = append(data.Countries, cv)
	}
	data.HasData = len(data.Countries) > 0
	if totals, err := s.app().ForeignTaxTotalsForYear(r.Context(), year); err == nil {
		data.Deductible = totals.Deduct
		data.Reference = totals.None
	}
	v.Data = data
	s.renderer.Render(w, "foreign_tax", v)
}

func (s *Server) handleForeignTaxUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "ugyldig skjema", http.StatusBadRequest)
		return
	}
	in := core.ForeignTaxStatusInput{
		Year:              parseInt(r.FormValue("year")),
		Country:           r.FormValue("country"),
		DocumentationType: r.FormValue("documentation_type"),
		TaxFinalized:      r.FormValue("tax_finalized") != "",
		RF1147Ready:       r.FormValue("rf1147_ready") != "",
		Notes:             r.FormValue("notes"),
	}
	if err := s.app().UpdateForeignTaxStatus(r.Context(), core.ActorWeb, in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/foreign-tax?saved=1", http.StatusSeeOther)
}
