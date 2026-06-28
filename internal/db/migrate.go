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
		{"income_foreign_taxes", "treatment", "TEXT NOT NULL DEFAULT 'credit'"},
		{"expenses", "amount_orig", "REAL NOT NULL DEFAULT 0"},
		{"expenses", "currency", "TEXT NOT NULL DEFAULT 'NOK'"},
		{"expenses", "exchange_rate", "REAL"},
		{"expenses", "rate_date", "TEXT"},
		{"expenses", "country_code", "TEXT NOT NULL DEFAULT 'NO'"},
		{"country_tax_types", "default_treatment", "TEXT"},
	} {
		if err := ensureColumn(conn, c.table, c.column, c.def); err != nil {
			return err
		}
	}
	// Eldre utgifter var alltid NOK: sett amount_orig = amount_nok der den ikke
	// er satt (nye kolonner får default 0).
	_, _ = conn.Exec(`UPDATE expenses SET amount_orig = amount_nok WHERE amount_orig = 0 AND amount_nok <> 0`)
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

	// Flytt flate utenlandsskatt-kolonner på income til den normaliserte
	// income_foreign_taxes-tabellen, og fjern så kolonnene. Tilstedeværelsen av
	// foreign_tax_orig fungerer som idempotens-vakt: når kolonnene er droppet
	// kjøres ikke dette på nytt.
	if err := migrateForeignTaxes(conn); err != nil {
		return err
	}
	// Erstatt den binære creditable-kolonnen med treatment-enum (credit/deduct/
	// none). Idempotent: kjøres bare så lenge creditable fortsatt finnes.
	if err := migrateForeignTaxTreatment(conn); err != nil {
		return err
	}
	return nil
}

// migrateForeignTaxTreatment flytter income_foreign_taxes.creditable over til
// treatment-kolonnen (1->'credit', 0->'deduct') og dropper creditable.
func migrateForeignTaxTreatment(conn *sql.DB) error {
	has, err := columnExists(conn, "income_foreign_taxes", "creditable")
	if err != nil {
		return err
	}
	if !has {
		return nil // allerede migrert (eller fersk database)
	}
	if _, err := conn.Exec(`UPDATE income_foreign_taxes
		SET treatment = CASE creditable WHEN 1 THEN 'credit' ELSE 'deduct' END`); err != nil {
		return fmt.Errorf("backfill treatment: %w", err)
	}
	if _, err := conn.Exec(`ALTER TABLE income_foreign_taxes DROP COLUMN creditable`); err != nil {
		return fmt.Errorf("drop income_foreign_taxes.creditable: %w", err)
	}
	return nil
}

// migrateForeignTaxes overfører gamle income.foreign_tax_*-kolonner til
// income_foreign_taxes og dropper deretter kolonnene. Idempotent: gjør ingenting
// hvis kolonnene allerede er borte.
func migrateForeignTaxes(conn *sql.DB) error {
	has, err := columnExists(conn, "income", "foreign_tax_orig")
	if err != nil {
		return err
	}
	if !has {
		return nil // allerede migrert
	}
	// Kopier eksisterende enkelt-skattelinjer (kun der det faktisk er trukket
	// skatt med et beløp). Mangler typen, merkes den som UKJENT.
	if _, err := conn.Exec(`INSERT INTO income_foreign_taxes (income_id, tax_type, amount_orig, currency, amount_nok)
		SELECT id,
		       COALESCE(NULLIF(TRIM(foreign_tax_type), ''), 'UKJENT'),
		       foreign_tax_orig,
		       COALESCE(NULLIF(TRIM(foreign_tax_currency), ''), currency),
		       COALESCE(foreign_tax_nok, foreign_tax_orig)
		FROM income
		WHERE foreign_tax_paid = 1 AND foreign_tax_orig IS NOT NULL AND foreign_tax_orig > 0`); err != nil {
		return fmt.Errorf("backfill income_foreign_taxes: %w", err)
	}
	for _, col := range []string{"foreign_tax_orig", "foreign_tax_currency", "foreign_tax_nok", "foreign_tax_type"} {
		if _, err := conn.Exec("ALTER TABLE income DROP COLUMN " + col); err != nil {
			return fmt.Errorf("drop income.%s: %w", col, err)
		}
	}
	return nil
}

// columnExists sjekker om en kolonne finnes på en tabell.
func columnExists(conn *sql.DB, table, column string) (bool, error) {
	rows, err := conn.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
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
