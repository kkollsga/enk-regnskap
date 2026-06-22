// Package db handler databasetilkobling, migrasjoner og seed-data for
// ENK Regnskap. Den typesikre query-koden i denne pakken genereres av sqlc
// (filer som slutter på .sql.go, samt models.go og querier.go).
package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open åpner (eller oppretter) SQLite-databasen på angitt sti, kjorer
// migrasjoner og seed-data, og returnerer en klar *sql.DB.
//
// Bruk ":memory:" som path for en flyktig database (brukes i tester).
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", path)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("åpne database: %w", err)
	}
	// SQLite (uten WAL) takler kun en skriver; appen er enbruker, men vi
	// begrenser tilkoblingspoolen for forutsigbar oppforsel.
	conn.SetMaxOpenConns(1)

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := Migrate(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrer database: %w", err)
	}

	if err := SeedCountryData(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("seed landdata: %w", err)
	}

	return conn, nil
}
