package scenarios

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/core"
	apptest "github.com/kkollsga/enk-regnskap/internal/testing"
)

// Generisk import: .zip sikkerhetskopi, .db database, og speilkopi-mappe.

func TestImportFromBackupZip(t *testing.T) {
	src := apptest.Start(t)
	src.LoadFixtures(t)
	var buf bytes.Buffer
	if err := src.App.WriteBackup(src.Context(), &buf); err != nil {
		t.Fatal(err)
	}

	// Fersk app, importer zip-en.
	dst := apptest.Start(t)
	if err := dst.App.Import(dst.Context(), "backup.zip", buf.Bytes()); err != nil {
		t.Fatalf("Import zip: %v", err)
	}
	rows, _ := dst.App.ListIncome(dst.Context(), 2025)
	if len(rows) != 12 {
		t.Errorf("etter zip-import: %d inntekter, forventet 12", len(rows))
	}
}

func TestImportFromRawDB(t *testing.T) {
	src := apptest.Start(t)
	src.LoadFixtures(t)
	// Lag et rent oyeblikksbilde av databasen.
	dbBytes := snapshotDB(t, src)

	dst := apptest.Start(t)
	if err := dst.App.Import(dst.Context(), "data.db", dbBytes); err != nil {
		t.Fatalf("Import .db: %v", err)
	}
	rows, _ := dst.App.ListExpenses(dst.Context(), 2025)
	if len(rows) != 8 {
		t.Errorf("etter db-import: %d utgifter, forventet 8", len(rows))
	}
	// Endringsloggen skal ogsaa vaere med (full kopi).
	changes, _ := dst.App.Q.ListChangeLog(dst.Context(), 100)
	if len(changes) == 0 {
		t.Error("import mistet endringsloggen")
	}
}

func TestImportFromMirrorFolder(t *testing.T) {
	src := apptest.Start(t)
	src.LoadFixtures(t)
	// Kopier speilkopi-mappen til et eget sted.
	mirror := filepath.Join(t.TempDir(), "mirror")
	copyDir(t, filepath.Join(src.DataDir, "mirror"), mirror)

	dst := apptest.Start(t)
	if err := dst.App.ImportMirror(dst.Context(), mirror); err != nil {
		t.Fatalf("ImportMirror: %v", err)
	}
	rows, _ := dst.App.ListIncome(dst.Context(), 2025)
	if len(rows) != 12 {
		t.Errorf("etter mirror-import: %d inntekter, forventet 12", len(rows))
	}
}

func TestImportReplacesExistingData(t *testing.T) {
	src := apptest.Start(t)
	src.LoadFixtures(t) // 12 inntekter
	var buf bytes.Buffer
	src.App.WriteBackup(src.Context(), &buf)

	dst := apptest.Start(t)
	dst.App.SetConfig(dst.Context(), core.ConfigActiveYear, "2025")
	// Forhaandsfyll med noe annet som skal forsvinne.
	dst.App.AddIncome(dst.Context(), core.ActorWeb, core.IncomeInput{
		Date: "2025-01-01", Description: "Skal erstattes", Currency: "NOK",
		CountryCode: "NO", AmountOrig: 999, Category: "annet",
	})
	if err := dst.App.Import(dst.Context(), "b.zip", buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	rows, _ := dst.App.ListIncome(dst.Context(), 2025)
	if len(rows) != 12 {
		t.Errorf("import skal ERSTATTE: %d inntekter, forventet 12", len(rows))
	}
	for _, r := range rows {
		if r.Description == "Skal erstattes" {
			t.Error("gammel data ble ikke erstattet")
		}
	}
}

func TestImportViaHTTPEndpoint(t *testing.T) {
	src := apptest.Start(t)
	src.LoadFixtures(t)
	var buf bytes.Buffer
	src.App.WriteBackup(src.Context(), &buf)

	dst := apptest.Start(t)
	res := dst.Browser().PostMultipart("/import",
		map[string]string{}, "file", "backup.zip", "application/zip", buf.Bytes())
	apptest.AssertStatus(t, res, 200) // foelger redirect til /
	rows, _ := dst.App.ListIncome(dst.Context(), 2025)
	if len(rows) != 12 {
		t.Errorf("HTTP-import: %d inntekter, forventet 12", len(rows))
	}
}

func TestImportUnknownTypeRejected(t *testing.T) {
	h := apptest.Start(t)
	err := h.App.Import(h.Context(), "notes.txt", []byte("hello world"))
	if err == nil {
		t.Fatal("ukjent filtype skulle gi feil")
	}
	if _, ok := core.AsValidation(err); !ok {
		t.Errorf("forventet valideringsfeil, fikk %T", err)
	}
}

// snapshotDB lager en ren kopi av appens database (via backup-zip, henter data.db).
func snapshotDB(t *testing.T, h *apptest.Harness) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := h.App.WriteBackup(h.Context(), &buf); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name == "data.db" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			return b
		}
	}
	t.Fatal("fant ikke data.db i backup")
	return nil
}

var _ = os.Stat
