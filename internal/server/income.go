package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kkollsga/enk-regnskap/internal/core"
	"github.com/kkollsga/enk-regnskap/internal/db"
)

// incomeFormData er datamodellen for inntektsskjemaet.
type incomeFormData struct {
	Values     map[string]string
	Errors     map[string]string
	Categories []core.Category
	Currencies []string
	Countries  []core.CountryOption
	Clients    []string
	Today      string
}

// incomeListData er datamodellen for inntektslisten.
type incomeListData struct {
	Income []db.Income
	Total  float64
}

func (s *Server) handleIncomeList(w http.ResponseWriter, r *http.Request) {
	v := s.view(r, "income", s.tr(r, "nav_income"))
	rows, err := s.app().ListIncome(r.Context(), v.Year)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var total float64
	for _, in := range rows {
		total += in.AmountNok
	}
	v.Data = incomeListData{Income: rows, Total: total}
	s.renderer.Render(w, "income_list", v)
}

func (s *Server) handleIncomeNew(w http.ResponseWriter, r *http.Request) {
	v := s.view(r, "income", s.tr(r, "action_add_income"))
	v.Data = s.newIncomeForm(r)
	s.renderer.Render(w, "income_form", v)
}

func (s *Server) handleIncomeCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "ugyldig skjema", http.StatusBadRequest)
			return
		}
	}
	in := core.IncomeInput{
		Date:               r.FormValue("date"),
		Description:        r.FormValue("description"),
		Category:           r.FormValue("category"),
		Client:             r.FormValue("client"),
		CountryCode:        r.FormValue("country_code"),
		Currency:           r.FormValue("currency"),
		AmountOrig:         parseAmount(r.FormValue("amount_orig")),
		Notes:              r.FormValue("notes"),
		ForeignTaxType:     r.FormValue("foreign_tax_type"),
		ForeignTaxCurrency: r.FormValue("foreign_tax_currency"),
	}
	in.ForeignTaxPaid = parseInt(r.FormValue("foreign_tax_paid"))
	in.ForeignTaxOrig = parseAmount(r.FormValue("foreign_tax_orig"))

	// Valgfri kvittering.
	rid, upErr := s.maybeUploadReceipt(r, parseYear(r.FormValue("date")))
	if upErr != nil {
		v := s.view(r, "income", s.tr(r, "action_add_income"))
		form := s.newIncomeForm(r)
		form.Values = formValues(r)
		form.Errors = map[string]string{"file": upErr.Error()}
		v.Data = form
		w.WriteHeader(http.StatusOK)
		s.renderer.Render(w, "income_form", v)
		return
	}
	in.ReceiptID = rid

	_, err := s.app().AddIncome(r.Context(), core.ActorWeb, in)
	if err != nil {
		if ve, ok := core.AsValidation(err); ok {
			// Vis feil inline, behold utfylte verdier.
			v := s.view(r, "income", s.tr(r, "action_add_income"))
			form := s.newIncomeForm(r)
			form.Values = formValues(r)
			form.Errors = ve.Fields
			v.Data = form
			w.WriteHeader(http.StatusOK)
			s.renderer.Render(w, "income_form", v)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/income?saved=1", http.StatusSeeOther)
}

func (s *Server) newIncomeForm(r *http.Request) incomeFormData {
	countries, _ := s.app().Countries(r.Context())
	clients, _ := s.app().IncomeClients(r.Context())
	return incomeFormData{
		Values: map[string]string{
			"date":             time.Now().Format("2006-01-02"),
			"currency":         "NOK",
			"country_code":     "NO",
			"foreign_tax_paid": "0",
		},
		Errors:     map[string]string{},
		Categories: core.IncomeCategories(),
		Currencies: core.SupportedCurrencies(),
		Countries:  countries,
		Clients:    clients,
		Today:      time.Now().Format("2006-01-02"),
	}
}

// formValues plukker ut innsendte verdier for gjenvisning ved feil.
func formValues(r *http.Request) map[string]string {
	keys := []string{"date", "description", "client", "country_code", "currency",
		"amount_orig", "category", "notes", "foreign_tax_paid", "foreign_tax_orig",
		"foreign_tax_type", "foreign_tax_currency"}
	m := make(map[string]string, len(keys))
	for _, k := range keys {
		m[k] = r.FormValue(k)
	}
	return m
}

// parseAmount tolker et beløp som kan bruke komma som desimalskille.
func parseAmount(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ",", ".")
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

func parseInt(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
