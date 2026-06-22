package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/kkollsga/enk-regnskap/internal/db"
	"github.com/kkollsga/enk-regnskap/internal/tax"
)

// Foreign-tax-paid-tilstander for en inntekt.
const (
	ForeignTaxNo      = 0 // ingen utenlandsk skatt trukket
	ForeignTaxYes     = 1 // skatt trukket, beløp oppgitt
	ForeignTaxUnknown = 2 // vet ikke enna
)

// IncomeInput er brukerens/agentens innspill for en ny inntekt.
type IncomeInput struct {
	Date        string
	Description string
	Category    string
	Client      string
	CountryCode string  // ISO 3166-1, default 'NO'
	Currency    string  // default 'NOK'
	AmountOrig  float64 // beløp i valgt valuta
	TaxYear     int     // 0 = utled fra dato
	Notes       string

	ForeignTaxPaid     int     // 0/1/2
	ForeignTaxOrig     float64 // betalt skatt i utenlandsk valuta
	ForeignTaxCurrency string  // default = Currency
	ForeignTaxType     string  // f.eks. 'IRRF'

	ReceiptID *int64
}

// IncomeResult returnerer den lagrede inntekten og kursinfoen som ble brukt.
type IncomeResult struct {
	Income   db.Income
	RateUsed float64
	RateDate string
}

// resolvedIncome er ferdig beregnede DB-verdier for en inntekt.
type resolvedIncome struct {
	exchangeRate sql.NullFloat64
	rateDate     sql.NullString
	amountNOK    float64
	ftOrig       sql.NullFloat64
	ftNOK        sql.NullFloat64
	ftCurrency   sql.NullString
	ftType       sql.NullString
	usedRate     float64
	usedRateDate string
}

// resolveIncome henter valutakurs og beregner NOK-beløp + utenlandsk skatt.
func (a *App) resolveIncome(ctx context.Context, in IncomeInput) (resolvedIncome, error) {
	res := resolvedIncome{amountNOK: in.AmountOrig, usedRate: 1.0, usedRateDate: in.Date}
	if in.Currency != "NOK" {
		r, err := a.Currency.Rate(ctx, in.Currency, in.Date)
		if err != nil {
			ve := newValidation()
			ve.add("currency", "kunne ikke hente valutakurs: "+err.Error())
			return res, ve
		}
		res.usedRate = r.RateNOK
		res.usedRateDate = r.Date
		res.amountNOK = tax.Round2(in.AmountOrig * r.RateNOK)
		res.exchangeRate = sql.NullFloat64{Float64: r.RateNOK, Valid: true}
		res.rateDate = sql.NullString{String: r.Date, Valid: true}
	}
	if in.ForeignTaxPaid == ForeignTaxYes && in.ForeignTaxOrig > 0 {
		ftCur := in.ForeignTaxCurrency
		if ftCur == "" {
			ftCur = in.Currency
		}
		nok := in.ForeignTaxOrig
		if ftCur != "NOK" {
			r, err := a.Currency.Rate(ctx, ftCur, in.Date)
			if err != nil {
				ve := newValidation()
				ve.add("foreign_tax_orig", "kunne ikke hente kurs for utenlandsk skatt: "+err.Error())
				return res, ve
			}
			nok = tax.Round2(in.ForeignTaxOrig * r.RateNOK)
		}
		res.ftOrig = sql.NullFloat64{Float64: in.ForeignTaxOrig, Valid: true}
		res.ftNOK = sql.NullFloat64{Float64: nok, Valid: true}
		res.ftCurrency = sql.NullString{String: ftCur, Valid: true}
		if in.ForeignTaxType != "" {
			res.ftType = sql.NullString{String: in.ForeignTaxType, Valid: true}
		}
	}
	return res, nil
}

// AddIncome validerer, henter valutakurs, beregner NOK-beløp, lagrer inntekten,
// loggfor endringen og kringkaster en live-hendelse. actor er "web" eller "mcp".
func (a *App) AddIncome(ctx context.Context, actor string, in IncomeInput) (*IncomeResult, error) {
	in.normalize()
	if err := in.validate(); err != nil {
		return nil, err
	}
	res, err := a.resolveIncome(ctx, in)
	if err != nil {
		return nil, err
	}
	created, err := a.Q.CreateIncome(ctx, db.CreateIncomeParams{
		Date: in.Date, Description: in.Description, AmountOrig: in.AmountOrig,
		Currency: in.Currency, ExchangeRate: res.exchangeRate, RateDate: res.rateDate,
		AmountNok: res.amountNOK, Category: in.Category, Client: nullString(in.Client),
		CountryCode: in.CountryCode, ForeignTaxPaid: int64(in.ForeignTaxPaid),
		ForeignTaxOrig: res.ftOrig, ForeignTaxCurrency: res.ftCurrency, ForeignTaxNok: res.ftNOK,
		ForeignTaxType: res.ftType, ReceiptID: nullInt(in.ReceiptID),
		TaxYear: int64(in.TaxYear), Notes: nullString(in.Notes),
	})
	if err != nil {
		return nil, fmt.Errorf("lagre inntekt: %w", err)
	}
	after, _ := a.snapshotRow(ctx, "income", created.ID)
	desc := fmt.Sprintf("La til inntekt: %s (%s)", in.Description, formatMoney(res.amountNOK))
	if err := a.logChange(ctx, actor, "insert", "income", created.ID, nil, after, in.TaxYear, desc); err != nil {
		return nil, err
	}
	if in.CountryCode != "NO" {
		if err := a.RecomputeForeignTaxCredits(ctx, in.TaxYear); err != nil {
			return nil, err
		}
	}
	return &IncomeResult{Income: created, RateUsed: res.usedRate, RateDate: res.usedRateDate}, nil
}

// UpdateIncome oppdaterer en eksisterende inntekt (revisjonslogges).
func (a *App) UpdateIncome(ctx context.Context, actor string, id int64, in IncomeInput) (*IncomeResult, error) {
	in.normalize()
	if err := in.validate(); err != nil {
		return nil, err
	}
	before, err := a.snapshotRow(ctx, "income", id)
	if err != nil || before == nil {
		return nil, fmt.Errorf("inntekt %d finnes ikke", id)
	}
	res, err := a.resolveIncome(ctx, in)
	if err != nil {
		return nil, err
	}
	updated, err := a.Q.UpdateIncome(ctx, db.UpdateIncomeParams{
		ID: id, Date: in.Date, Description: in.Description, AmountOrig: in.AmountOrig,
		Currency: in.Currency, ExchangeRate: res.exchangeRate, RateDate: res.rateDate,
		AmountNok: res.amountNOK, Category: in.Category, Client: nullString(in.Client),
		CountryCode: in.CountryCode, ForeignTaxPaid: int64(in.ForeignTaxPaid),
		ForeignTaxOrig: res.ftOrig, ForeignTaxCurrency: res.ftCurrency, ForeignTaxNok: res.ftNOK,
		ForeignTaxType: res.ftType, TaxYear: int64(in.TaxYear), Notes: nullString(in.Notes),
	})
	if err != nil {
		return nil, fmt.Errorf("oppdater inntekt: %w", err)
	}
	after, _ := a.snapshotRow(ctx, "income", id)
	desc := fmt.Sprintf("Endret inntekt #%d: %s", id, in.Description)
	if err := a.logChange(ctx, actor, "update", "income", id, before, after, in.TaxYear, desc); err != nil {
		return nil, err
	}
	// Aggreger på nytt for både gammelt og nytt år/land.
	a.RecomputeForeignTaxCredits(ctx, in.TaxYear)
	if y := toInt(before["tax_year"]); y != in.TaxYear {
		a.RecomputeForeignTaxCredits(ctx, y)
	}
	return &IncomeResult{Income: updated, RateUsed: res.usedRate, RateDate: res.usedRateDate}, nil
}

// GetIncome henter en inntekt.
func (a *App) GetIncome(ctx context.Context, id int64) (db.Income, error) {
	return a.Q.GetIncome(ctx, id)
}

// DeleteIncome sletter en inntekt med revisjonsspor og live-hendelse.
func (a *App) DeleteIncome(ctx context.Context, actor string, id int64) error {
	before, err := a.snapshotRow(ctx, "income", id)
	if err != nil {
		return err
	}
	if before == nil {
		return fmt.Errorf("inntekt %d finnes ikke", id)
	}
	if err := a.Q.DeleteIncome(ctx, id); err != nil {
		return fmt.Errorf("slett inntekt: %w", err)
	}
	year := toInt(before["tax_year"])
	country, _ := before["country_code"].(string)
	desc := fmt.Sprintf("Slettet inntekt #%d", id)
	if err := a.logChange(ctx, actor, "delete", "income", id, before, nil, year, desc); err != nil {
		return err
	}
	if country != "NO" {
		if err := a.RecomputeForeignTaxCredits(ctx, year); err != nil {
			return err
		}
	}
	return nil
}

// ListIncome henter alle inntekter for et inntektsår.
func (a *App) ListIncome(ctx context.Context, year int) ([]db.Income, error) {
	return a.Q.ListIncomeByYear(ctx, int64(year))
}

// IncomeClients returnerer tidligere brukte klientnavn (autocomplete).
func (a *App) IncomeClients(ctx context.Context) ([]string, error) {
	rows, err := a.Q.DistinctClients(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.Valid && r.String != "" {
			out = append(out, r.String)
		}
	}
	return out, nil
}

// --- helpers ---

func (in *IncomeInput) normalize() {
	in.Date = strings.TrimSpace(in.Date)
	in.Description = strings.TrimSpace(in.Description)
	in.Category = strings.TrimSpace(in.Category)
	in.Client = strings.TrimSpace(in.Client)
	in.Notes = strings.TrimSpace(in.Notes)
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	if in.Currency == "" {
		in.Currency = "NOK"
	}
	in.CountryCode = strings.ToUpper(strings.TrimSpace(in.CountryCode))
	if in.CountryCode == "" {
		in.CountryCode = "NO"
	}
	in.ForeignTaxCurrency = strings.ToUpper(strings.TrimSpace(in.ForeignTaxCurrency))
	in.ForeignTaxType = strings.TrimSpace(in.ForeignTaxType)
	if in.TaxYear == 0 {
		in.TaxYear = yearFromDate(in.Date)
	}
}

func (in *IncomeInput) validate() error {
	ve := newValidation()
	if in.Date == "" || !validDate(in.Date) {
		ve.add("date", "Ugyldig eller manglende dato (forventet AAAA-MM-DD).")
	}
	if in.Description == "" {
		ve.add("description", "Beskrivelse er påkrevd.")
	}
	if in.AmountOrig <= 0 {
		ve.add("amount_orig", "Beløp må være større enn 0.")
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

func validDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func yearFromDate(s string) int {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Year()
	}
	return time.Now().Year()
}

func nullInt(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

func toInt(v any) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}

func formatMoney(v float64) string {
	return fmt.Sprintf("%.2f kr", v)
}
