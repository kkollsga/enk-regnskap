-- name: CreateExpense :one
INSERT INTO expenses (
  date, description, amount_nok, category, deductible_pct, deductible_nok,
  receipt_id, tax_year, notes
) VALUES (
  ?, ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: GetExpense :one
SELECT * FROM expenses WHERE id = ?;

-- name: ListExpensesByYear :many
SELECT * FROM expenses WHERE tax_year = ? ORDER BY date DESC, id DESC;

-- name: ListAllExpenses :many
SELECT * FROM expenses ORDER BY date DESC, id DESC;

-- name: UpdateExpenseReceipt :exec
UPDATE expenses SET receipt_id = ? WHERE id = ?;

-- name: DeleteExpense :exec
DELETE FROM expenses WHERE id = ?;

-- name: SumDeductibleByYear :one
SELECT COALESCE(SUM(deductible_nok), 0) AS total FROM expenses WHERE tax_year = ?;

-- name: SumExpensesByCategory :many
SELECT category,
       COALESCE(SUM(amount_nok), 0) AS total,
       COALESCE(SUM(deductible_nok), 0) AS deductible
FROM expenses WHERE tax_year = ?
GROUP BY category ORDER BY deductible DESC;

-- name: UpdateExpense :one
UPDATE expenses SET
  date = ?, description = ?, amount_nok = ?, category = ?, deductible_pct = ?,
  deductible_nok = ?, tax_year = ?, notes = ?
WHERE id = ?
RETURNING *;
