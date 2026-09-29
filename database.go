package main

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite" // the SQLite driver (the _ means: load it, we don't call it directly)
)

// db is our connection to the database. Every file can use it.
var db *sql.DB

// connectDatabase opens (or creates) a file called bookshare.db
// SQLite = a whole database stored in ONE file, no installation needed.
func connectDatabase() {
	var err error
	db, err = sql.Open("sqlite", "bookshare.db")
	if err != nil {
		log.Fatal("could not open database: ", err)
	}

	// SQLite works best with one connection at a time.
	// RULE FOR THIS PROJECT: never run a new query while you are still
	// looping over rows from another query. Finish (close) the loop first.
	db.SetMaxOpenConns(1)

	createTables()
}

// createTables runs once at startup. "IF NOT EXISTS" means it's safe to run every time.
func createTables() {
	tables := []string{
		// People
		`CREATE TABLE IF NOT EXISTS users (
			id       INTEGER PRIMARY KEY AUTOINCREMENT,
			name     TEXT NOT NULL,
			email    TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			bio      TEXT NOT NULL DEFAULT '',
			city     TEXT NOT NULL DEFAULT ''
		)`,

		// Books (status: available / borrowed / sold)
		`CREATE TABLE IF NOT EXISTS books (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			title       TEXT NOT NULL,
			author      TEXT NOT NULL,
			genre       TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			owner_id    INTEGER NOT NULL,
			for_lend    INTEGER NOT NULL DEFAULT 0,
			for_sale    INTEGER NOT NULL DEFAULT 0,
			price       REAL NOT NULL DEFAULT 0,
			status      TEXT NOT NULL DEFAULT 'available'
		)`,

		// Borrow requests (status: pending / approved / rejected / returned)
		`CREATE TABLE IF NOT EXISTS borrow_requests (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			book_id      INTEGER NOT NULL,
			borrower_id  INTEGER NOT NULL,
			status       TEXT NOT NULL DEFAULT 'pending',
			requested_at TEXT NOT NULL,
			due_date     TEXT NOT NULL DEFAULT '',
			returned_at  TEXT NOT NULL DEFAULT ''
		)`,

		// Shopping cart
		`CREATE TABLE IF NOT EXISTS cart_items (
			id      INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			book_id INTEGER NOT NULL,
			UNIQUE(user_id, book_id)
		)`,

		// Orders (created at checkout)
		`CREATE TABLE IF NOT EXISTS orders (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			buyer_id   INTEGER NOT NULL,
			total      REAL NOT NULL,
			created_at TEXT NOT NULL
		)`,

		`CREATE TABLE IF NOT EXISTS order_items (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id  INTEGER NOT NULL,
			book_id   INTEGER NOT NULL,
			seller_id INTEGER NOT NULL,
			price     REAL NOT NULL
		)`,

		// Ratings between users (one review per pair)
		`CREATE TABLE IF NOT EXISTS reviews (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			reviewer_id INTEGER NOT NULL,
			reviewed_id INTEGER NOT NULL,
			rating      INTEGER NOT NULL,
			comment     TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL,
			UNIQUE(reviewer_id, reviewed_id)
		)`,
	}

	for _, statement := range tables {
		_, err := db.Exec(statement)
		if err != nil {
			log.Fatal("could not create table: ", err)
		}
	}
}
