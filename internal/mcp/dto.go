package mcp

import (
	"database/sql"

	"github.com/kkollsga/enk-regnskap/internal/core"
	"github.com/kkollsga/enk-regnskap/internal/db"
	"github.com/kkollsga/enk-regnskap/internal/tax"
)

// Rene DTO-er for verktoyutdata. Database-radene bruker sql.Null*-innpakninger
// ({"String":..,"Valid":..}) som er ordrike og lekker en intern representasjon.
// Disse strukturene flater ut nullbare felt (verdi-eller-utelatt) og dropper
// støy (receipt_id, before/after_json) så agenten får kompakt, lesbar JSON.

func nf(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	x := v.Float64
	return &x
}
func ns(v sql.NullString) string {
	if !v.Valid {
		return ""
	}
	return v.String
}
func ni(v sql.NullInt64) *int64 {
	if !v.Valid || v.Int64 == 0 {
		return nil
	}
	x := v.Int64
	return &x
}

type incomeOut struct {
	ID             int64    `json:"id"`
	Date           string   `json:"date"`
	Description    string   `json:"description"`
	AmountOrig     float64  `json:"amount_orig"`
	Currency       string   `json:"currency"`
	AmountNok      float64  `json:"amount_nok"`
	ExchangeRate   *float64 `json:"exchange_rate,omitempty"`
	RateDate       string   `json:"rate_date,omitempty"`
	Category       string   `json:"category"`
	Client         string   `json:"client,omitempty"`
	CountryCode    string   `json:"country_code"`
	ForeignTaxPaid int64    `json:"foreign_tax_paid"`
	TaxYear        int64    `json:"tax_year"`
	Notes          string   `json:"notes,omitempty"`
}

func incomeDTO(in db.Income) incomeOut {
	return incomeOut{
		ID: in.ID, Date: in.Date, Description: in.Description,
		AmountOrig: in.AmountOrig, Currency: in.Currency, AmountNok: in.AmountNok,
		ExchangeRate: nf(in.ExchangeRate), RateDate: ns(in.RateDate),
		Category: in.Category, Client: ns(in.Client), CountryCode: in.CountryCode,
		ForeignTaxPaid: in.ForeignTaxPaid, TaxYear: in.TaxYear, Notes: ns(in.Notes),
	}
}

func incomeDTOs(rows []db.Income) []incomeOut {
	out := make([]incomeOut, len(rows))
	for i, r := range rows {
		out[i] = incomeDTO(r)
	}
	return out
}

type expenseOut struct {
	ID            int64    `json:"id"`
	Date          string   `json:"date"`
	Description   string   `json:"description"`
	AmountOrig    float64  `json:"amount_orig"`
	Currency      string   `json:"currency"`
	AmountNok     float64  `json:"amount_nok"`
	ExchangeRate  *float64 `json:"exchange_rate,omitempty"`
	RateDate      string   `json:"rate_date,omitempty"`
	CountryCode   string   `json:"country_code"`
	Category      string   `json:"category"`
	DeductiblePct float64  `json:"deductible_pct"`
	DeductibleNok float64  `json:"deductible_nok"`
	IncomeID      *int64   `json:"income_id,omitempty"`
	TaxYear       int64    `json:"tax_year"`
	Notes         string   `json:"notes,omitempty"`
}

func expenseDTO(e db.Expense) expenseOut {
	return expenseOut{
		ID: e.ID, Date: e.Date, Description: e.Description,
		AmountOrig: e.AmountOrig, Currency: e.Currency, AmountNok: e.AmountNok,
		ExchangeRate: nf(e.ExchangeRate), RateDate: ns(e.RateDate),
		CountryCode: e.CountryCode, Category: e.Category,
		DeductiblePct: e.DeductiblePct, DeductibleNok: e.DeductibleNok,
		IncomeID: ni(e.IncomeID), TaxYear: e.TaxYear, Notes: ns(e.Notes),
	}
}

func expenseDTOs(rows []db.Expense) []expenseOut {
	out := make([]expenseOut, len(rows))
	for i, r := range rows {
		out[i] = expenseDTO(r)
	}
	return out
}

type changeOut struct {
	ID         int64  `json:"id"`
	Ts         string `json:"ts"`
	Actor      string `json:"actor"`
	Operation  string `json:"operation"`
	Entity     string `json:"entity"`
	EntityID   *int64 `json:"entity_id,omitempty"`
	Desc       string `json:"description"`
	RolledBack bool   `json:"rolled_back,omitempty"`
}

func changeDTOs(rows []db.ChangeLog) []changeOut {
	out := make([]changeOut, len(rows))
	for i, c := range rows {
		out[i] = changeOut{
			ID: c.ID, Ts: c.Ts, Actor: c.Actor, Operation: c.Operation,
			Entity: c.Entity, EntityID: ni(c.EntityID), Desc: c.Description,
			RolledBack: c.RolledBack != 0,
		}
	}
	return out
}

type foreignOverviewOut struct {
	CountryCode   string  `json:"country_code"`
	CountryName   string  `json:"country_name"`
	IncomeNok     float64 `json:"income_nok"`
	ForeignTaxNok float64 `json:"foreign_tax_nok"`
	Currency      string  `json:"foreign_currency,omitempty"`
	MaxCreditEst  float64 `json:"max_credit_est"`
	DocsMissing   bool    `json:"docs_missing"`
	LegalBasisRef string  `json:"legal_basis_ref,omitempty"`
}

func foreignOverviewDTOs(rows []core.ForeignTaxOverview) []foreignOverviewOut {
	out := make([]foreignOverviewOut, len(rows))
	for i, o := range rows {
		out[i] = foreignOverviewOut{
			CountryCode: o.Credit.CountryCode, CountryName: o.Credit.CountryName,
			IncomeNok: o.Credit.IncomeNok, ForeignTaxNok: o.Credit.ForeignTaxNok,
			Currency: o.Credit.ForeignCurrency, MaxCreditEst: o.MaxCreditEst,
			DocsMissing: o.DocsMissing, LegalBasisRef: o.LegalBasisRef,
		}
	}
	return out
}

type reportOut struct {
	Year                 int                  `json:"year"`
	BusinessName         string               `json:"business_name,omitempty"`
	OrgNr                string               `json:"org_nr,omitempty"`
	Income               []incomeOut          `json:"income,omitempty"`
	Expenses             []expenseOut         `json:"expenses,omitempty"`
	IncomeByCategory     []core.CategorySum   `json:"income_by_category"`
	ExpenseByCategory    []core.CategorySum   `json:"expense_by_category"`
	Foreign              []foreignOverviewOut `json:"foreign_credits,omitempty"`
	ForeignTaxDeductible float64              `json:"foreign_tax_deductible,omitempty"`
	ForeignTaxReference  float64              `json:"foreign_tax_reference,omitempty"`
	TotalIncome          float64              `json:"total_income"`
	TotalDeductible      float64              `json:"total_deductible"`
	Result               float64              `json:"result"`
	Tax                  *tax.TaxEstimate     `json:"tax,omitempty"`
}

// reportDTO bygger en kompakt rapport. foreign sendes inn separat (beriket
// oversikt) for å unngå de rå ForeignTaxCredit-radene med null-innpakninger.
func reportDTO(rep core.Report, foreign []core.ForeignTaxOverview) reportOut {
	return reportOut{
		Year: rep.Year, BusinessName: rep.BusinessName, OrgNr: rep.OrgNr,
		Income: incomeDTOs(rep.Income), Expenses: expenseDTOs(rep.Expenses),
		IncomeByCategory: rep.IncomeByCategory, ExpenseByCategory: rep.ExpenseByCategory,
		Foreign:              foreignOverviewDTOs(foreign),
		ForeignTaxDeductible: rep.ForeignTaxDeductible, ForeignTaxReference: rep.ForeignTaxReference,
		TotalIncome: rep.TotalIncome, TotalDeductible: rep.TotalDeductible,
		Result: rep.Result, Tax: rep.Tax,
	}
}
