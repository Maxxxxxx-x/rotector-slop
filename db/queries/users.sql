-- name: GetUsers :many
SELECT * FROM Users;

-- name: GetUserById :one
SELECT * FROM Users WHERE Id = ?;

-- name: GetUsersByGroupId :many
SELECT * FROM Users WHERE GroupId = ?;

-- name: CreateUser :one
INSERT INTO Users (Id, Name, GroupId, Role)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: UpdateUserName :exec
UPDATE Users
SET Name = ?
WHERE Id = ?;

-- name: FullUpdateUser :exec
UPDATE users
SET
Name = ?,
GroupId = ?,
Role = ?
WHERE Id = ?;

