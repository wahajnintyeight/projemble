package storage

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

func Open(databaseURL string) (*sql.DB, error) {
	if databaseURL == "" {
		databaseURL = "projemble.db"
	}
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
