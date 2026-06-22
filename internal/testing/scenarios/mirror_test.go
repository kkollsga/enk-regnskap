package scenarios

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/core"
	apptest "github.com/kkollsga/enk-regnskap/internal/testing"
)

// Lesbar mirror-mappe: ekstra sikkerhetsnett + import for å sette tilstand.

func TestMirrorAutoSyncIsHumanReadable(t *testing.T) {
	h := apptest.Start(t)
	h.App.AddIncome(h.Context(), core.ActorWeb, core.IncomeInput{
		Date: "2025-01-15", Description: "Lesbar testinntekt", Currency: "NOK",
		CountryCode: "NO", AmountOrig: 1234, Category: "tjenesteinntekt",
	})

	incomePath := filepath.Join(h.DataDir, "mirror", "income.json")
	b, err := os.ReadFile(incomePath)
	if err != nil {
		t.Fatalf("mirror income.json ble ikke skrevet: %v", err)
	}
	// Menneskelesbar: pretty JSON med klartekstfelter, ingen sql.Null-stoy.
	s := string(b)
	if !strings.Contains(s, "Lesbar testinntekt") || !strings.Contains(s, "\"amount_nok\": 1234") {
		t.Errorf("income.json ikke som forventet:\n%s", s)
	}
	if strings.Contains(s, "Valid") {
		t.Error("income.json inneholder sql.Null-stoy (ikke lesbart)")
	}
	// Skal kunne parses som JSON-array.
	var arr []map[string]any
	if err := json.Unmarshal(b, &arr); err != nil || len(arr) != 1 {
		t.Errorf("income.json er ikke en gyldig JSON-array med 1 element: %v", err)
	}
}

func TestMirrorIncludesReceiptFiles(t *testing.T) {
	h := apptest.Start(t)
	rec, err := h.App.SaveReceipt(h.Context(), core.ActorWeb, core.ReceiptInput{
		OriginalName: "kvittering.png", MimeType: "image/png", Data: onePixelPNG(), TaxYear: 2025,
	})
	if err != nil {
		t.Fatal(err)
	}
	mirrored := filepath.Join(h.DataDir, "mirror", "receipts", rec.Filename)
	if _, err := os.Stat(mirrored); err != nil {
		t.Errorf("kvitteringsfil ikke speilet til %s: %v", mirrored, err)
	}
}

func TestMirrorImportSetsState(t *testing.T) {
	h := apptest.Start(t)
	ctx := h.Context()
	h.App.SetConfig(ctx, core.ConfigActiveYear, "2025")

	// Bygg en tilstand og la mirror bli skrevet automatisk.
	h.App.AddIncome(ctx, core.ActorWeb, core.IncomeInput{
		Date: "2025-02-01", Description: "Beholdes", Currency: "NOK",
		CountryCode: "NO", AmountOrig: 5000, Category: "tjenesteinntekt",
	})
	h.App.AddExpense(ctx, core.ActorWeb, core.ExpenseInput{
		Date: "2025-02-02", Description: "Beholdes utgift", Category: "kontorrekvisita", AmountNOK: 800,
	})

	// Ta en kopi av mirror-mappen til et eget sted (representerer brukerens backup-mappe).
	backupDir := filepath.Join(t.TempDir(), "min-backup")
	copyDir(t, filepath.Join(h.DataDir, "mirror"), backupDir)

	// Endre tilstanden etterpå (legg til mer som IKKE er i backup-en).
	h.App.AddIncome(ctx, core.ActorWeb, core.IncomeInput{
		Date: "2025-03-01", Description: "Skal forsvinne ved import", Currency: "NOK",
		CountryCode: "NO", AmountOrig: 99999, Category: "honorar",
	})
	if rows, _ := h.App.ListIncome(ctx, 2025); len(rows) != 2 {
		t.Fatalf("forventet 2 inntekter for import, fikk %d", len(rows))
	}

	// Importer backup-mappen -> setter tilstanden tilbake.
	if err := h.App.ImportMirror(ctx, backupDir); err != nil {
		t.Fatalf("ImportMirror: %v", err)
	}

	rows, _ := h.App.ListIncome(ctx, 2025)
	if len(rows) != 1 {
		t.Fatalf("etter import forventet 1 inntekt, fikk %d", len(rows))
	}
	if rows[0].Description != "Beholdes" {
		t.Errorf("feil inntekt etter import: %q", rows[0].Description)
	}
	exps, _ := h.App.ListExpenses(ctx, 2025)
	if len(exps) != 1 || exps[0].Description != "Beholdes utgift" {
		t.Errorf("utgifter ikke korrekt gjenopprettet: %+v", exps)
	}
}

func TestMirrorImportRebuildsForeignCredits(t *testing.T) {
	h := apptest.Start(t)
	ctx := h.Context()
	h.App.SetConfig(ctx, core.ConfigActiveYear, "2025")
	h.Mock.AddRate("BRL", "2025-04-10", 2.00)
	h.App.AddIncome(ctx, core.ActorWeb, core.IncomeInput{
		Date: "2025-04-10", Description: "Brasil", Currency: "BRL", CountryCode: "BR",
		AmountOrig: 10000, Category: "tjenesteinntekt", ForeignTaxPaid: core.ForeignTaxYes,
		ForeignTaxOrig: 1500, ForeignTaxCurrency: "BRL", ForeignTaxType: "IRRF",
	})

	backupDir := filepath.Join(t.TempDir(), "b")
	copyDir(t, filepath.Join(h.DataDir, "mirror"), backupDir)

	// Importer til en fersk app.
	h2 := apptest.Start(t)
	if err := h2.App.ImportMirror(h2.Context(), backupDir); err != nil {
		t.Fatal(err)
	}
	credits, _ := h2.App.ForeignTaxForYear(h2.Context(), 2025)
	if len(credits) != 1 {
		t.Fatalf("kreditfradrag ble ikke gjenoppbygd, fikk %d", len(credits))
	}
	if credits[0].Credit.IncomeNok != 20000 {
		t.Errorf("gjenoppbygd utenlandsinntekt = %v, forventet 20000", credits[0].Credit.IncomeNok)
	}
}

// copyDir kopierer en mappe rekursivt (testhjelper).
func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		os.MkdirAll(filepath.Dir(target), 0o755)
		return os.WriteFile(target, b, 0o644)
	})
}
