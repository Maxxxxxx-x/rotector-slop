-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS Connections(
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL DEFAULT '',
    target TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_connection_src
ON Connections(source);
CREATE INDEX idx_connection_tgt
ON Connections(target);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE Connections;
-- +goose StatementEnd
