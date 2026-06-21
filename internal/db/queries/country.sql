-- name: GetCountryRule :one
SELECT * FROM country_tax_rules
WHERE country_code = ?
  AND effective_from <= ?
  AND (effective_to IS NULL OR effective_to >= ?)
ORDER BY effective_from DESC
LIMIT 1;

-- name: ListCountryRules :many
SELECT * FROM country_tax_rules
ORDER BY country_name, effective_from;

-- name: ListCountryTaxTypes :many
SELECT * FROM country_tax_types
WHERE country_code = ?
  AND effective_from <= ?
  AND (effective_to IS NULL OR effective_to >= ?)
ORDER BY tax_type_code;

-- name: ListAllCountryTaxTypes :many
SELECT * FROM country_tax_types
ORDER BY country_code, tax_type_code;

-- name: ListCountryCodes :many
SELECT DISTINCT country_code, country_name FROM country_tax_rules
ORDER BY country_name;
