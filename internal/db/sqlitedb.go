package db

import (
	"database/sql"
)

func NewSqliteDb(path string) *sql.DB {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		panic(err)
	}

	return db
}
