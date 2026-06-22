package core

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// restorableTables er tabellene som kopieres ved gjenoppretting fra en
// SQLite-kilde (hele tilstanden, inkludert endringsloggen).
var restorableTables = []string{
	"receipts", "income", "expenses", "exchange_rates",
	"country_tax_rules", "country_tax_types", "foreign_tax_credits",
	"config", "change_log",
}

// Import oppdager filtypen og gjenoppretter tilstanden fra den. Støtter:
//   - full sikkerhetskopi (.zip: data.db + kvitteringer)
//   - rå database (.db / .sqlite)
//
// Returnerer en valideringsfeil ved ukjent filtype.
func (a *App) Import(ctx context.Context, filename string, data []byte) error {
	lower := strings.ToLower(filename)
	switch {
	case bytes.HasPrefix(data, []byte("PK")) || strings.HasSuffix(lower, ".zip"):
		return a.RestoreFromBackupZip(ctx, data)
	case bytes.HasPrefix(data, []byte("SQLite format 3\x00")) ||
		strings.HasSuffix(lower, ".db") || strings.HasSuffix(lower, ".sqlite"):
		tmp := filepath.Join(os.TempDir(), fmt.Sprintf("enk-import-%d.db", time.Now().UnixNano()))
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			return err
		}
		defer os.Remove(tmp)
		return a.RestoreFromSQLite(ctx, tmp)
	default:
		ve := newValidation()
		ve.add("file", "Ukjent filtype. Støtter .zip (sikkerhetskopi) eller .db/.sqlite.")
		return ve
	}
}

// RestoreFromBackupZip pakker ut data.db og kvitteringsfiler fra en
// sikkerhetskopi og gjenoppretter tilstanden.
func (a *App) RestoreFromBackupZip(ctx context.Context, data []byte) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		ve := newValidation()
		ve.add("file", "Ugyldig ZIP-fil: "+err.Error())
		return ve
	}
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("enk-restore-%d.db", time.Now().UnixNano()))
	defer os.Remove(tmp)

	var foundDB bool
	for _, f := range zr.File {
		switch {
		case f.Name == "data.db":
			if err := extractZipFile(f, tmp); err != nil {
				return err
			}
			foundDB = true
		case strings.HasPrefix(f.Name, "receipts/") && !strings.HasSuffix(f.Name, "/"):
			dst := filepath.Join(a.DataDir, filepath.FromSlash(f.Name))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			if err := extractZipFile(f, dst); err != nil {
				return err
			}
		}
	}
	if !foundDB {
		ve := newValidation()
		ve.add("file", "ZIP-filen mangler data.db (er det en gyldig sikkerhetskopi?).")
		return ve
	}
	return a.RestoreFromSQLite(ctx, tmp)
}

// RestoreFromSQLite erstatter all tilstand i den aktive databasen med innholdet
// fra en annen SQLite-fil (via ATTACH). Atomisk for tabellkopieringen.
func (a *App) RestoreFromSQLite(ctx context.Context, srcPath string) error {
	// Kopieringen pinner den eneste DB-tilkoblingen; mirror-synk og kringkasting
	// må skje ETTER at tilkoblingen er frigitt for å unnga deadlock.
	if err := a.restoreCopy(ctx, srcPath); err != nil {
		return err
	}
	a.Events.Broadcast(Event{Type: "import", Action: "import"})
	a.syncMirrorBestEffort(ctx)
	return nil
}

func (a *App) restoreCopy(ctx context.Context, srcPath string) error {
	conn, err := a.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		return err
	}
	defer conn.ExecContext(ctx, "PRAGMA foreign_keys=ON")

	esc := strings.ReplaceAll(srcPath, "'", "''")
	if _, err := conn.ExecContext(ctx, "ATTACH DATABASE '"+esc+"' AS src"); err != nil {
		return fmt.Errorf("åpne kildedatabase: %w", err)
	}
	defer conn.ExecContext(ctx, "DETACH DATABASE src")

	// Hvilke tabeller finnes i kilden?
	srcTables := map[string]bool{}
	rows, err := conn.QueryContext(ctx, "SELECT name FROM src.sqlite_master WHERE type='table'")
	if err != nil {
		return err
	}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err == nil {
			srcTables[n] = true
		}
	}
	rows.Close()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	for _, t := range restorableTables {
		if !srcTables[t] {
			continue
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+t); err != nil {
			return fmt.Errorf("tom %s: %w", t, err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO "+t+" SELECT * FROM src."+t); err != nil {
			return fmt.Errorf("kopier %s: %w", t, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func extractZipFile(f *zip.File, dst string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}
