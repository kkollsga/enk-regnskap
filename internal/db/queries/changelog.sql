-- name: CreateChangeLog :one
INSERT INTO change_log (
  actor, operation, entity, entity_id, before_json, after_json, description, rollback_of
) VALUES (
  ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: GetChangeLog :one
SELECT * FROM change_log WHERE id = ?;

-- name: ListChangeLog :many
SELECT * FROM change_log ORDER BY id DESC LIMIT ?;

-- name: MarkChangeRolledBack :exec
UPDATE change_log SET rolled_back = 1 WHERE id = ?;
