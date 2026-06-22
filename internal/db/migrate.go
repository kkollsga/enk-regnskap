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
// (CREATE TABLE/INDEX IF NOT EXISTS), så funksjonen kan trygt kjores ved
// hver oppstart.
func Migrate(conn *sql.DB) error {
	if _, err := conn.Exec(schemaSQL); err != nil {
		return fmt.Errorf("kjor schema.sql: %w", err)
	}
	// Legg til nyere kolonner på eksisterende databaser (idempotent). Dette må
	// skje FØR indeksen under, ellers feiler indeks på eldre databaser.
	for _, c := range []struct{ table, column, def string }{
		{"receipts", "title", "TEXT"},
		{"receipts", "description", "TEXT"},
		{"receipts", "parent_kind", "TEXT"},
		{"receipts", "parent_id", "INTEGER"},
	} {
		if err := ensureColumn(conn, c.table, c.column, c.def); err != nil {
			return err
		}
	}
	// Indeks på vedleggets parent-felter (etter at kolonnene finnes).
	if _, err := conn.Exec(`CREATE INDEX IF NOT EXISTS idx_receipts_parent ON receipts(parent_kind, parent_id)`); err != nil {
		return fmt.Errorf("opprett vedleggsindeks: %w", err)
	}
	// Migrer eksisterende enkeltkoblinger (income/expenses.receipt_id) til
	// vedleggets parent-felter.
	_, _ = conn.Exec(`UPDATE receipts SET parent_kind='income', parent_id=(
		SELECT id FROM income WHERE income.receipt_id = receipts.id LIMIT 1)
		WHERE parent_kind IS NULL AND EXISTS (SELECT 1 FROM income WHERE income.receipt_id = receipts.id)`)
	_, _ = conn.Exec(`UPDATE receipts SET parent_kind='expense', parent_id=(
		SELECT id FROM expenses WHERE expenses.receipt_id = receipts.id LIMIT 1)
		WHERE parent_kind IS NULL AND EXISTS (SELECT 1 FROM expenses WHERE expenses.receipt_id = receipts.id)`)
	return nil
}

// ensureColumn legger til en kolonne hvis den mangler (SQLite mangler
// "ADD COLUMN IF NOT EXISTS").
func ensureColumn(conn *sql.DB, table, column, def string) error {
	rows, err := conn.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return err
	}
	defer rows.Close()
	exists := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			exists = true
		}
	}
	if !exists {
		if _, err := conn.Exec("ALTER TABLE " + table + " ADD COLUMN " + column + " " + def); err != nil {
			return fmt.Errorf("legg til kolonne %s.%s: %w", table, column, err)
		}
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
