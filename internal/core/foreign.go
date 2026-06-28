package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/kkollsga/enk-regnskap/internal/db"
	"github.com/kkollsga/enk-regnskap/internal/tax"
)

// LegalBasis for kreditfradrag.
const (
	LegalBasisTreaty   = "treaty"   // skatteavtale i kraft (2025+ for Brasil)
	LegalBasisInternal = "internal" // intern norsk rett (sktl. § 16-20 flg.)
)

// RecomputeForeignTaxCredits aggregerer all utenlandsinntekt per land for et
// inntektsår og oppdaterer foreign_tax_credits. Brukerinnskrevne felter
// (dokumentasjon, status, notater) bevares. Dette er en avledet systemhandling
// og logges ikke i endringsloggen.
func (a *App) RecomputeForeignTaxCredits(ctx context.Context, year int) error {
	aggs, err := a.Q.AggregateForeignIncomeByYear(ctx, int64(year))
	if err != nil {
		return fmt.Errorf("aggreger utenlandsinntekt: %w", err)
	}
	for _, agg := range aggs {
		incomeNOK := toFloat(agg.IncomeNok)
		ftOrig := toFloat(agg.ForeignTaxOrig)
		ftNOK := toFloat(agg.ForeignTaxNok)

		basis := a.legalBasis(ctx, agg.CountryCode, year)
		countryName := a.countryName(ctx, agg.CountryCode)
		foreignCurrency := countryCurrency(agg.CountryCode)

		// Bevar eksisterende brukerfelter hvis raden finnes.
		params := db.UpsertForeignTaxCreditParams{
			TaxYear:         int64(year),
			CountryCode:     agg.CountryCode,
			CountryName:     countryName,
			IncomeNok:       incomeNOK,
			ForeignTaxOrig:  ftOrig,
			ForeignCurrency: foreignCurrency,
			ForeignTaxNok:   ftNOK,
			LegalBasis:      sql.NullString{String: basis, Valid: true},
			CarryforwardNok: sql.NullFloat64{Float64: 0, Valid: true},
		}
		if existing, err := a.Q.GetForeignTaxCredit(ctx, db.GetForeignTaxCreditParams{
			TaxYear: int64(year), CountryCode: agg.CountryCode,
		}); err == nil {
			params.MaxCreditNok = existing.MaxCreditNok
			params.UtilizedNok = existing.UtilizedNok
			params.CarryforwardNok = existing.CarryforwardNok
			params.TaxFinalizedAbroad = existing.TaxFinalizedAbroad
			params.DocumentationType = existing.DocumentationType
			params.Rf1147Ready = existing.Rf1147Ready
			params.Notes = existing.Notes
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		if _, err := a.Q.UpsertForeignTaxCredit(ctx, params); err != nil {
			return fmt.Errorf("oppdater kreditfradrag for %s: %w", agg.CountryCode, err)
		}
	}
	return nil
}

// legalBasis avgjør rettsgrunnlaget for kreditfradrag for et land og år.
func (a *App) legalBasis(ctx context.Context, country string, year int) string {
	rule, err := a.Q.GetCountryRule(ctx, db.GetCountryRuleParams{
		CountryCode:   country,
		EffectiveFrom: int64(year),
		EffectiveTo:   sql.NullInt64{Int64: int64(year), Valid: true},
	})
	if err == nil && rule.HasTaxTreaty == 1 {
		return LegalBasisTreaty
	}
	return LegalBasisInternal
}

func (a *App) countryName(ctx context.Context, country string) string {
	rule, err := a.Q.GetCountryRule(ctx, db.GetCountryRuleParams{
		CountryCode:   country,
		EffectiveFrom: 9999,
		EffectiveTo:   sql.NullInt64{},
	})
	if err == nil && rule.CountryName != "" {
		return rule.CountryName
	}
	return country
}

// ForeignTaxOverview er en beriket visning av kreditfradrag for et år.
type ForeignTaxOverview struct {
	Credit        db.ForeignTaxCredit
	MaxCreditEst  float64 // estimert maksimalt kreditfradrag (§ 16-21)
	DocsMissing   bool
	LegalBasisRef string
}

// ForeignTaxForYear henter alle kreditfradrag for et år, beriket med estimat.
func (a *App) ForeignTaxForYear(ctx context.Context, year int) ([]ForeignTaxOverview, error) {
	rows, err := a.Q.ListForeignTaxCreditsByYear(ctx, int64(year))
	if err != nil {
		return nil, err
	}
	out := make([]ForeignTaxOverview, 0, len(rows))
	for _, c := range rows {
		ov := ForeignTaxOverview{Credit: c}
		// Estimert tak: norsk skatt som forholdsmessig faller på inntekten.
		// Bruker alminnelig sats som konservativt anslag (sktl. § 16-21).
		if rules, err := tax.Load(year); err == nil {
			ov.MaxCreditEst = tax.Round2(c.IncomeNok * rules.AlminneligInntektsskattPct / 100.0)
		}
		ov.DocsMissing = !c.DocumentationType.Valid || c.DocumentationType.String == ""
		if c.LegalBasis.String == LegalBasisTreaty {
			ov.LegalBasisRef = "Skatteavtale Norge-" + c.CountryName + " (kreditmetoden)"
		} else {
			ov.LegalBasisRef = "Intern norsk rett, sktl. § 16-20 flg."
		}
		out = append(out, ov)
	}
	return out, nil
}

// EstimatedForeignTaxCredit anslår samlet kreditfradrag for et år: betalt
// krediterbar utenlandsk skatt per land, begrenset til det estimerte taket
// (§ 16-21). Konservativt anslag – ikke et endelig kreditfradrag.
func (a *App) EstimatedForeignTaxCredit(ctx context.Context, year int) (float64, error) {
	ovs, err := a.ForeignTaxForYear(ctx, year)
	if err != nil {
		return 0, err
	}
	var total float64
	for _, ov := range ovs {
		paid := ov.Credit.ForeignTaxNok
		if paid > ov.MaxCreditEst {
			paid = ov.MaxCreditEst
		}
		total += paid
	}
	return tax.Round2(total), nil
}

// CountryTaxTypes henter skattetypene for et land og inntektsår (sjekkliste).
func (a *App) CountryTaxTypes(ctx context.Context, country string, year int) ([]db.CountryTaxType, error) {
	return a.Q.ListCountryTaxTypes(ctx, db.ListCountryTaxTypesParams{
		CountryCode:   country,
		EffectiveFrom: int64(year),
		EffectiveTo:   sql.NullInt64{Int64: int64(year), Valid: true},
	})
}

// ForeignTaxTotals er årets utenlandske skatt summert per behandling (NOK).
type ForeignTaxTotals struct {
	Credit float64 // kreditfradrag
	Deduct float64 // fradragsberettiget kostnad
	None   float64 // ingen lettelse (kun referanse)
}

// ForeignTaxTotalsForYear summerer utenlandsk skatt per skattemessig behandling
// for et inntektsår.
func (a *App) ForeignTaxTotalsForYear(ctx context.Context, year int) (ForeignTaxTotals, error) {
	rows, err := a.Q.SumForeignTaxByTreatmentYear(ctx, int64(year))
	if err != nil {
		return ForeignTaxTotals{}, err
	}
	var t ForeignTaxTotals
	for _, r := range rows {
		switch r.Treatment {
		case TaxTreatmentCredit:
			t.Credit = toFloat(r.Total)
		case TaxTreatmentDeduct:
			t.Deduct = toFloat(r.Total)
		case TaxTreatmentNone:
			t.None = toFloat(r.Total)
		}
	}
	return t, nil
}

// LinkedForeignTax er en utenlandsk skattelinje vist som en koblet (skrivebeskyttet)
// post i utgiftslisten, med peker tilbake til inntekten den hører til.
type LinkedForeignTax struct {
	IncomeID          int64
	IncomeDate        string
	IncomeDescription string
	TaxType           string
	AmountOrig        float64
	Currency          string
	AmountNok         float64
}

// DeductibleForeignTaxLines henter utenlandske skattelinjer som behandles som
// fradragsberettiget kostnad for et år, til visning i utgiftslisten.
func (a *App) DeductibleForeignTaxLines(ctx context.Context, year int) ([]LinkedForeignTax, error) {
	rows, err := a.Q.ListForeignTaxLinesByYearTreatment(ctx, db.ListForeignTaxLinesByYearTreatmentParams{
		TaxYear: int64(year), Treatment: TaxTreatmentDeduct,
	})
	if err != nil {
		return nil, err
	}
	out := make([]LinkedForeignTax, 0, len(rows))
	for _, r := range rows {
		out = append(out, LinkedForeignTax{
			IncomeID: r.IncomeID, IncomeDate: r.IncomeDate, IncomeDescription: r.IncomeDescription,
			TaxType: r.TaxType, AmountOrig: r.AmountOrig, Currency: r.Currency, AmountNok: r.AmountNok,
		})
	}
	return out, nil
}

// TaxTypeOption er et forslag i skattetype-comboboksen på inntektsskjemaet.
type TaxTypeOption struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Desc       string `json:"desc"`
	Creditable bool   `json:"creditable"`
	Treatment  string `json:"treatment"` // standardbehandling: credit/deduct/none
}

// TaxTypeSuggestions returnerer skattetype-forslag gruppert per kildeland for et
// gitt inntektsår, til bruk i inntektsskjemaets combobox. Året avgjør
// krediterbarheten (f.eks. er CSLL krediterbar fra 2025).
func (a *App) TaxTypeSuggestions(ctx context.Context, year int) (map[string][]TaxTypeOption, error) {
	countries, err := a.Countries(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string][]TaxTypeOption{}
	for _, c := range countries {
		if c.Code == "NO" {
			continue
		}
		types, err := a.CountryTaxTypes(ctx, c.Code, year)
		if err != nil {
			continue
		}
		for _, r := range types {
			out[c.Code] = append(out[c.Code], TaxTypeOption{
				Code: r.TaxTypeCode, Name: r.TaxTypeName, Desc: nsVal(r.Description),
				Creditable: r.IsCreditableInNorway.Int64 == 1,
				Treatment:  catalogTreatment(r),
			})
		}
	}
	return out, nil
}

// ForeignTaxStatusInput er brukerens oppdatering av dokumentasjonsstatus.
type ForeignTaxStatusInput struct {
	Year              int
	Country           string
	DocumentationType string
	TaxFinalized      bool
	RF1147Ready       bool
	Notes             string
}

// UpdateForeignTaxStatus oppdaterer brukerstyrte felter på et kreditfradrag
// (dokumentasjon, status, notater) uten å røre de aggregerte tallene.
// Endringen revisjonslogges.
func (a *App) UpdateForeignTaxStatus(ctx context.Context, actor string, in ForeignTaxStatusInput) error {
	existing, err := a.Q.GetForeignTaxCredit(ctx, db.GetForeignTaxCreditParams{
		TaxYear: int64(in.Year), CountryCode: in.Country,
	})
	if err != nil {
		return fmt.Errorf("fant ikke kreditfradrag for %s %d: %w", in.Country, in.Year, err)
	}
	before, _ := a.snapshotRow(ctx, "foreign_tax_credits", existing.ID)

	finalized := int64(0)
	if in.TaxFinalized {
		finalized = 1
	}
	rf := int64(0)
	if in.RF1147Ready {
		rf = 1
	}
	_, err = a.Q.UpsertForeignTaxCredit(ctx, db.UpsertForeignTaxCreditParams{
		TaxYear:            existing.TaxYear,
		CountryCode:        existing.CountryCode,
		CountryName:        existing.CountryName,
		IncomeNok:          existing.IncomeNok,
		ForeignTaxOrig:     existing.ForeignTaxOrig,
		ForeignCurrency:    existing.ForeignCurrency,
		ForeignTaxNok:      existing.ForeignTaxNok,
		MaxCreditNok:       existing.MaxCreditNok,
		UtilizedNok:        existing.UtilizedNok,
		CarryforwardNok:    existing.CarryforwardNok,
		TaxFinalizedAbroad: sql.NullInt64{Int64: finalized, Valid: true},
		DocumentationType:  nullString(in.DocumentationType),
		LegalBasis:         existing.LegalBasis,
		Rf1147Ready:        sql.NullInt64{Int64: rf, Valid: true},
		Notes:              nullString(in.Notes),
	})
	if err != nil {
		return fmt.Errorf("oppdater status: %w", err)
	}
	after, _ := a.snapshotRow(ctx, "foreign_tax_credits", existing.ID)
	return a.logChange(ctx, actor, "update", "foreign_tax_credits", existing.ID, before, after, in.Year,
		fmt.Sprintf("Oppdaterte dokumentasjonsstatus for %s %d", in.Country, in.Year))
}

// CountryOption er et land i en nedtrekksmeny.
type CountryOption struct {
	Code string
	Name string
}

// Countries returnerer registrerte land (for nedtrekksmenyer). Norge først.
func (a *App) Countries(ctx context.Context) ([]CountryOption, error) {
	rows, err := a.Q.ListCountryCodes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]CountryOption, 0, len(rows))
	var no *CountryOption
	for _, r := range rows {
		opt := CountryOption{Code: r.CountryCode, Name: r.CountryName}
		if r.CountryCode == "NO" {
			c := opt
			no = &c
			continue
		}
		out = append(out, opt)
	}
	if no != nil {
		out = append([]CountryOption{*no}, out...)
	}
	return out, nil
}

// countryCurrency gir standard valuta for et land.
func countryCurrency(code string) string {
	switch code {
	case "BR":
		return "BRL"
	case "NO":
		return "NOK"
	default:
		return code
	}
}
