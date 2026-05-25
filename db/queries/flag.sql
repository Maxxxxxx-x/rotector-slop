-- name: GetFlags :many
SELECT * FROM Flags;

-- name: GetFlagByID :one
SELECT * FROM Flags WHERE id = ?;

-- name: CreateFlag :one
INSERT INTO Flags (
    id,
    flag_type,
    category,
    confidence,
    reviewer,
    is_reportable,
    is_locked,
    queued_at,
    processed_at,
    last_updated,
    reasons
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;
