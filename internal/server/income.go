package server

import (
	"context"
	"encoding/json"
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
	Values         map[string]string
	Errors         map[string]string
	Categories     []core.Category
	Currencies     []string
	Countries      []core.CountryOption
	Clients        []string
	TaxLines       []db.IncomeForeignTax // eksisterende/innsendte skattelinjer
	TaxSuggestions string                // JSON: {landkode: [{code,name,desc}]}
	Today          string
	EditID         int64        // 0 = ny
	Action         string       // hvor skjemaet postes
	Receipts       []db.Receipt // eksisterende vedlegg (ved redigering)
}

// incomeListData er datamodellen for inntektslisten.
type incomeListData struct {
	Income         []db.Income
	Total          float64
	Receipts       map[int64][]db.Receipt
	TaxLines       map[int64][]db.IncomeForeignTax
	TaxSuggestions string // JSON: {landkode: [{code,name,desc}]}
	CatNames       map[string]string
	Categories     []core.Category      // for redigering i listen
	Currencies     []string             // for redigering i listen
	Countries      []core.CountryOption // for redigering i listen
	Clients        []string             // for redigering i listen
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
	taxLines := map[int64][]db.IncomeForeignTax{}
	for _, in := range rows {
		total += in.AmountNok
		receipts[in.ID], _ = s.app().ReceiptsFor(r.Context(), "income", in.ID)
		if in.ForeignTaxPaid == core.ForeignTaxYes {
			taxLines[in.ID], _ = s.app().IncomeForeignTaxes(r.Context(), in.ID)
		}
	}
	cats := core.IncomeCategories()
	catNames := map[string]string{}
	for _, c := range cats {
		catNames[c.Key] = c.Name
	}
	countries, _ := s.app().Countries(r.Context())
	clients, _ := s.app().IncomeClients(r.Context())
	v.Data = incomeListData{
		Income: rows, Total: total, Receipts: receipts, TaxLines: taxLines,
		TaxSuggestions: s.taxSuggestionsJSON(r.Context()), CatNames: catNames,
		Categories: cats, Currencies: core.SupportedCurrencies(),
		Countries: countries, Clients: clients,
	}
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
	form.TaxLines, _ = s.app().IncomeForeignTaxes(r.Context(), id)
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
		Date:           r.FormValue("date"),
		Description:    r.FormValue("description"),
		Category:       r.FormValue("category"),
		Client:         r.FormValue("client"),
		CountryCode:    r.FormValue("country_code"),
		Currency:       r.FormValue("currency"),
		AmountOrig:     parseAmount(r.FormValue("amount_orig")),
		Notes:          r.FormValue("notes"),
		ForeignTaxPaid: parseInt(r.FormValue("foreign_tax_paid")),
		ForeignTaxes:   parseForeignTaxLines(r),
	}

	render := func(errs map[string]string) {
		v := s.view(r, "income", s.tr(r, "action_add_income"))
		form := s.newIncomeForm(r)
		form.Values = formValues(r)
		form.Errors = errs
		form.TaxLines = taxLinesFromForm(r)
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
	if active := s.app().ActiveYear(r.Context()); !dateInYear(in.Date, active) {
		render(map[string]string{"date": wrongYearMsg(active)})
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
			"date":             entryDefaultDate(s.app().ActiveYear(r.Context())),
			"currency":         "NOK",
			"country_code":     "NO",
			"foreign_tax_paid": "0",
		},
		Errors:         map[string]string{},
		Categories:     core.IncomeCategories(),
		Currencies:     core.SupportedCurrencies(),
		Countries:      countries,
		Clients:        clients,
		TaxSuggestions: s.taxSuggestionsJSON(r.Context()),
		Today:          time.Now().Format("2006-01-02"),
		Action:         "/income",
	}
}

// taxSuggestionsJSON returnerer skattetype-forslag per land som JSON-streng for
// inntektsskjemaets combobox, for det aktive året. Feiler det, returneres et
// tomt objekt.
func (s *Server) taxSuggestionsJSON(ctx context.Context) string {
	m, err := s.app().TaxTypeSuggestions(ctx, s.app().ActiveYear(ctx))
	if err != nil {
		return "{}"
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// incomeToValues fyller skjemaverdier fra en eksisterende inntekt.
func incomeToValues(in db.Income) map[string]string {
	return map[string]string{
		"date": in.Date, "description": in.Description, "category": in.Category,
		"client": in.Client.String, "country_code": in.CountryCode, "currency": in.Currency,
		"amount_orig": strconv.FormatFloat(in.AmountOrig, 'f', -1, 64), "notes": in.Notes.String,
		"foreign_tax_paid": strconv.FormatInt(in.ForeignTaxPaid, 10),
	}
}

// formValues plukker ut innsendte verdier for gjenvisning ved feil.
func formValues(r *http.Request) map[string]string {
	keys := []string{"date", "description", "client", "country_code", "currency",
		"amount_orig", "category", "notes", "foreign_tax_paid"}
	m := make(map[string]string, len(keys))
	for _, k := range keys {
		m[k] = r.FormValue(k)
	}
	return m
}

// parseForeignTaxLines leser de gjentatte skattelinje-feltene (tax_type[] +
// tax_amount[]) fra skjemaet til core-modellen.
func parseForeignTaxLines(r *http.Request) []core.ForeignTaxLine {
	types := r.Form["tax_type"]
	amounts := r.Form["tax_amount"]
	treatments := r.Form["tax_treatment"]
	var out []core.ForeignTaxLine
	for i := range types {
		amt := 0.0
		if i < len(amounts) {
			amt = parseAmount(amounts[i])
		}
		tr := ""
		if i < len(treatments) {
			tr = treatments[i]
		}
		out = append(out, core.ForeignTaxLine{Type: types[i], AmountOrig: amt, Treatment: tr})
	}
	return out
}

// taxLinesToValues bygger gjenvisningsverdier for innsendte skattelinjer ved
// valideringsfeil (parallelle arrayer).
func taxLinesFromForm(r *http.Request) []db.IncomeForeignTax {
	types := r.Form["tax_type"]
	amounts := r.Form["tax_amount"]
	treatments := r.Form["tax_treatment"]
	var out []db.IncomeForeignTax
	for i := range types {
		if strings.TrimSpace(types[i]) == "" {
			continue
		}
		amt := 0.0
		if i < len(amounts) {
			amt = parseAmount(amounts[i])
		}
		tr := ""
		if i < len(treatments) {
			tr = treatments[i]
		}
		out = append(out, db.IncomeForeignTax{TaxType: types[i], AmountOrig: amt, Treatment: tr})
	}
	return out
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
