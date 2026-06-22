package core

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// WriteBackup skriver en fullstendig, ren sikkerhetskopi som en ZIP til w:
//   - data.db  (et konsistent oyeblikksbilde av HELE databasen, inkludert
//     endringsloggen/rollback-historikken, via SQLite VACUUM INTO)
//   - receipts/...  (alle opplastede kvitteringsfiler)
//
// Resultatet kan gjenopprettes ved aa pakke det ut i en data-mappe.
func (a *App) WriteBackup(ctx context.Context, w io.Writer) error {
	// 1. Konsistent oyeblikksbilde av databasen til en midlertidig fil.
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("enk-backup-%d.db", time.Now().UnixNano()))
	defer os.Remove(tmp)
	esc := strings.ReplaceAll(tmp, "'", "''")
	if _, err := a.DB.ExecContext(ctx, "VACUUM INTO '"+esc+"'"); err != nil {
		return fmt.Errorf("lag databasebilde: %w", err)
	}

	zw := zip.NewWriter(w)
	if err := addFileToZip(zw, tmp, "data.db"); err != nil {
		zw.Close()
		return err
	}

	// 2. Alle kvitteringsfiler under data/receipts/.
	receiptsDir := filepath.Join(a.DataDir, "receipts")
	if info, err := os.Stat(receiptsDir); err == nil && info.IsDir() {
		err = filepath.WalkDir(receiptsDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(a.DataDir, path)
			if err != nil {
				return err
			}
			return addFileToZip(zw, path, filepath.ToSlash(rel))
		})
		if err != nil {
			zw.Close()
			return fmt.Errorf("legg til kvitteringer: %w", err)
		}
	}

	return zw.Close()
}

// BackupFilename gir et datostemplet filnavn for sikkerhetskopien.
func BackupFilename() string {
	return "enk-regnskap-backup-" + time.Now().Format("2006-01-02") + ".zip"
}

func addFileToZip(zw *zip.Writer, path, name string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	hw, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(hw, f)
	return err
}
