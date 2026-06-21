package scenarios

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/core"
	apptest "github.com/kkollsga/enk-regnskap/internal/testing"
)

// Steg 3: utgiftsregistrering og kvitteringer.

func TestExpenseDeductiblePctDefault(t *testing.T) {
	h := apptest.Start(t)
	// Uten eksplisitt prosent skal kategoriens standard (100%) brukes.
	exp, err := h.App.AddExpense(h.Context(), core.ActorWeb, core.ExpenseInput{
		Date: "2025-03-01", Description: "Kontorrekvisita", Category: "kontorrekvisita",
		AmountNOK: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if exp.DeductiblePct != 100 || exp.DeductibleNok != 1000 {
		t.Errorf("standard fradrag feil: pct=%v nok=%v", exp.DeductiblePct, exp.DeductibleNok)
	}
}

func TestExpenseDeductiblePctOverride(t *testing.T) {
	h := apptest.Start(t)
	exp, err := h.App.AddExpense(h.Context(), core.ActorWeb, core.ExpenseInput{
		Date: "2025-03-01", Description: "Mobil (50% privat)", Category: "telefon_internett",
		AmountNOK: 1000, DeductiblePct: 50, HasDeductiblePct: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if exp.DeductibleNok != 500 {
		t.Errorf("fradragsberettiget = %v, forventet 500 (50%%)", exp.DeductibleNok)
	}
}

func TestExpenseViaFormWithReceipt(t *testing.T) {
	h := apptest.Start(t)
	b := h.Browser()
	png := onePixelPNG()
	res := b.PostMultipart("/expenses",
		map[string]string{
			"date":        "2025-04-02",
			"description": "Faglitteratur med kvittering",
			"amount_nok":  "750",
			"category":    "kurs_faglitteratur",
		},
		"receipt", "kvittering.png", "image/png", png)
	apptest.AssertStatus(t, res, 200)

	exps, _ := h.App.ListExpenses(h.Context(), 2025)
	if len(exps) != 1 {
		t.Fatalf("forventet 1 utgift, fikk %d", len(exps))
	}
	if !exps[0].ReceiptID.Valid {
		t.Fatal("utgiften mangler tilknyttet kvittering")
	}
	// Filen skal ligge under data/receipts/2025/.
	recs, _ := h.App.ListReceipts(h.Context())
	if len(recs) != 1 {
		t.Fatalf("forventet 1 kvittering, fikk %d", len(recs))
	}
	full := filepath.Join(h.DataDir, "receipts", recs[0].Filename)
	if _, err := os.Stat(full); err != nil {
		t.Errorf("kvitteringsfil ikke funnet paa %s: %v", full, err)
	}
	if filepath.Dir(recs[0].Filename) != "2025" {
		t.Errorf("kvittering lagret i feil mappe: %s (forventet 2025/...)", recs[0].Filename)
	}
}

func TestReceiptWrongTypeRejected(t *testing.T) {
	h := apptest.Start(t)
	_, err := h.App.SaveReceipt(h.Context(), core.ActorWeb, core.ReceiptInput{
		OriginalName: "ondsinnet.exe", MimeType: "application/x-msdownload",
		Data: []byte("MZ"), TaxYear: 2025,
	})
	if err == nil {
		t.Fatal("ugyldig filtype skulle gi feil")
	}
	if _, ok := core.AsValidation(err); !ok {
		t.Errorf("forventet valideringsfeil, fikk %T", err)
	}
}

func TestReceiptServedInline(t *testing.T) {
	h := apptest.Start(t)
	rec, err := h.App.SaveReceipt(h.Context(), core.ActorWeb, core.ReceiptInput{
		OriginalName: "bilde.png", MimeType: "image/png", Data: onePixelPNG(), TaxYear: 2025,
	})
	if err != nil {
		t.Fatal(err)
	}
	b := h.Browser()
	status, _, hdr := b.GetRaw("/receipts/file/" + itoa(rec.ID))
	apptest.AssertEqual(t, status, 200, "kvitteringsfil status")
	if ct := hdr.Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, forventet image/png", ct)
	}
}

func TestLinkReceiptToIncome(t *testing.T) {
	h := apptest.Start(t)
	inc, _ := h.App.AddIncome(h.Context(), core.ActorWeb, core.IncomeInput{
		Date: "2025-01-15", Description: "Tjeneste", Currency: "NOK",
		CountryCode: "NO", AmountOrig: 5000, Category: "tjenesteinntekt",
	})
	rec, _ := h.App.SaveReceipt(h.Context(), core.ActorWeb, core.ReceiptInput{
		OriginalName: "k.png", MimeType: "image/png", Data: onePixelPNG(), TaxYear: 2025,
	})
	if err := h.App.LinkReceipt(h.Context(), core.ActorWeb, "income", inc.Income.ID, rec.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := h.App.Q.GetIncome(h.Context(), inc.Income.ID)
	if !got.ReceiptID.Valid || got.ReceiptID.Int64 != rec.ID {
		t.Errorf("kvittering ikke knyttet: %+v", got.ReceiptID)
	}
	// Etter tilknytning skal kvitteringen ikke lenger telles som ubehandlet.
	unlinked, _ := h.App.ListUnlinkedReceipts(h.Context())
	if len(unlinked) != 0 {
		t.Errorf("forventet 0 ubehandlede kvitteringer, fikk %d", len(unlinked))
	}
}

// onePixelPNG returnerer en gyldig 1x1 PNG (fiktiv testfil).
func onePixelPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00,
		0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49,
		0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
