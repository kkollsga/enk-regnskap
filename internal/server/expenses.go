package server

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/kkollsga/enk-regnskap/internal/core"
	"github.com/kkollsga/enk-regnskap/internal/db"
)

type expenseFormData struct {
	Values     map[string]string
	Errors     map[string]string
	Categories []core.ExpenseCategory
	Currencies []string
	Countries  []core.CountryOption
	Incomes    []db.Income // for «knytt til inntekt»-nedtrekk
	Today      string
	EditID     int64
	Action     string
	Receipts   []db.Receipt
}

type expenseListData struct {
	Expenses       []db.Expense
	Kinds          map[string]string // kategorinøkkel -> TaxKind
	CatNames       map[string]string
	Categories     []core.ExpenseCategory // for redigering i listen
	Currencies     []string
	Countries      []core.CountryOption
	Receipts       map[int64][]db.Receipt
	TotalAmount    float64
	TotalDeduct    float64
	TaxSummary     core.TaxExpenseSummary
	LinkedTaxes    []core.LinkedForeignTax // utenlandsk skatt ført som fradrag, koblet til inntekt
	LinkedTaxTotal float64
	Rows           []expenseRow // utgifter + koblede skattefradrag, sortert på dato
	TotalDeductAll float64      // TotalDeduct + LinkedTaxTotal
}

// expenseRow er en rad i den samlede fradragslisten: enten en vanlig utgift
// (redigerbar) eller en koblet utenlandsk skattelinje (skrivebeskyttet).
type expenseRow struct {
	Linked  bool
	Date    string
	Expense db.Expense
	Tax     core.LinkedForeignTax
}

func (s *Server) handleExpenseList(w http.ResponseWriter, r *http.Request) {
	v := s.view(r, "expenses", s.tr(r, "nav_expenses"))
	rows, err := s.app().ListExpenses(r.Context(), v.Year)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cats := s.app().ExpenseCategories(v.Year)
	kinds := map[string]string{}
	catNames := map[string]string{}
	for _, c := range cats {
		kinds[c.Key] = c.Kind
		catNames[c.Key] = c.Name
	}
	var amt, ded float64
	receipts := map[int64][]db.Receipt{}
	for _, e := range rows {
		amt += e.AmountNok
		ded += e.DeductibleNok
		receipts[e.ID], _ = s.app().ReceiptsFor(r.Context(), "expense", e.ID)
	}
	summary, _ := s.app().TaxExpenseSummaryForYear(r.Context(), v.Year)
	linked, _ := s.app().DeductibleForeignTaxLines(r.Context(), v.Year)
	var linkedTotal float64
	for _, l := range linked {
		linkedTotal += l.AmountNok
	}
	countries, _ := s.app().Countries(r.Context())
	// Samlet fradragsliste: utgifter + koblede skattefradrag, sortert på dato.
	combined := make([]expenseRow, 0, len(rows)+len(linked))
	for _, e := range rows {
		combined = append(combined, expenseRow{Date: e.Date, Expense: e})
	}
	for _, l := range linked {
		combined = append(combined, expenseRow{Linked: true, Date: l.IncomeDate, Tax: l})
	}
	sort.SliceStable(combined, func(i, j int) bool { return combined[i].Date > combined[j].Date })
	v.Data = expenseListData{
		Expenses: rows, Kinds: kinds, CatNames: catNames, Categories: cats, Receipts: receipts,
		Currencies: core.SupportedCurrencies(), Countries: countries,
		TotalAmount: amt, TotalDeduct: ded, TaxSummary: summary,
		LinkedTaxes: linked, LinkedTaxTotal: linkedTotal,
		Rows: combined, TotalDeductAll: ded + linkedTotal,
	}
	s.renderer.Render(w, "expenses_list", v)
}

func (s *Server) handleExpenseNew(w http.ResponseWriter, r *http.Request) {
	v := s.view(r, "expenses", s.tr(r, "action_add_expense"))
	v.Data = s.newExpenseForm(r, v.Year)
	s.renderer.Render(w, "expenses_form", v)
}

func (s *Server) handleExpenseEdit(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	exp, err := s.app().GetExpense(r.Context(), id)
	if err != nil {
		http.Error(w, "utgift ikke funnet", http.StatusNotFound)
		return
	}
	v := s.view(r, "expenses", "Endre utgift")
	form := s.newExpenseForm(r, v.Year)
	form.EditID = id
	form.Action = "/expenses/" + strconv.FormatInt(id, 10)
	form.Values = expenseToValues(exp)
	form.Receipts, _ = s.app().ReceiptsFor(r.Context(), "expense", id)
	v.Data = form
	s.renderer.Render(w, "expenses_form", v)
}

func (s *Server) handleExpenseCreate(w http.ResponseWriter, r *http.Request) {
	s.saveExpense(w, r, 0)
}

func (s *Server) handleExpenseUpdate(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	s.saveExpense(w, r, id)
}

func (s *Server) saveExpense(w http.ResponseWriter, r *http.Request, id int64) {
	_ = r.ParseMultipartForm(32 << 20)
	in := core.ExpenseInput{
		Date:        r.FormValue("date"),
		Description: r.FormValue("description"),
		Category:    r.FormValue("category"),
		CountryCode: r.FormValue("country_code"),
		Currency:    r.FormValue("currency"),
		AmountOrig:  parseAmount(r.FormValue("amount_orig")),
		Notes:       r.FormValue("notes"),
	}
	if v := parseInt(r.FormValue("income_id")); v > 0 {
		id := int64(v)
		in.IncomeID = &id
	}
	if pctStr := r.FormValue("deductible_pct"); pctStr != "" {
		in.DeductiblePct = parseAmount(pctStr)
		in.HasDeductiblePct = true
	}

	render := func(errs map[string]string) {
		v := s.view(r, "expenses", s.tr(r, "action_add_expense"))
		form := s.newExpenseForm(r, v.Year)
		form.Values = map[string]string{
			"date": r.FormValue("date"), "description": r.FormValue("description"),
			"amount_orig": r.FormValue("amount_orig"), "category": r.FormValue("category"),
			"currency": r.FormValue("currency"), "country_code": r.FormValue("country_code"),
			"deductible_pct": r.FormValue("deductible_pct"), "notes": r.FormValue("notes"),
			"income_id": r.FormValue("income_id"),
		}
		form.Errors = errs
		if id > 0 {
			form.EditID = id
			form.Action = "/expenses/" + strconv.FormatInt(id, 10)
			form.Receipts, _ = s.app().ReceiptsFor(r.Context(), "expense", id)
		}
		v.Data = form
		w.WriteHeader(http.StatusOK)
		s.renderer.Render(w, "expenses_form", v)
	}

	if msg := attachmentTypeError(r); msg != "" {
		render(map[string]string{"file": msg})
		return
	}
	if active := s.app().ActiveYear(r.Context()); !dateInYear(in.Date, active) {
		render(map[string]string{"date": wrongYearMsg(active)})
		return
	}

	var exp *db.Expense
	var err error
	if id > 0 {
		exp, err = s.app().UpdateExpense(r.Context(), core.ActorWeb, id, in)
	} else {
		exp, err = s.app().AddExpense(r.Context(), core.ActorWeb, in)
	}
	if err != nil {
		if ve, ok := core.AsValidation(err); ok {
			render(ve.Fields)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := s.uploadReceipts(r, "expense", exp.ID, int(exp.TaxYear)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.rememberCountryCurrency(r.Context(), exp.CountryCode, exp.Currency)
	http.Redirect(w, r, "/expenses?saved=1", http.StatusSeeOther)
}

// entryDefaultDate gir standard dato for en ny post: i dag hvis vi er i det
// aktive året, ellers samme dag/måned i det aktive året (så posten havner i
// riktig år).
func entryDefaultDate(activeYear int) string {
	now := time.Now()
	if now.Year() == activeYear {
		return now.Format("2006-01-02")
	}
	return time.Date(activeYear, now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

// dateInYear er sann hvis datoen er en gyldig dato i det gitte året. En ugyldig
// dato slipper gjennom her (kjernens validering fanger formatfeil).
func dateInYear(dateStr string, year int) bool {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(dateStr))
	if err != nil {
		return true
	}
	return t.Year() == year
}

func wrongYearMsg(year int) string {
	return "Datoen må være i inntektsåret " + strconv.Itoa(year) + ". Bytt år øverst hvis du vil føre i et annet år."
}

func (s *Server) newExpenseForm(r *http.Request, year int) expenseFormData {
	values := map[string]string{
		"date":         entryDefaultDate(year),
		"currency":     s.app().GetConfig(r.Context(), core.ConfigLastCurrency, "NOK"),
		"country_code": s.app().GetConfig(r.Context(), core.ConfigLastCountry, "NO"),
	}
	if cat := r.URL.Query().Get("category"); cat != "" {
		values["category"] = cat
	}
	countries, _ := s.app().Countries(r.Context())
	incomes, _ := s.app().ListIncome(r.Context(), year)
	return expenseFormData{
		Values:     values,
		Errors:     map[string]string{},
		Categories: s.app().ExpenseCategories(year),
		Currencies: core.SupportedCurrencies(),
		Countries:  countries,
		Incomes:    incomes,
		Today:      time.Now().Format("2006-01-02"),
		Action:     "/expenses",
	}
}

func expenseToValues(e db.Expense) map[string]string {
	v := map[string]string{
		"date": e.Date, "description": e.Description, "category": e.Category,
		"currency": e.Currency, "country_code": e.CountryCode,
		"amount_orig":    strconv.FormatFloat(e.AmountOrig, 'f', -1, 64),
		"deductible_pct": strconv.FormatFloat(e.DeductiblePct, 'f', -1, 64),
		"notes":          e.Notes.String,
	}
	if e.IncomeID.Valid {
		v["income_id"] = strconv.FormatInt(e.IncomeID.Int64, 10)
	}
	return v
}
