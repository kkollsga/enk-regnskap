package core

import (
	"context"

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

// TaxRulesFor returnerer skattereglene for et inntektsaar.
func (a *App) TaxRulesFor(year int) (tax.Rules, error) {
	return tax.Load(year)
}

// AvailableTaxYears returnerer aar det finnes skatteregler for.
func (a *App) AvailableTaxYears() []int {
	return tax.AvailableYears()
}
