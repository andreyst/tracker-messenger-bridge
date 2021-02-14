package storage

// TODO: add correct error handling

import (
	"database/sql"

	// Import and register sqlite3
	_ "github.com/mattn/go-sqlite3"
)

// Storage - storage for storing incoming webhooks data
type Storage struct {
	DB *sql.DB
}

// WebhookData stores incoming webhook data
type WebhookData struct {
	RowID   int64
	Path    string
	Headers string
	Body    string
}

// NewDB - opens DB, applies needed migrations and returns it
func NewDB(path string) *sql.DB {
	db, err := sql.Open("sqlite3", path)
	checkErr(err)

	applyMigrations(db)

	return db
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

// SaveWebhookData saves webhook data to db
func SaveWebhookData(db *sql.DB, whd WebhookData) WebhookData {
	res, err := db.Exec(`
	INSERT INTO webhooks_data(created_at, updated_at, path, headers, body, visible_at) VALUES(
		datetime("now"),
		datetime("now"),
		$1,
		$2,
		$3,
		datetime("now")
	)
	`, whd.Path, whd.Headers, whd.Body)

	if err != nil {
		panic(err)
	}

	whd.RowID, err = res.LastInsertId()
	if err != nil {
		panic(err)
	}

	return whd
}

// LoadWebhookData loads webhook data saved previously
func LoadWebhookData(db *sql.DB) *WebhookData {
	// 1. Find "visible" row id
	row := db.QueryRow(`
			SELECT rowid, path, headers, body, visible_at
			FROM webhooks_data
			WHERE visible_at <= datetime("now")
			LIMIT 1
			`)
	if row == nil {
		return nil
	}

	whd := &WebhookData{}
	var visibleAt string

	err := row.Scan(&whd.RowID, &whd.Path, &whd.Headers, &whd.Body, &visibleAt)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		panic(err)
	}

	// 2. Try to "lock" it from others by advancing visible_at
	// This will not succeed if somebody else has already done the same.
	res, err := db.Exec(`
			UPDATE webhooks_data SET
				visible_at = datetime("now", "+30 seconds")
			WHERE rowid = $1 AND visible_at = $2
			`, whd.RowID, visibleAt)
	if err != nil {
		panic(err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		panic(err)
	}

	// Somebody else has already grabbed this row, skip this attempt
	if rowsAffected == 0 {
		return nil
	}

	return whd
}

// DeleteWebhookData deletes webhook data from db
func DeleteWebhookData(db *sql.DB, rowID int64) {
	_, err := db.Exec(`
	DELETE FROM webhooks_data
	WHERE rowid = $1
	`, rowID)
	if err != nil {
		panic(err)
	}
}
