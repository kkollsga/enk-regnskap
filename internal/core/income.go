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

// Skattemessig behandling av en utenlandsk skattelinje i Norge.
const (
	TaxTreatmentCredit = "credit" // kreditfradrag (sktl. § 16-20 flg.)
	TaxTreatmentDeduct = "deduct" // fradragsberettiget kostnad (sktl. § 6-1/§ 6-15)
	TaxTreatmentNone   = "none"   // ingen lettelse – kun referanse
)

// isTreatment sjekker om en streng er en gyldig behandlingskode.
func isTreatment(s string) bool {
	return s == TaxTreatmentCredit || s == TaxTreatmentDeduct || s == TaxTreatmentNone
}

// ForeignTaxLine er én betalt utenlandsk skatt av en bestemt type (f.eks. IRRF)
// på en inntekt. En inntekt kan ha flere slike linjer.
type ForeignTaxLine struct {
	Type       string  // f.eks. 'IRRF', 'ISS', 'CSLL'
	AmountOrig float64 // beløp i utenlandsk valuta
	Currency   string  // default = inntektens valuta
	Treatment  string  // 'credit'/'deduct'/'none'; tom = utled fra katalogen
}

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

	ForeignTaxPaid int              // 0/1/2
	ForeignTaxes   []ForeignTaxLine // betalt utenlandsk skatt, brutt ned per type

	ReceiptID *int64
}

// IncomeResult returnerer den lagrede inntekten og kursinfoen som ble brukt.
type IncomeResult struct {
	Income   db.Income
	RateUsed float64
	RateDate string
}

// resolvedTaxLine er en skattelinje med beregnet NOK-beløp, klar for lagring.
type resolvedTaxLine struct {
	taxType    string
	amountOrig float64
	currency   string
	amountNOK  float64
	treatment  string
}

// resolvedIncome er ferdig beregnede DB-verdier for en inntekt.
type resolvedIncome struct {
	exchangeRate sql.NullFloat64
	rateDate     sql.NullString
	amountNOK    float64
	taxes        []resolvedTaxLine
	usedRate     float64
	usedRateDate string
}

// resolveIncome henter valutakurs og beregner NOK-beløp + utenlandske skatter.
func (a *App) resolveIncome(ctx context.Context, in IncomeInput) (resolvedIncome, error) {
	res := resolvedIncome{amountNOK: in.AmountOrig, usedRate: 1.0, usedRateDate: in.Date}
	conv, err := a.convertToNOK(ctx, in.Currency, in.Date, in.AmountOrig, "currency")
	if err != nil {
		return res, err
	}
	res.amountNOK = conv.AmountNOK
	res.usedRate = conv.Rate
	res.usedRateDate = conv.RateDate
	res.exchangeRate = conv.ExchangeRate
	res.rateDate = conv.RateDateNull
	if in.ForeignTaxPaid == ForeignTaxYes {
		defaults := a.defaultTreatmentMap(ctx, in.CountryCode, in.TaxYear)
		for _, line := range in.ForeignTaxes {
			if line.AmountOrig <= 0 {
				continue
			}
			cur := line.Currency
			if cur == "" {
				cur = in.Currency
			}
			nok := line.AmountOrig
			switch {
			case cur == "NOK":
				// allerede NOK
			case cur == in.Currency && res.exchangeRate.Valid:
				nok = tax.Round2(line.AmountOrig * res.usedRate)
			default:
				r, err := a.Currency.Rate(ctx, cur, in.Date)
				if err != nil {
					ve := newValidation()
					ve.add("foreign_tax_orig", "kunne ikke hente kurs for utenlandsk skatt: "+err.Error())
					return res, ve
				}
				nok = tax.Round2(line.AmountOrig * r.RateNOK)
			}
			res.taxes = append(res.taxes, resolvedTaxLine{
				taxType: line.Type, amountOrig: line.AmountOrig, currency: cur, amountNOK: nok,
				treatment: resolveTreatment(line.Treatment, defaults, line.Type),
			})
		}
	}
	return res, nil
}

// resolveTreatment velger skattemessig behandling for en linje: brukerens
// eksplisitte valg hvis gyldig, ellers katalogens standard for typen. Ukjente
// typer (ikke i katalogen) antas å være inntektsskatt -> credit, så brukeren
// ikke taper kredit på en egendefinert kode.
func resolveTreatment(override string, defaults map[string]string, taxType string) string {
	if t := strings.ToLower(strings.TrimSpace(override)); isTreatment(t) {
		return t
	}
	if t, ok := defaults[strings.ToUpper(strings.TrimSpace(taxType))]; ok {
		return t
	}
	return TaxTreatmentCredit
}

// defaultTreatmentMap gir et oppslag fra skattetype-kode til standard
// behandling (credit/deduct/none) for et land og inntektsår. Bruker katalogens
// eksplisitte default_treatment hvis satt, ellers utledes det fra
// is_creditable_in_norway (1 -> credit, 0 -> deduct).
func (a *App) defaultTreatmentMap(ctx context.Context, country string, year int) map[string]string {
	rows, err := a.CountryTaxTypes(ctx, country, year)
	if err != nil {
		return nil
	}
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		m[strings.ToUpper(r.TaxTypeCode)] = catalogTreatment(r)
	}
	return m
}

// catalogTreatment gir standardbehandlingen for en katalogtype.
func catalogTreatment(r db.CountryTaxType) string {
	if t := strings.ToLower(strings.TrimSpace(r.DefaultTreatment.String)); isTreatment(t) {
		return t
	}
	if r.IsCreditableInNorway.Int64 == 1 {
		return TaxTreatmentCredit
	}
	return TaxTreatmentDeduct
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
	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	qtx := a.Q.WithTx(tx)
	created, err := qtx.CreateIncome(ctx, db.CreateIncomeParams{
		Date: in.Date, Description: in.Description, AmountOrig: in.AmountOrig,
		Currency: in.Currency, ExchangeRate: res.exchangeRate, RateDate: res.rateDate,
		AmountNok: res.amountNOK, Category: in.Category, Client: nullString(in.Client),
		CountryCode: in.CountryCode, ForeignTaxPaid: int64(in.ForeignTaxPaid),
		ReceiptID: nullInt(in.ReceiptID),
		TaxYear:   int64(in.TaxYear), Notes: nullString(in.Notes),
	})
	if err != nil {
		return nil, fmt.Errorf("lagre inntekt: %w", err)
	}
	if err := insertTaxLines(ctx, qtx, created.ID, res.taxes); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
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
	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	qtx := a.Q.WithTx(tx)
	updated, err := qtx.UpdateIncome(ctx, db.UpdateIncomeParams{
		ID: id, Date: in.Date, Description: in.Description, AmountOrig: in.AmountOrig,
		Currency: in.Currency, ExchangeRate: res.exchangeRate, RateDate: res.rateDate,
		AmountNok: res.amountNOK, Category: in.Category, Client: nullString(in.Client),
		CountryCode: in.CountryCode, ForeignTaxPaid: int64(in.ForeignTaxPaid),
		TaxYear: int64(in.TaxYear), Notes: nullString(in.Notes),
	})
	if err != nil {
		return nil, fmt.Errorf("oppdater inntekt: %w", err)
	}
	// Erstatt skattelinjene (slett + sett inn på nytt).
	if err := qtx.DeleteIncomeForeignTaxesByIncome(ctx, id); err != nil {
		return nil, fmt.Errorf("oppdater inntekt: %w", err)
	}
	if err := insertTaxLines(ctx, qtx, id, res.taxes); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
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

// IncomeForeignTaxes henter de utenlandske skattelinjene for en inntekt.
func (a *App) IncomeForeignTaxes(ctx context.Context, id int64) ([]db.IncomeForeignTax, error) {
	return a.Q.ListIncomeForeignTaxes(ctx, id)
}

// insertTaxLines lagrer skattelinjene for en inntekt via den gitte spørreren
// (typisk en transaksjon).
func insertTaxLines(ctx context.Context, q *db.Queries, incomeID int64, lines []resolvedTaxLine) error {
	for _, l := range lines {
		if _, err := q.CreateIncomeForeignTax(ctx, db.CreateIncomeForeignTaxParams{
			IncomeID: incomeID, TaxType: l.taxType, AmountOrig: l.amountOrig,
			Currency: l.currency, AmountNok: l.amountNOK, Treatment: l.treatment,
		}); err != nil {
			return fmt.Errorf("lagre skattelinje: %w", err)
		}
	}
	return nil
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
	// Normaliser og behold bare skattelinjer med en type og et beløp > 0.
	lines := in.ForeignTaxes[:0]
	for _, line := range in.ForeignTaxes {
		line.Type = strings.TrimSpace(line.Type)
		line.Currency = strings.ToUpper(strings.TrimSpace(line.Currency))
		if line.Type == "" || line.AmountOrig <= 0 {
			continue
		}
		lines = append(lines, line)
	}
	in.ForeignTaxes = lines
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
