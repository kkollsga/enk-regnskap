-- name: CreateIncome :one
INSERT INTO income (
  date, description, amount_orig, currency, exchange_rate, rate_date,
  amount_nok, category, client, country_code,
  foreign_tax_paid, receipt_id, tax_year, notes
) VALUES (
  ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: GetIncome :one
SELECT * FROM income WHERE id = ?;

-- name: ListIncomeByYear :many
SELECT * FROM income WHERE tax_year = ? ORDER BY date DESC, id DESC;

-- name: ListIncomeByCountryYear :many
SELECT * FROM income WHERE country_code = ? AND tax_year = ? ORDER BY date;

-- name: ListAllIncome :many
SELECT * FROM income ORDER BY date DESC, id DESC;

-- name: UpdateIncomeReceipt :exec
UPDATE income SET receipt_id = ? WHERE id = ?;

-- name: DeleteIncome :exec
DELETE FROM income WHERE id = ?;

-- name: SumIncomeNOKByYear :one
SELECT COALESCE(SUM(amount_nok), 0) AS total FROM income WHERE tax_year = ?;

-- name: SumIncomeByCategory :many
SELECT category, COALESCE(SUM(amount_nok), 0) AS total
FROM income WHERE tax_year = ?
GROUP BY category ORDER BY total DESC;

-- name: DistinctClients :many
SELECT DISTINCT client FROM income
WHERE client IS NOT NULL AND client <> ''
ORDER BY client;

-- name: AggregateForeignIncomeByYear :many
-- Skattelinjer kollapses per inntekt i subspørringen slik at en inntekt med
-- flere skattetyper ikke dobbelttelles når amount_nok summeres per land.
SELECT i.country_code,
       COALESCE(SUM(i.amount_nok), 0) AS income_nok,
       COALESCE(SUM(t.tax_orig), 0) AS foreign_tax_orig,
       COALESCE(SUM(t.tax_nok), 0) AS foreign_tax_nok
FROM income i
LEFT JOIN (
  SELECT income_id,
         SUM(amount_orig) AS tax_orig,
         SUM(amount_nok) AS tax_nok
  FROM income_foreign_taxes
  WHERE creditable = 1
  GROUP BY income_id
) t ON t.income_id = i.id
WHERE i.tax_year = ? AND i.country_code <> 'NO'
GROUP BY i.country_code;

-- name: UpdateIncome :one
UPDATE income SET
  date = ?, description = ?, amount_orig = ?, currency = ?, exchange_rate = ?,
  rate_date = ?, amount_nok = ?, category = ?, client = ?, country_code = ?,
  foreign_tax_paid = ?, tax_year = ?, notes = ?
WHERE id = ?
RETURNING *;

-- name: CreateIncomeForeignTax :one
INSERT INTO income_foreign_taxes (
  income_id, tax_type, amount_orig, currency, amount_nok, creditable
) VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListIncomeForeignTaxes :many
SELECT * FROM income_foreign_taxes WHERE income_id = ? ORDER BY id;

-- name: ListAllIncomeForeignTaxes :many
SELECT * FROM income_foreign_taxes ORDER BY income_id, id;

-- name: DeleteIncomeForeignTaxesByIncome :exec
DELETE FROM income_foreign_taxes WHERE income_id = ?;
