package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/kkollsga/enk-regnskap/internal/db"
	"github.com/kkollsga/enk-regnskap/internal/tax"
)

// Kind for en utgiftskategori.
const (
	TaxKindNone          = ""              // vanlig driftskostnad
	TaxKindCreditable    = "creditable"    // utenlandsk skatt som gir kreditfradrag
	TaxKindNonCreditable = "noncreditable" // utenlandsk skatt uten kredit (overskytende)
)

// Gruppenavn for kategorier i nedtrekksmenyen.
const (
	GroupDriftskostnad = "Driftskostnader"
	GroupSkattBrasil   = "Skatt – Brasil (skatteavtale)"
)

// ExpenseCategory beriker en fradragskategori med satser for et inntektsår.
type ExpenseCategory struct {
	Key            string
	Name           string
	Description    string
	DefaultPct     float64
	SjablongAmount float64
	MaxAmount      float64
	Note           string
	Kind           string // TaxKind*: vanlig kostnad eller utenlandsk skatt
	Group          string
}

// ExpenseCategories henter fradragskategoriene for et inntektsår fra
// skattereglene, pluss kategorier for utenlandsk skatt (skatteavtale Brasil).
func (a *App) ExpenseCategories(year int) []ExpenseCategory {
	rules, err := tax.Load(year)
	if err != nil {
		return ForeignTaxExpenseCategories()
	}
	out := make([]ExpenseCategory, 0, len(rules.Deductions)+6)
	for _, d := range rules.Deductions {
		out = append(out, ExpenseCategory{
			Key: d.Key, Name: d.Name, Description: d.Description,
			DefaultPct: d.DefaultPct, SjablongAmount: d.SjablongAmount,
			MaxAmount: d.MaxAmount, Note: d.Note,
			Kind: TaxKindNone, Group: GroupDriftskostnad,
		})
	}
	out = append(out, ForeignTaxExpenseCategories()...)
	return out
}

// ExpenseCategoryKind returnerer kategoriens kind (vanlig / kreditberettiget /
// ikke-krediterbar utenlandsk skatt).
func (a *App) ExpenseCategoryKind(year int, key string) string {
	if c, ok := a.expenseCategory(year, key); ok {
		return c.Kind
	}
	return TaxKindNone
}

func (a *App) expenseCategory(year int, key string) (ExpenseCategory, bool) {
	for _, c := range a.ExpenseCategories(year) {
		if c.Key == key {
			return c, true
		}
	}
	return ExpenseCategory{}, false
}

// ExpenseInput er innspillet for en ny utgift (alltid i NOK).
type ExpenseInput struct {
	Date             string
	Description      string
	Category         string
	AmountNOK        float64
	DeductiblePct    float64
	HasDeductiblePct bool // true hvis bruker eksplisitt satte prosent
	TaxYear          int
	Notes            string
	ReceiptID        *int64
}

// AddExpense validerer, beregner fradragsberettiget beløp, lagrer utgiften og
// loggfor endringen.
func (a *App) AddExpense(ctx context.Context, actor string, in ExpenseInput) (*db.Expense, error) {
	in.normalize()
	if err := in.validate(); err != nil {
		return nil, err
	}

	pct := in.DeductiblePct
	if !in.HasDeductiblePct {
		if cat, ok := a.expenseCategory(in.TaxYear, in.Category); ok {
			pct = cat.DefaultPct
		} else {
			pct = 100
		}
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	deductible := tax.Round2(in.AmountNOK * pct / 100.0)

	created, err := a.Q.CreateExpense(ctx, db.CreateExpenseParams{
		Date:          in.Date,
		Description:   in.Description,
		AmountNok:     in.AmountNOK,
		Category:      in.Category,
		DeductiblePct: pct,
		DeductibleNok: deductible,
		ReceiptID:     nullInt(in.ReceiptID),
		TaxYear:       int64(in.TaxYear),
		Notes:         nullString(in.Notes),
	})
	if err != nil {
		return nil, fmt.Errorf("lagre utgift: %w", err)
	}
	after, _ := a.snapshotRow(ctx, "expenses", created.ID)
	desc := fmt.Sprintf("La til utgift: %s (%s, %g%% fradrag)", in.Description, formatMoney(deductible), pct)
	if err := a.logChange(ctx, actor, "insert", "expenses", created.ID, nil, after, in.TaxYear, desc); err != nil {
		return nil, err
	}
	return &created, nil
}

// DeleteExpense sletter en utgift med revisjonsspor.
func (a *App) DeleteExpense(ctx context.Context, actor string, id int64) error {
	before, err := a.snapshotRow(ctx, "expenses", id)
	if err != nil {
		return err
	}
	if before == nil {
		return fmt.Errorf("utgift %d finnes ikke", id)
	}
	if err := a.Q.DeleteExpense(ctx, id); err != nil {
		return fmt.Errorf("slett utgift: %w", err)
	}
	year := toInt(before["tax_year"])
	return a.logChange(ctx, actor, "delete", "expenses", id, before, nil, year, fmt.Sprintf("Slettet utgift #%d", id))
}

// ListExpenses henter alle utgifter for et inntektsår.
func (a *App) ListExpenses(ctx context.Context, year int) ([]db.Expense, error) {
	return a.Q.ListExpensesByYear(ctx, int64(year))
}

func (in *ExpenseInput) normalize() {
	in.Date = strings.TrimSpace(in.Date)
	in.Description = strings.TrimSpace(in.Description)
	in.Category = strings.TrimSpace(in.Category)
	in.Notes = strings.TrimSpace(in.Notes)
	if in.TaxYear == 0 {
		in.TaxYear = yearFromDate(in.Date)
	}
}

func (in *ExpenseInput) validate() error {
	ve := newValidation()
	if in.Date == "" || !validDate(in.Date) {
		ve.add("date", "Ugyldig eller manglende dato (forventet AAAA-MM-DD).")
	}
	if in.Description == "" {
		ve.add("description", "Beskrivelse er påkrevd.")
	}
	if in.AmountNOK <= 0 {
		ve.add("amount_nok", "Beløp må være større enn 0.")
	}
	if in.Category == "" {
		ve.add("category", "Velg en kategori.")
	}
	if in.TaxYear < 2000 || in.TaxYear > 2100 {
		ve.add("date", "Kunne ikke utlede inntektsar fra dato.")
	}
	if ve.has() {
		return ve
	}
	return nil
}
