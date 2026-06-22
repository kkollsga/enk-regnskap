package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
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
	EditID     int64        // 0 = ny
	Action     string       // hvor skjemaet postes
	Receipts   []db.Receipt // eksisterende vedlegg (ved redigering)
}

// incomeListData er datamodellen for inntektslisten.
type incomeListData struct {
	Income   []db.Income
	Total    float64
	Receipts map[int64][]db.Receipt
	CatNames map[string]string
}

func (s *Server) handleIncomeList(w http.ResponseWriter, r *http.Request) {
	v := s.view(r, "income", s.tr(r, "nav_income"))
	rows, err := s.app().ListIncome(r.Context(), v.Year)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var total float64
	receipts := map[int64][]db.Receipt{}
	for _, in := range rows {
		total += in.AmountNok
		receipts[in.ID], _ = s.app().ReceiptsFor(r.Context(), "income", in.ID)
	}
	catNames := map[string]string{}
	for _, c := range core.IncomeCategories() {
		catNames[c.Key] = c.Name
	}
	v.Data = incomeListData{Income: rows, Total: total, Receipts: receipts, CatNames: catNames}
	s.renderer.Render(w, "income_list", v)
}

func (s *Server) handleIncomeNew(w http.ResponseWriter, r *http.Request) {
	v := s.view(r, "income", s.tr(r, "action_add_income"))
	v.Data = s.newIncomeForm(r)
	s.renderer.Render(w, "income_form", v)
}

func (s *Server) handleIncomeEdit(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	inc, err := s.app().GetIncome(r.Context(), id)
	if err != nil {
		http.Error(w, "inntekt ikke funnet", http.StatusNotFound)
		return
	}
	v := s.view(r, "income", "Endre inntekt")
	form := s.newIncomeForm(r)
	form.EditID = id
	form.Action = "/income/" + strconv.FormatInt(id, 10)
	form.Values = incomeToValues(inc)
	form.Receipts, _ = s.app().ReceiptsFor(r.Context(), "income", id)
	v.Data = form
	s.renderer.Render(w, "income_form", v)
}

func (s *Server) handleIncomeCreate(w http.ResponseWriter, r *http.Request) {
	s.saveIncome(w, r, 0)
}

func (s *Server) handleIncomeUpdate(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	s.saveIncome(w, r, id)
}

// saveIncome håndterer både ny (id=0) og endring (id>0).
func (s *Server) saveIncome(w http.ResponseWriter, r *http.Request, id int64) {
	_ = r.ParseMultipartForm(32 << 20)
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
		ForeignTaxPaid:     parseInt(r.FormValue("foreign_tax_paid")),
		ForeignTaxOrig:     parseAmount(r.FormValue("foreign_tax_orig")),
	}

	render := func(errs map[string]string) {
		v := s.view(r, "income", s.tr(r, "action_add_income"))
		form := s.newIncomeForm(r)
		form.Values = formValues(r)
		form.Errors = errs
		if id > 0 {
			form.EditID = id
			form.Action = "/income/" + strconv.FormatInt(id, 10)
			form.Receipts, _ = s.app().ReceiptsFor(r.Context(), "income", id)
		}
		v.Data = form
		w.WriteHeader(http.StatusOK)
		s.renderer.Render(w, "income_form", v)
	}

	if msg := attachmentTypeError(r); msg != "" {
		render(map[string]string{"file": msg})
		return
	}

	var res *core.IncomeResult
	var err error
	if id > 0 {
		res, err = s.app().UpdateIncome(r.Context(), core.ActorWeb, id, in)
	} else {
		res, err = s.app().AddIncome(r.Context(), core.ActorWeb, in)
	}
	if err != nil {
		if ve, ok := core.AsValidation(err); ok {
			render(ve.Fields)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := s.uploadReceipts(r, "income", res.Income.ID, int(res.Income.TaxYear)); err != nil {
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
		Action:     "/income",
	}
}

// incomeToValues fyller skjemaverdier fra en eksisterende inntekt.
func incomeToValues(in db.Income) map[string]string {
	v := map[string]string{
		"date": in.Date, "description": in.Description, "category": in.Category,
		"client": in.Client.String, "country_code": in.CountryCode, "currency": in.Currency,
		"amount_orig": strconv.FormatFloat(in.AmountOrig, 'f', -1, 64), "notes": in.Notes.String,
		"foreign_tax_paid": strconv.FormatInt(in.ForeignTaxPaid, 10),
		"foreign_tax_type": in.ForeignTaxType.String, "foreign_tax_currency": in.ForeignTaxCurrency.String,
	}
	if in.ForeignTaxOrig.Valid {
		v["foreign_tax_orig"] = strconv.FormatFloat(in.ForeignTaxOrig.Float64, 'f', -1, 64)
	}
	return v
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
