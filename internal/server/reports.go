package server

import (
	"fmt"
	"net/http"

	"github.com/kkollsga/enk-regnskap/internal/core"
	"github.com/kkollsga/enk-regnskap/internal/export"
	"github.com/kkollsga/enk-regnskap/internal/pdf"
)

func (s *Server) handleReports(w http.ResponseWriter, r *http.Request) {
	v := s.view(r, "reports", s.tr(r, "nav_reports"))
	rep, err := s.app.BuildReport(r.Context(), v.Year)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	v.Data = rep
	s.renderer.Render(w, "report", v)
}

// catNamer gir en funksjon som oversetter kategorinokler til navn for aaret.
func (s *Server) catNamer(year int) func(string) string {
	return func(key string) string { return s.app.CategoryDisplayName(year, key) }
}

func (s *Server) buildReportFor(r *http.Request) (core.Report, error) {
	year := s.app.ActiveYear(r.Context())
	return s.app.BuildReport(r.Context(), year)
}

func (s *Server) handleAnnualPDF(w http.ResponseWriter, r *http.Request) {
	rep, err := s.buildReportFor(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	setDownload(w, fmt.Sprintf("arsrapport-%d.pdf", rep.Year), "application/pdf")
	if err := pdf.AnnualReport(w, rep, s.catNamer(rep.Year)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleTaxSummaryPDF(w http.ResponseWriter, r *http.Request) {
	rep, err := s.buildReportFor(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	setDownload(w, fmt.Sprintf("naeringsspesifikasjon-%d.pdf", rep.Year), "application/pdf")
	if err := pdf.TaxSummary(w, rep, s.catNamer(rep.Year)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleTransactionsXLSX(w http.ResponseWriter, r *http.Request) {
	rep, err := s.buildReportFor(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	setDownload(w, fmt.Sprintf("transaksjoner-%d.xlsx", rep.Year),
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	if err := export.TransactionsXLSX(w, rep, s.catNamer(rep.Year)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleNaeringsspesifikasjonXLSX(w http.ResponseWriter, r *http.Request) {
	rep, err := s.buildReportFor(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	setDownload(w, fmt.Sprintf("naeringsspesifikasjon-%d.xlsx", rep.Year),
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	if err := export.Naeringsspesifikasjon(w, rep, s.catNamer(rep.Year)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleTransactionsCSV(w http.ResponseWriter, r *http.Request) {
	rep, err := s.buildReportFor(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	setDownload(w, fmt.Sprintf("transaksjoner-%d.csv", rep.Year), "text/csv; charset=utf-8")
	if err := export.TransactionsCSV(w, rep, s.catNamer(rep.Year)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func setDownload(w http.ResponseWriter, filename, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
}
