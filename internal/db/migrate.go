package db

import (
	"database/sql"
	_ "embed"
	"fmt"
)

//go:embed schema.sql
var schemaSQL string

//go:embed seed_country_data.sql
var seedCountrySQL string

// Migrate kjorer skjema-migrasjonen. Alle setninger er idempotente
// (CREATE TABLE/INDEX IF NOT EXISTS), saa funksjonen kan trygt kjores ved
// hver oppstart.
func Migrate(conn *sql.DB) error {
	if _, err := conn.Exec(schemaSQL); err != nil {
		return fmt.Errorf("kjor schema.sql: %w", err)
	}
	return nil
}

// SeedCountryData populerer country_tax_rules og country_tax_types med
// Norge og Brasil. Idempotent via INSERT OR IGNORE.
func SeedCountryData(conn *sql.DB) error {
	if _, err := conn.Exec(seedCountrySQL); err != nil {
		return fmt.Errorf("kjor seed_country_data.sql: %w", err)
	}
	return nil
}
