package core

import (
	"context"
	"fmt"
	"time"

	"github.com/kkollsga/enk-regnskap/internal/db"
	"github.com/kkollsga/enk-regnskap/internal/tax"
)

// CountryInfo samler skatteregler og skattetyper for ett land (alle perioder).
type CountryInfo struct {
	Code  string
	Name  string
	Rules []db.CountryTaxRule
	Types []db.CountryTaxType
}

// CountryOverview grupperer alle landregler og skattetyper per land.
func (a *App) CountryOverview(ctx context.Context) ([]CountryInfo, error) {
	rules, err := a.Q.ListCountryRules(ctx)
	if err != nil {
		return nil, err
	}
	types, err := a.Q.ListAllCountryTaxTypes(ctx)
	if err != nil {
		return nil, err
	}
	order := []string{}
	byCode := map[string]*CountryInfo{}
	get := func(code, name string) *CountryInfo {
		if ci, ok := byCode[code]; ok {
			return ci
		}
		ci := &CountryInfo{Code: code, Name: name}
		byCode[code] = ci
		order = append(order, code)
		return ci
	}
	for _, r := range rules {
		ci := get(r.CountryCode, r.CountryName)
		ci.Rules = append(ci.Rules, r)
	}
	for _, t := range types {
		ci := get(t.CountryCode, t.CountryCode)
		ci.Types = append(ci.Types, t)
	}
	out := make([]CountryInfo, 0, len(order))
	for _, code := range order {
		out = append(out, *byCode[code])
	}
	return out, nil
}

// CountryTaxSummary er en lesbar oppsummering av skatteavtale og skattetyper
// for ETT utland (ikke Norge), klar for visning.
type CountryTaxSummary struct {
	Code            string
	Name            string
	HasTreaty       bool
	TreatyInForce   string // f.eks. "30. desember 2024"
	TreatyFromYear  int64  // gjelder fra inntektsår
	TreatyMethodNO  string // "kreditfradrag (kreditmetoden)"
	TreatyRef       string
	TreatyURL       string
	PreTreatyToYear int64 // siste år før avtalen (intern rett gjelder t.o.m.)
	Creditable      []db.CountryTaxType
	NonCreditable   []db.CountryTaxType
}

// ForeignCountrySummaries gir lesbare oppsummeringer for alle utland (ikke NO).
func (a *App) ForeignCountrySummaries(ctx context.Context) ([]CountryTaxSummary, error) {
	infos, err := a.CountryOverview(ctx)
	if err != nil {
		return nil, err
	}
	var out []CountryTaxSummary
	for _, ci := range infos {
		if ci.Code == "NO" {
			continue // Norge er hjemstaten – ikke et avtaleland
		}
		s := CountryTaxSummary{Code: ci.Code, Name: ci.Name}
		for _, rule := range ci.Rules {
			if rule.HasTaxTreaty == 1 {
				s.HasTreaty = true
				s.TreatyFromYear = rule.EffectiveFrom
				if rule.TreatyInForceDate.Valid {
					s.TreatyInForce = formatNorwegianDate(rule.TreatyInForceDate.String)
				}
				s.TreatyMethodNO = treatyMethodNO(rule.TreatyMethod.String)
				s.TreatyRef = rule.TreatyReference.String
				s.TreatyURL = rule.TreatySourceUrl.String
			} else if rule.EffectiveTo.Valid {
				s.PreTreatyToYear = rule.EffectiveTo.Int64
			}
		}
		for _, t := range ci.Types {
			if t.IsCreditableInNorway.Valid && t.IsCreditableInNorway.Int64 == 1 {
				s.Creditable = append(s.Creditable, t)
			} else {
				s.NonCreditable = append(s.NonCreditable, t)
			}
		}
		out = append(out, s)
	}
	return out, nil
}

func treatyMethodNO(method string) string {
	switch method {
	case "credit":
		return "kreditfradrag (kreditmetoden)"
	case "exemption":
		return "unntak (unntaksmetoden)"
	default:
		return method
	}
}

var nbMonths = []string{"", "januar", "februar", "mars", "april", "mai", "juni",
	"juli", "august", "september", "oktober", "november", "desember"}

// formatNorwegianDate gjør "2024-12-30" om til "30. desember 2024".
func formatNorwegianDate(iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	return fmt.Sprintf("%d. %s %d", t.Day(), nbMonths[int(t.Month())], t.Year())
}

// TaxRulesFor returnerer skattereglene for et inntektsår.
func (a *App) TaxRulesFor(year int) (tax.Rules, error) {
	return tax.Load(year)
}

// AvailableTaxYears returnerer år det finnes skatteregler for.
func (a *App) AvailableTaxYears() []int {
	return tax.AvailableYears()
}
