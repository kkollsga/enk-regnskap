package scenarios

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/core"
	"github.com/kkollsga/enk-regnskap/internal/db"
	apptest "github.com/kkollsga/enk-regnskap/internal/testing"
)

// Full sikkerhetskopi: data.db (inkl. endringslogg) + kvitteringsfiler.

func TestBackupZipIsComplete(t *testing.T) {
	h := apptest.Start(t)
	h.LoadFixtures(t)
	// Last opp en kvittering slik at backup-en ogsaa inneholder filer.
	if _, err := h.App.SaveReceipt(h.Context(), core.ActorWeb, core.ReceiptInput{
		OriginalName: "kvittering.png", MimeType: "image/png", Data: onePixelPNG(), TaxYear: 2025,
	}); err != nil {
		t.Fatal(err)
	}

	status, body, hdr := h.Browser().GetRaw("/export/backup.zip")
	apptest.AssertEqual(t, status, 200, "backup status")
	if ct := hdr.Get("Content-Type"); ct != "application/zip" {
		t.Errorf("Content-Type = %q, forventet application/zip", ct)
	}

	raw := []byte(body)
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("kunne ikke lese ZIP: %v", err)
	}

	var hasDB, hasReceipt bool
	var dbBytes []byte
	for _, f := range zr.File {
		switch {
		case f.Name == "data.db":
			hasDB = true
			rc, _ := f.Open()
			dbBytes, _ = io.ReadAll(rc)
			rc.Close()
		case strings.HasPrefix(f.Name, "receipts/"):
			hasReceipt = true
		}
	}
	if !hasDB {
		t.Error("backup mangler data.db")
	}
	if !hasReceipt {
		t.Error("backup mangler kvitteringsfiler")
	}
	// data.db skal vaere en gyldig SQLite-fil.
	if !bytes.HasPrefix(dbBytes, []byte("SQLite format 3\x00")) {
		t.Error("data.db har ikke gyldig SQLite-signatur")
	}
}

func TestBackupDBReopensWithData(t *testing.T) {
	h := apptest.Start(t)
	h.LoadFixtures(t)

	var buf bytes.Buffer
	if err := h.App.WriteBackup(h.Context(), &buf); err != nil {
		t.Fatal(err)
	}
	raw := buf.Bytes()
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}

	// Skriv data.db til en midlertidig fil og aapne den paa nytt.
	tmp := filepath.Join(t.TempDir(), "restored.db")
	for _, f := range zr.File {
		if f.Name == "data.db" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			if err := os.WriteFile(tmp, b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	conn, err := db.Open(tmp)
	if err != nil {
		t.Fatalf("gjenaapne backup-db: %v", err)
	}
	defer conn.Close()
	q := db.New(conn)

	// Inntektene skal vaere med (12 i datasettet).
	rows, err := q.ListIncomeByYear(h.Context(), 2025)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 12 {
		t.Errorf("gjenopprettet db har %d inntekter, forventet 12", len(rows))
	}

	// Endringsloggen (rollback-historikk) skal ogsaa vaere med.
	changes, err := q.ListChangeLog(h.Context(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) == 0 {
		t.Error("gjenopprettet db mangler endringslogg")
	}
}
