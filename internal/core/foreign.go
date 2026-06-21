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
// inntektsaar og oppdaterer foreign_tax_credits. Brukerinnskrevne felter
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

// legalBasis avgjor rettsgrunnlaget for kreditfradrag for et land og aar.
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

// ForeignTaxOverview er en beriket visning av kreditfradrag for et aar.
type ForeignTaxOverview struct {
	Credit        db.ForeignTaxCredit
	MaxCreditEst  float64 // estimert maksimalt kreditfradrag (§ 16-21)
	DocsMissing   bool
	LegalBasisRef string
}

// ForeignTaxForYear henter alle kreditfradrag for et aar, beriket med estimat.
func (a *App) ForeignTaxForYear(ctx context.Context, year int) ([]ForeignTaxOverview, error) {
	rows, err := a.Q.ListForeignTaxCreditsByYear(ctx, int64(year))
	if err != nil {
		return nil, err
	}
	out := make([]ForeignTaxOverview, 0, len(rows))
	for _, c := range rows {
		ov := ForeignTaxOverview{Credit: c}
		// Estimert tak: norsk skatt som forholdsmessig faller paa inntekten.
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

// CountryOption er et land i en nedtrekksmeny.
type CountryOption struct {
	Code string
	Name string
}

// Countries returnerer registrerte land (for nedtrekksmenyer). Norge forst.
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
