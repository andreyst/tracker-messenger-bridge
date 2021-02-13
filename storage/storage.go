package storage

import (
	"database/sql"

	// Import and register sqlite3
	_ "github.com/mattn/go-sqlite3"
)

// Storage - storage for storing incoming webhooks data
type Storage struct {
	DB *sql.DB
}

// NewStorage - creates a new instance of storage
func NewStorage(path string) *Storage {
	s := &Storage{}

	db, err := sql.Open("sqlite3", path)
	checkErr(err)

	applyMigrations(db)

	s.DB = db
	return s
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
