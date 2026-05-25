-- name: GetConnections :many
SELECT * FROM Connections;

-- name: GetConnectionsInvolving :many
SELECT * FROM Connections WHERE source = ? OR target = ?;

-- name: CreateConnection :one
INSERT INTO Connections (
    id,
    source,
    target
) VALUES (
    ?, ?, ?
)
RETURNING *;
