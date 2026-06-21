-- name: CreateReceipt :one
INSERT INTO receipts (filename, original_name, mime_type, tax_year)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetReceipt :one
SELECT * FROM receipts WHERE id = ?;

-- name: ListReceipts :many
SELECT * FROM receipts ORDER BY uploaded_at DESC, id DESC;

-- name: DeleteReceipt :exec
DELETE FROM receipts WHERE id = ?;

-- name: ListUnlinkedReceipts :many
SELECT * FROM receipts
WHERE id NOT IN (SELECT receipt_id FROM income WHERE receipt_id IS NOT NULL)
  AND id NOT IN (SELECT receipt_id FROM expenses WHERE receipt_id IS NOT NULL)
ORDER BY uploaded_at DESC, id DESC;

-- name: CountUnlinkedReceipts :one
SELECT COUNT(*) AS cnt FROM receipts
WHERE id NOT IN (SELECT receipt_id FROM income WHERE receipt_id IS NOT NULL)
  AND id NOT IN (SELECT receipt_id FROM expenses WHERE receipt_id IS NOT NULL);
