package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Maxxxxxx-x/rotector-slop/config"
	"github.com/Maxxxxxx-x/rotector-slop/db/sqlc"
	_ "modernc.org/sqlite"
)

func TestConnection(db *sql.DB) error {
	return db.Ping()
}

type DB struct {
	*sqlc.Queries
	rawDb *sql.DB
}

func ConnectDB(cfg config.Database) (*DB, error) {
	connStr := fmt.Sprintf("file:%s.sqlite", cfg.Name)
	rawDb, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, err
	}

	rawDb.SetMaxOpenConns(cfg.MaxOpenConn)
	rawDb.SetMaxIdleConns(cfg.MaxIdleConn)

	if _, err := rawDb.ExecContext(context.Background(), "PRAGMA foreign_keys = ON;"); err != nil {
		rawDb.Close()
		return nil, fmt.Errorf("Failed to enable foreign keys: %w", err)
	}

	queries := sqlc.New(rawDb)

	return &DB{
		Queries: queries,
		rawDb:   rawDb,
	}, nil
}

func (db *DB) Close() error {
	return db.rawDb.Close()
}
