-- name: CreateIncome :one
INSERT INTO income (
  date, description, amount_orig, currency, exchange_rate, rate_date,
  amount_nok, category, client, country_code,
  foreign_tax_paid, foreign_tax_orig, foreign_tax_currency, foreign_tax_nok,
  foreign_tax_type, receipt_id, tax_year, notes
) VALUES (
  ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
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
SELECT country_code,
       COALESCE(SUM(amount_nok), 0) AS income_nok,
       COALESCE(SUM(foreign_tax_orig), 0) AS foreign_tax_orig,
       COALESCE(SUM(foreign_tax_nok), 0) AS foreign_tax_nok
FROM income
WHERE tax_year = ? AND country_code <> 'NO'
GROUP BY country_code;
