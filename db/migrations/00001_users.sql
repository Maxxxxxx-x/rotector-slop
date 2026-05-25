-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS Users(
    Id TEXT PRIMARY KEY,
    Name TEXT,
    GroupId TEXT DEFAULT "",
    Role TEXT
);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE Users;
-- +goose StatementEnd
