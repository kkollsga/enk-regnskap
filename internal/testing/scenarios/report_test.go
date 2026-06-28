package scenarios

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/kkollsga/enk-regnskap/internal/core"
	apptest "github.com/kkollsga/enk-regnskap/internal/testing"
)

// Steg 5: rapporter og eksport.

func seedReportData(t *testing.T, h *apptest.Harness) {
	t.Helper()
	h.App.SetConfig(h.Context(), core.ConfigActiveYear, "2025")
	ctx := h.Context()
	h.App.AddIncome(ctx, core.ActorWeb, core.IncomeInput{
		Date: "2025-01-15", Description: "Tjeneste 1", Currency: "NOK",
		CountryCode: "NO", AmountOrig: 100000, Category: "tjenesteinntekt",
	})
	h.App.AddIncome(ctx, core.ActorWeb, core.IncomeInput{
		Date: "2025-02-15", Description: "Honorar", Currency: "NOK",
		CountryCode: "NO", AmountOrig: 40000, Category: "honorar",
	})
	h.App.AddExpense(ctx, core.ActorWeb, core.ExpenseInput{
		Date: "2025-01-20", Description: "PC", Category: "små_driftsmidler", AmountOrig: 15000,
	})
}

func TestAnnualReportIsValidPDF(t *testing.T) {
	h := apptest.Start(t)
	seedReportData(t, h)
	status, body, hdr := h.Browser().GetRaw("/reports/annual.pdf")
	apptest.AssertEqual(t, status, 200, "annual.pdf status")
	if !strings.HasPrefix(body, "%PDF") {
		t.Errorf("PDF magic bytes mangler, fikk: %q", body[:min(8, len(body))])
	}
	if ct := hdr.Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestTaxSummaryIsValidPDF(t *testing.T) {
	h := apptest.Start(t)
	seedReportData(t, h)
	_, body, _ := h.Browser().GetRaw("/reports/tax-summary.pdf")
	if !strings.HasPrefix(body, "%PDF") {
		t.Error("næringsspesifikasjon er ikke en gyldig PDF")
	}
}

func TestTransactionsXLSXIsValid(t *testing.T) {
	h := apptest.Start(t)
	seedReportData(t, h)
	_, body, _ := h.Browser().GetRaw("/reports/transactions.xlsx")
	// Gyldig xlsx er en ZIP -> starter med "PK".
	if !strings.HasPrefix(body, "PK") {
		t.Fatal("xlsx mangler ZIP-signatur (PK)")
	}
	// Må kunne åpnes av excelize.
	f, err := excelize.OpenReader(bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("kunne ikke åpne xlsx: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows("Inntekter")
	if err != nil {
		t.Fatalf("mangler Inntekter-ark: %v", err)
	}
	if len(rows) < 3 {
		t.Errorf("forventet minst header + 2 inntekter, fikk %d rader", len(rows))
	}
}

func TestReportTotalsMatchDatabase(t *testing.T) {
	h := apptest.Start(t)
	seedReportData(t, h)
	rep, err := h.App.BuildReport(h.Context(), 2025)
	if err != nil {
		t.Fatal(err)
	}
	// 100000 + 40000 = 140000 inntekt
	if rep.TotalIncome != 140000 {
		t.Errorf("TotalIncome = %v, forventet 140000", rep.TotalIncome)
	}
	// PC 15000, 100% fradrag
	if rep.TotalDeductible != 15000 {
		t.Errorf("TotalDeductible = %v, forventet 15000", rep.TotalDeductible)
	}
	if rep.Result != 125000 {
		t.Errorf("Result = %v, forventet 125000", rep.Result)
	}
}

func TestTransactionsCSV(t *testing.T) {
	h := apptest.Start(t)
	seedReportData(t, h)
	status, body, hdr := h.Browser().GetRaw("/reports/transactions.csv")
	apptest.AssertEqual(t, status, 200, "csv status")
	if !strings.Contains(hdr.Get("Content-Type"), "text/csv") {
		t.Errorf("Content-Type = %q", hdr.Get("Content-Type"))
	}
	if !strings.Contains(body, "Tjeneste 1") || !strings.Contains(body, "inntekt") {
		t.Error("CSV mangler forventede rader")
	}
}
