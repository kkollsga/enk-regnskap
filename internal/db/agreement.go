package db

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
)

// Skatteavtaler og landspesifikke skattetyper lagres som JSON og lastes inn i
// country_tax_rules / country_tax_types ved seeding. Dette holder selve
// klassifiseringen (krediterbar / fradrag / kun referanse) deklarativ og samlet
// per avtale, i stedet for spredt i SQL.
//
//go:embed agreements/*.json
var agreementFS embed.FS

// taxAgreement er en lands skatteavtale med Norge + skattetypene den dekker.
type taxAgreement struct {
	CountryCode string             `json:"country_code"`
	CountryName string             `json:"country_name"`
	Agreement   string             `json:"agreement"`
	Rules       []agreementRule    `json:"rules"`
	TaxTypes    []agreementTaxType `json:"tax_types"`
}

type agreementRule struct {
	EffectiveFrom          int      `json:"effective_from"`
	EffectiveTo            *int     `json:"effective_to"`
	HasTaxTreaty           bool     `json:"has_tax_treaty"`
	TreatyInForceDate      *string  `json:"treaty_in_force_date"`
	TreatyMethod           *string  `json:"treaty_method"`
	TreatyReference        *string  `json:"treaty_reference"`
	TreatySourceURL        *string  `json:"treaty_source_url"`
	StandardWithholdingPct *float64 `json:"standard_withholding_pct"`
	Notes                  string   `json:"notes"`
}

type agreementTaxType struct {
	Code           string   `json:"code"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	AppliesTo      string   `json:"applies_to"`
	Creditable     bool     `json:"creditable"`
	Treatment      string   `json:"treatment"` // credit/deduct/none
	Basis          string   `json:"basis"`
	TypicalRatePct *float64 `json:"typical_rate_pct"`
	EffectiveFrom  int      `json:"effective_from"`
	EffectiveTo    *int     `json:"effective_to"`
}

// seedTaxAgreements laster alle JSON-avtaler i agreements/ inn i katalogen.
// Idempotent via ON CONFLICT DO UPDATE.
func seedTaxAgreements(conn *sql.DB) error {
	entries, err := agreementFS.ReadDir("agreements")
	if err != nil {
		return fmt.Errorf("les agreements/: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		raw, err := agreementFS.ReadFile("agreements/" + e.Name())
		if err != nil {
			return fmt.Errorf("les %s: %w", e.Name(), err)
		}
		var ag taxAgreement
		if err := json.Unmarshal(raw, &ag); err != nil {
			return fmt.Errorf("tolk %s: %w", e.Name(), err)
		}
		if err := upsertAgreement(conn, ag); err != nil {
			return fmt.Errorf("last %s: %w", e.Name(), err)
		}
	}
	return nil
}

func upsertAgreement(conn *sql.DB, ag taxAgreement) error {
	for _, r := range ag.Rules {
		treaty := int64(0)
		if r.HasTaxTreaty {
			treaty = 1
		}
		if _, err := conn.Exec(`INSERT INTO country_tax_rules
			(country_code, country_name, effective_from, effective_to, has_tax_treaty,
			 treaty_in_force_date, treaty_method, treaty_reference, treaty_source_url,
			 standard_withholding_pct, notes)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(country_code, effective_from) DO UPDATE SET
			  country_name = excluded.country_name,
			  effective_to = excluded.effective_to,
			  has_tax_treaty = excluded.has_tax_treaty,
			  treaty_in_force_date = excluded.treaty_in_force_date,
			  treaty_method = excluded.treaty_method,
			  treaty_reference = excluded.treaty_reference,
			  treaty_source_url = excluded.treaty_source_url,
			  standard_withholding_pct = excluded.standard_withholding_pct,
			  notes = excluded.notes`,
			ag.CountryCode, ag.CountryName, r.EffectiveFrom, intPtrArg(r.EffectiveTo), treaty,
			strPtrArg(r.TreatyInForceDate), strPtrArg(r.TreatyMethod), strPtrArg(r.TreatyReference),
			strPtrArg(r.TreatySourceURL), floatPtrArg(r.StandardWithholdingPct), r.Notes); err != nil {
			return err
		}
	}
	for _, t := range ag.TaxTypes {
		creditable := int64(0)
		if t.Creditable {
			creditable = 1
		}
		if _, err := conn.Exec(`INSERT INTO country_tax_types
			(country_code, tax_type_code, tax_type_name, description, applies_to,
			 is_creditable_in_norway, default_treatment, basis, typical_rate_pct,
			 effective_from, effective_to)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(country_code, tax_type_code, effective_from) DO UPDATE SET
			  tax_type_name = excluded.tax_type_name,
			  description = excluded.description,
			  applies_to = excluded.applies_to,
			  is_creditable_in_norway = excluded.is_creditable_in_norway,
			  default_treatment = excluded.default_treatment,
			  basis = excluded.basis,
			  typical_rate_pct = excluded.typical_rate_pct,
			  effective_to = excluded.effective_to`,
			ag.CountryCode, t.Code, t.Name, t.Description, t.AppliesTo,
			creditable, t.Treatment, t.Basis, floatPtrArg(t.TypicalRatePct),
			t.EffectiveFrom, intPtrArg(t.EffectiveTo)); err != nil {
			return err
		}
	}
	return nil
}

func intPtrArg(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}
func strPtrArg(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}
func floatPtrArg(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}
