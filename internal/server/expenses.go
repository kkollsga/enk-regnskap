package server

import (
	"net/http"
	"time"

	"github.com/kkollsga/enk-regnskap/internal/core"
	"github.com/kkollsga/enk-regnskap/internal/db"
)

type expenseFormData struct {
	Values     map[string]string
	Errors     map[string]string
	Categories []core.ExpenseCategory
	Today      string
}

type expenseListData struct {
	Expenses    []db.Expense
	TotalAmount float64
	TotalDeduct float64
}

func (s *Server) handleExpenseList(w http.ResponseWriter, r *http.Request) {
	v := s.view(r, "expenses", s.tr(r, "nav_expenses"))
	rows, err := s.app().ListExpenses(r.Context(), v.Year)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var amt, ded float64
	for _, e := range rows {
		amt += e.AmountNok
		ded += e.DeductibleNok
	}
	v.Data = expenseListData{Expenses: rows, TotalAmount: amt, TotalDeduct: ded}
	s.renderer.Render(w, "expenses_list", v)
}

func (s *Server) handleExpenseNew(w http.ResponseWriter, r *http.Request) {
	v := s.view(r, "expenses", s.tr(r, "action_add_expense"))
	v.Data = s.newExpenseForm(r, v.Year)
	s.renderer.Render(w, "expenses_form", v)
}

func (s *Server) handleExpenseCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		// Ikke multipart? Prov vanlig skjema.
		if err := r.ParseForm(); err != nil {
			http.Error(w, "ugyldig skjema", http.StatusBadRequest)
			return
		}
	}

	in := core.ExpenseInput{
		Date:        r.FormValue("date"),
		Description: r.FormValue("description"),
		Category:    r.FormValue("category"),
		AmountNOK:   parseAmount(r.FormValue("amount_nok")),
		Notes:       r.FormValue("notes"),
	}
	if pctStr := r.FormValue("deductible_pct"); pctStr != "" {
		in.DeductiblePct = parseAmount(pctStr)
		in.HasDeductiblePct = true
	}

	// Valgfri kvittering.
	rid, upErr := s.maybeUploadReceipt(r, parseYear(r.FormValue("date")))
	if upErr != nil {
		s.renderExpenseError(w, r, map[string]string{"file": upErr.Error()})
		return
	}
	if rid != nil {
		in.ReceiptID = rid
	}

	_, err := s.app().AddExpense(r.Context(), core.ActorWeb, in)
	if err != nil {
		if ve, isVE := core.AsValidation(err); isVE {
			s.renderExpenseError(w, r, ve.Fields)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/expenses?saved=1", http.StatusSeeOther)
}

func (s *Server) renderExpenseError(w http.ResponseWriter, r *http.Request, errs map[string]string) {
	v := s.view(r, "expenses", s.tr(r, "action_add_expense"))
	form := s.newExpenseForm(r, v.Year)
	form.Values = map[string]string{
		"date": r.FormValue("date"), "description": r.FormValue("description"),
		"amount_nok": r.FormValue("amount_nok"), "category": r.FormValue("category"),
		"deductible_pct": r.FormValue("deductible_pct"), "notes": r.FormValue("notes"),
	}
	form.Errors = errs
	v.Data = form
	w.WriteHeader(http.StatusOK)
	s.renderer.Render(w, "expenses_form", v)
}

func (s *Server) newExpenseForm(r *http.Request, year int) expenseFormData {
	return expenseFormData{
		Values: map[string]string{
			"date": time.Now().Format("2006-01-02"),
		},
		Errors:     map[string]string{},
		Categories: s.app().ExpenseCategories(year),
		Today:      time.Now().Format("2006-01-02"),
	}
}

// parseYear utleder aaret fra en ISO-dato (for kvitteringslagring).
func parseYear(date string) int {
	if t, err := time.Parse("2006-01-02", date); err == nil {
		return t.Year()
	}
	return time.Now().Year()
}
