-- name: GetExchangeRate :one
SELECT * FROM exchange_rates WHERE currency = ? AND date = ?;

-- name: GetNearestExchangeRate :one
SELECT * FROM exchange_rates
WHERE currency = ? AND date <= ?
ORDER BY date DESC
LIMIT 1;

-- name: UpsertExchangeRate :exec
INSERT INTO exchange_rates (currency, date, rate_nok, source)
VALUES (?, ?, ?, ?)
ON CONFLICT(currency, date) DO UPDATE SET rate_nok = excluded.rate_nok, source = excluded.source;

-- name: ListExchangeRates :many
SELECT * FROM exchange_rates WHERE currency = ? ORDER BY date DESC;
