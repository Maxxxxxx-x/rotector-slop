-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS Flags(
    id TEXT PRIMARY KEY,
    flag_type TEXT NOT NULL,
    category TEXT,
    confidence REAL,
    reviewer TEXT,
    is_reportable INTEGER DEFAULT 0,
    is_locked INTEGER DEFAULT 0,
    queued_at TEXT,
    processed_at TEXT,
    last_updated TEXT,
    reasons TEXT DEFAULT '{}',
    created_at TEXT NULL DEFAULT(datetime('now'))
);
CREATE INDEX idx_flag_type
ON Flags(flag_type);
CREATE INDEX idx_category
ON Flags(category);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE Flags;
-- +goose StatementEnd
