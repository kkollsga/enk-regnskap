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
		AmountOrig: 1000,
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
		AmountOrig: 1000, DeductiblePct: 50, HasDeductiblePct: true,
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
			"amount_orig": "750",
			"category":    "kurs_faglitteratur",
		},
		"attachment", "kvittering.png", "image/png", png)
	apptest.AssertStatus(t, res, 200)

	exps, _ := h.App.ListExpenses(h.Context(), 2025)
	if len(exps) != 1 {
		t.Fatalf("forventet 1 utgift, fikk %d", len(exps))
	}
	att, _ := h.App.ReceiptsFor(h.Context(), "expense", exps[0].ID)
	if len(att) != 1 {
		t.Fatal("utgiften mangler tilknyttet vedlegg")
	}
	// Filen skal ligge under data/receipts/2025/.
	recs, _ := h.App.ListReceipts(h.Context())
	if len(recs) != 1 {
		t.Fatalf("forventet 1 kvittering, fikk %d", len(recs))
	}
	full := filepath.Join(h.DataDir, "receipts", recs[0].Filename)
	if _, err := os.Stat(full); err != nil {
		t.Errorf("kvitteringsfil ikke funnet på %s: %v", full, err)
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

func TestReceiptLinkedToParentAndEdited(t *testing.T) {
	h := apptest.Start(t)
	inc, _ := h.App.AddIncome(h.Context(), core.ActorWeb, core.IncomeInput{
		Date: "2025-01-15", Description: "Tjeneste", Currency: "NOK",
		CountryCode: "NO", AmountOrig: 5000, Category: "tjenesteinntekt",
	})
	rec, err := h.App.SaveReceipt(h.Context(), core.ActorWeb, core.ReceiptInput{
		OriginalName: "k.png", MimeType: "image/png", Data: onePixelPNG(),
		ParentKind: "income", ParentID: inc.Income.ID, TaxYear: 2025,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Vedlegget skal være knyttet til inntekten.
	att, _ := h.App.ReceiptsFor(h.Context(), "income", inc.Income.ID)
	if len(att) != 1 || att[0].ID != rec.ID {
		t.Fatalf("vedlegg ikke knyttet til inntekt: %+v", att)
	}
	// Rediger tittel + beskrivelse.
	if err := h.App.UpdateReceiptMeta(h.Context(), core.ActorWeb, rec.ID, "Faktura mars", "Konsulenttjeneste"); err != nil {
		t.Fatal(err)
	}
	updated, _ := h.App.GetReceipt(h.Context(), rec.ID)
	if updated.Title.String != "Faktura mars" || updated.Description.String != "Konsulenttjeneste" {
		t.Errorf("tittel/beskrivelse ikke lagret: %+v", updated)
	}
	// Slett vedlegget.
	if err := h.App.DeleteReceipt(h.Context(), core.ActorWeb, rec.ID); err != nil {
		t.Fatal(err)
	}
	att, _ = h.App.ReceiptsFor(h.Context(), "income", inc.Income.ID)
	if len(att) != 0 {
		t.Errorf("vedlegg ble ikke slettet, fikk %d", len(att))
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

// TestExpenseRejectedWhenYearDiffersFromActive sikrer at en post med dato i et
// annet år enn det valgte inntektsåret avvises (ikke lagres), så den ikke
// «forsvinner» i et annet års liste.
func TestExpenseRejectedWhenYearDiffersFromActive(t *testing.T) {
	h := apptest.Start(t)
	ctx := h.Context()
	h.App.SetConfig(ctx, core.ConfigActiveYear, "2025")
	h.Browser().Get("/expenses/new").Form("/expenses").
		Set("date", "2026-06-28").Set("description", "Kontorrekvisita").
		Set("amount_orig", "500").Set("category", "kontorrekvisita").Submit()
	a25, _ := h.App.ListExpenses(ctx, 2025)
	a26, _ := h.App.ListExpenses(ctx, 2026)
	if len(a25)+len(a26) != 0 {
		t.Errorf("forventet 0 utgifter (avvist), fikk 2025=%d 2026=%d", len(a25), len(a26))
	}
	if y := h.App.ActiveYear(ctx); y != 2025 {
		t.Errorf("aktivt år skal være uendret 2025, var %d", y)
	}
}

// TestExpenseForeignCurrencyConverted verifiserer at en utgift i utenlandsk
// valuta konverteres til NOK på utgiftsdatoen, og at valuta/land lagres.
func TestExpenseForeignCurrencyConverted(t *testing.T) {
	h := apptest.Start(t)
	ctx := h.Context()
	h.App.SetConfig(ctx, core.ConfigActiveYear, "2025")
	h.Mock.AddRate("BRL", "2025-03-10", 2.00)
	exp, err := h.App.AddExpense(ctx, core.ActorWeb, core.ExpenseInput{
		Date: "2025-03-10", Description: "Materiell i Brasil", Category: "kontorrekvisita",
		Currency: "BRL", CountryCode: "BR", AmountOrig: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if exp.Currency != "BRL" || exp.CountryCode != "BR" {
		t.Errorf("valuta/land ikke lagret: currency=%q country=%q", exp.Currency, exp.CountryCode)
	}
	if exp.AmountOrig != 1000 {
		t.Errorf("amount_orig = %v, forventet 1000", exp.AmountOrig)
	}
	if exp.AmountNok != 2000 {
		t.Errorf("amount_nok = %v, forventet 2000 (1000 * 2.00)", exp.AmountNok)
	}
	if exp.DeductibleNok != 2000 {
		t.Errorf("fradrag = %v, forventet 2000 (100%%)", exp.DeductibleNok)
	}
}

// TestForeignTaxExpenseCategoriesAreBR sikrer at de brasilianske skatte-
// kategoriene er merket med land BR (for landfiltrering i skjemaet).
func TestForeignTaxExpenseCategoriesAreBR(t *testing.T) {
	for _, c := range core.ForeignTaxExpenseCategories() {
		if c.Country != "BR" {
			t.Errorf("kategori %q har land %q, forventet BR", c.Key, c.Country)
		}
	}
}

// TestExpenseLinkedToIncome verifiserer gruppering av utgift under inntekt, og
// at feil land/valuta avvises.
func TestExpenseLinkedToIncome(t *testing.T) {
	h := apptest.Start(t)
	ctx := h.Context()
	h.App.SetConfig(ctx, core.ConfigActiveYear, "2025")
	h.Mock.AddRate("BRL", "2025-03-10", 2.00)
	inc, _ := h.App.AddIncome(ctx, core.ActorWeb, core.IncomeInput{
		Date: "2025-03-10", Description: "BR inntekt", Currency: "BRL", CountryCode: "BR",
		AmountOrig: 10000, Category: "tjenesteinntekt", TaxYear: 2025,
	})
	id := inc.Income.ID
	exp, err := h.App.AddExpense(ctx, core.ActorWeb, core.ExpenseInput{
		Date: "2025-03-11", Description: "Kostnad i Brasil", Category: "kontorrekvisita",
		Currency: "BRL", CountryCode: "BR", AmountOrig: 500, IncomeID: &id,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !exp.IncomeID.Valid || exp.IncomeID.Int64 != id {
		t.Errorf("utgift ble ikke koblet til inntekt")
	}
	linked, _ := h.App.ExpensesForIncome(ctx, id)
	if len(linked) != 1 {
		t.Errorf("forventet 1 koblet utgift, fikk %d", len(linked))
	}
	// Feil valuta (NOK vs inntektens BRL) skal avvises.
	_, err = h.App.AddExpense(ctx, core.ActorWeb, core.ExpenseInput{
		Date: "2025-03-12", Description: "NOK-kostnad", Category: "kontorrekvisita",
		Currency: "NOK", CountryCode: "BR", AmountOrig: 100, IncomeID: &id,
	})
	if err == nil {
		t.Errorf("forventet feil ved valuta-avvik")
	} else if _, ok := core.AsValidation(err); !ok {
		t.Errorf("forventet valideringsfeil, fikk %T", err)
	}
}
