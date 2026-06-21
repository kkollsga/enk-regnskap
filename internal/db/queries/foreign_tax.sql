-- name: UpsertForeignTaxCredit :one
INSERT INTO foreign_tax_credits (
  tax_year, country_code, country_name, income_nok, foreign_tax_orig,
  foreign_currency, foreign_tax_nok, max_credit_nok, utilized_nok,
  carryforward_nok, tax_finalized_abroad, documentation_type, legal_basis,
  rf1147_ready, notes
) VALUES (
  ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
ON CONFLICT(tax_year, country_code) DO UPDATE SET
  country_name = excluded.country_name,
  income_nok = excluded.income_nok,
  foreign_tax_orig = excluded.foreign_tax_orig,
  foreign_currency = excluded.foreign_currency,
  foreign_tax_nok = excluded.foreign_tax_nok,
  max_credit_nok = excluded.max_credit_nok,
  utilized_nok = excluded.utilized_nok,
  carryforward_nok = excluded.carryforward_nok,
  tax_finalized_abroad = excluded.tax_finalized_abroad,
  documentation_type = excluded.documentation_type,
  legal_basis = excluded.legal_basis,
  rf1147_ready = excluded.rf1147_ready,
  notes = excluded.notes
RETURNING *;

-- name: GetForeignTaxCredit :one
SELECT * FROM foreign_tax_credits WHERE tax_year = ? AND country_code = ?;

-- name: ListForeignTaxCreditsByYear :many
SELECT * FROM foreign_tax_credits WHERE tax_year = ? ORDER BY country_name;

-- name: ListAllForeignTaxCredits :many
SELECT * FROM foreign_tax_credits ORDER BY tax_year DESC, country_name;

-- name: DeleteForeignTaxCredit :exec
DELETE FROM foreign_tax_credits WHERE id = ?;
