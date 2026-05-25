-- name: GetGroups :many
SELECT * FROM Rbx_Groups;

-- name: GetGroupById :one
SELECT * FROM Rbx_Groups WHERE Id = ?;

-- name: CreateGroup :one
INSERT INTO Rbx_Groups (Id, Name, Members)
VALUES (?, ?, ?)
RETURNING *;

-- name: UpdateGroupMemberCount :one
UPDATE Rbx_Groups
SET Members = ?
WHERE Id = ?
RETURNING *;
