package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// openTest åpner en flyktig in-memory database for testing.
func openTest(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open(:memory:): %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func tableExists(t *testing.T, conn *sql.DB, name string) bool {
	t.Helper()
	var n string
	err := conn.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, name,
	).Scan(&n)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatalf("sjekk tabell %s: %v", name, err)
	}
	return n == name
}

func TestSchemaCreatesAllTables(t *testing.T) {
	conn := openTest(t)
	want := []string{
		"receipts", "income", "expenses", "exchange_rates",
		"country_tax_rules", "country_tax_types", "foreign_tax_credits",
		"config", "change_log",
	}
	for _, tbl := range want {
		if !tableExists(t, conn, tbl) {
			t.Errorf("tabell %q mangler", tbl)
		}
	}
}

func TestSeedCountryData(t *testing.T) {
	conn := openTest(t)
	q := New(conn)
	ctx := context.Background()

	// Norge: regel finnes for 2024 og 2025
	for _, year := range []int64{2024, 2025} {
		r, err := q.GetCountryRule(ctx, GetCountryRuleParams{
			CountryCode:   "NO",
			EffectiveFrom: year,
			EffectiveTo:   sql.NullInt64{Int64: year, Valid: true},
		})
		if err != nil {
			t.Fatalf("GetCountryRule NO %d: %v", year, err)
		}
		if r.CountryName != "Norge" {
			t.Errorf("NO %d navn = %q", year, r.CountryName)
		}
	}

	// Brasil 2024: ingen skatteavtale (intern rett)
	br24, err := q.GetCountryRule(ctx, GetCountryRuleParams{
		CountryCode: "BR", EffectiveFrom: 2024,
		EffectiveTo: sql.NullInt64{Int64: 2024, Valid: true},
	})
	if err != nil {
		t.Fatalf("GetCountryRule BR 2024: %v", err)
	}
	if br24.HasTaxTreaty != 0 {
		t.Errorf("BR 2024 has_tax_treaty = %d, forventet 0 (ingen avtale)", br24.HasTaxTreaty)
	}

	// Brasil 2025: skatteavtale i kraft, kreditmetoden
	br25, err := q.GetCountryRule(ctx, GetCountryRuleParams{
		CountryCode: "BR", EffectiveFrom: 2025,
		EffectiveTo: sql.NullInt64{Int64: 2025, Valid: true},
	})
	if err != nil {
		t.Fatalf("GetCountryRule BR 2025: %v", err)
	}
	if br25.HasTaxTreaty != 1 {
		t.Errorf("BR 2025 has_tax_treaty = %d, forventet 1", br25.HasTaxTreaty)
	}
	if br25.TreatyMethod.String != "credit" {
		t.Errorf("BR 2025 treaty_method = %q, forventet credit", br25.TreatyMethod.String)
	}
	if br25.TreatyInForceDate.String != "2024-12-30" {
		t.Errorf("BR 2025 in_force_date = %q, forventet 2024-12-30", br25.TreatyInForceDate.String)
	}

	// Brasil-skattetyper: IRRF skal være krediterbar, COFINS ikke
	types, err := q.ListCountryTaxTypes(ctx, ListCountryTaxTypesParams{
		CountryCode: "BR", EffectiveFrom: 2025,
		EffectiveTo: sql.NullInt64{Int64: 2025, Valid: true},
	})
	if err != nil {
		t.Fatalf("ListCountryTaxTypes BR: %v", err)
	}
	byCode := map[string]CountryTaxType{}
	for _, ty := range types {
		byCode[ty.TaxTypeCode] = ty
	}
	if irrf, ok := byCode["IRRF"]; !ok || irrf.IsCreditableInNorway.Int64 != 1 {
		t.Errorf("IRRF mangler eller ikke krediterbar: %+v", irrf)
	}
	if cofins, ok := byCode["COFINS"]; !ok || cofins.IsCreditableInNorway.Int64 != 0 {
		t.Errorf("COFINS mangler eller feilaktig krediterbar: %+v", cofins)
	}
}

func TestMigrationsIdempotent(t *testing.T) {
	conn := openTest(t)
	// Open kjorte allerede migrasjon + seed en gang. Kjor dem igjen.
	if err := Migrate(conn); err != nil {
		t.Fatalf("andre Migrate: %v", err)
	}
	if err := SeedCountryData(conn); err != nil {
		t.Fatalf("andre SeedCountryData: %v", err)
	}
	if err := SeedCountryData(conn); err != nil {
		t.Fatalf("tredje SeedCountryData: %v", err)
	}

	// Antallet land-regler skal være uendret (ingen duplikater)
	var count int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM country_tax_rules`).Scan(&count); err != nil {
		t.Fatalf("tell country_tax_rules: %v", err)
	}
	// NO(1) + BR(2 perioder) = 3
	if count != 3 {
		t.Errorf("country_tax_rules count = %d, forventet 3 (ingen duplikater etter gjentatt seed)", count)
	}

	var typeCount int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM country_tax_types`).Scan(&typeCount); err != nil {
		t.Fatalf("tell country_tax_types: %v", err)
	}
	// BR(5) + NO(3) = 8
	if typeCount != 8 {
		t.Errorf("country_tax_types count = %d, forventet 8", typeCount)
	}
}

// TestMigrateOldReceiptsTable simulerer en eldre database der receipts mangler
// parent-kolonnene, og verifiserer at migrasjonen legger dem til (og ikke
// feiler på indeksen).
func TestMigrateOldReceiptsTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	conn, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxOpenConns(1)
	defer conn.Close()

	// Gammel receipts-tabell (uten title/description/parent_*).
	if _, err := conn.Exec(`CREATE TABLE receipts (
		id INTEGER PRIMARY KEY, filename TEXT NOT NULL, original_name TEXT NOT NULL,
		mime_type TEXT NOT NULL, tax_year INTEGER,
		uploaded_at TEXT NOT NULL DEFAULT (datetime('now')))`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`CREATE TABLE income (id INTEGER PRIMARY KEY, receipt_id INTEGER, tax_year INTEGER, country_code TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`CREATE TABLE expenses (id INTEGER PRIMARY KEY, receipt_id INTEGER, tax_year INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO receipts (id, filename, original_name, mime_type) VALUES (1, 'a.png', 'a.png', 'image/png')`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO income (id, receipt_id, tax_year, country_code) VALUES (5, 1, 2025, 'NO')`); err != nil {
		t.Fatal(err)
	}

	// Migrasjonen skal IKKE feile (det var feilen brukeren fikk).
	if err := Migrate(conn); err != nil {
		t.Fatalf("Migrate på gammel database feilet: %v", err)
	}

	// Kolonnene skal nå finnes, og koblingen være migrert.
	var pk sql.NullString
	var pid sql.NullInt64
	if err := conn.QueryRow(`SELECT parent_kind, parent_id FROM receipts WHERE id=1`).Scan(&pk, &pid); err != nil {
		t.Fatalf("nye kolonner mangler: %v", err)
	}
	if pk.String != "income" || pid.Int64 != 5 {
		t.Errorf("kobling ikke migrert: parent_kind=%q parent_id=%d", pk.String, pid.Int64)
	}
}
