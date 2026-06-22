-- name: CreateReceipt :one
INSERT INTO receipts (filename, original_name, mime_type, title, description, parent_kind, parent_id, tax_year)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetReceipt :one
SELECT * FROM receipts WHERE id = ?;

-- name: ListReceipts :many
SELECT * FROM receipts ORDER BY uploaded_at DESC, id DESC;

-- name: ListReceiptsByParent :many
SELECT * FROM receipts WHERE parent_kind = ? AND parent_id = ? ORDER BY id;

-- name: UpdateReceiptMeta :exec
UPDATE receipts SET title = ?, description = ? WHERE id = ?;

-- name: UpdateReceiptParentLink :exec
UPDATE receipts SET parent_kind = ?, parent_id = ? WHERE id = ?;

-- name: DeleteReceipt :exec
DELETE FROM receipts WHERE id = ?;

-- name: CountReceiptsByParent :one
SELECT COUNT(*) AS cnt FROM receipts WHERE parent_kind = ? AND parent_id = ?;
