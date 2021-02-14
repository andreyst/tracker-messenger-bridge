package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
)

var migrations = []string{
	// 1
	`
	CREATE TABLE webhooks_data(
		created_at TEXT DEFAULT '' NOT NULL,
		updated_at TEXT DEFAULT '' NOT NULL,
		path TEXT DEFAULT '' NOT NULL,
		headers TEXT DEFAULT '' NOT NULL,
		body TEXT DEFAULT '' NOT NULL,
		visible_at TEXT DEFAULT '' NOT NULL
	);
	`,
}

func applyMigrations(db *sql.DB) {
	var userVersion int
	row := db.QueryRow("PRAGMA user_version")

	if row == nil {
		checkErr(errors.New("storage: error: cannot read user_version"))
	}

	err := row.Scan(&userVersion)
	checkErr(err)

	log.Printf("storage: current migration ID: %v\n", userVersion)
	log.Printf("storage: unapplied migrations: %d\n", len(migrations)-userVersion)

	tx, err := db.Begin()
	checkErr(err)

	migrationID := 1
	for _, migration := range migrations {
		if userVersion < migrationID {
			_, err = db.Exec(migration)
			checkErr(err)
		}
		migrationID++
	}

	_, err = db.Exec(fmt.Sprintf("PRAGMA user_version = %d", len(migrations)))

	err = tx.Commit()
	checkErr(err)
}
