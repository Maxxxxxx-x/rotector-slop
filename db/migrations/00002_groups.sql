-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS Rbx_Groups(
    Id TEXT PRIMARY KEY,
    Name TEXT NOT NULL,
    Members INT NOT NULL DEFAULT 0
);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE Rbx_Groups;
-- +goose StatementEnd
