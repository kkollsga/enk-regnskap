package core

import (
	"context"
	"fmt"

	"github.com/kkollsga/enk-regnskap/internal/db"
)

// dummyPNG er en gyldig 1x1 PNG som brukes som eksempelkvittering i testdata.
var dummyPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00,
	0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49,
	0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
}

const dummyBRLRate = 1.85

// GenerateDummyData fyller appen med et realistisk, FIKTIVT datasett for det
// aktive inntektsåret (12 inntekter inkl. brasilianske med IRRF, 8 utgifter
// og en eksempelkvittering). Alt går via de vanlige (reviderte) kjernekallene,
// så endringslogg, mirror og live oppdatering oppdateres normalt.
// Returnerer antall opprettede transaksjoner.
func (a *App) GenerateDummyData(ctx context.Context, actor string) (int, error) {
	year := a.ActiveYear(ctx)
	d := func(mmdd string) string { return fmt.Sprintf("%d-%s", year, mmdd) }

	// Seed valutakurser i cachen for BRL-datoene slik at generering fungerer
	// uten nett.
	for _, mmdd := range []string{"03-18", "07-22", "10-08"} {
		_ = a.Q.UpsertExchangeRate(ctx, db.UpsertExchangeRateParams{
			Currency: "BRL", Date: d(mmdd), RateNok: dummyBRLRate, Source: "testdata",
		})
	}

	type inc struct {
		mmdd, desc, cur, country, cat, client string
		amount                                float64
		ftPaid                                int
		ftAmount                              float64
		ftType                                string
	}
	incomes := []inc{
		{"01-15", "Konsulenttjeneste", "NOK", "NO", "tjenesteinntekt", "Nordic Tech AS", 10000, 0, 0, ""},
		{"02-03", "Webutvikling", "NOK", "NO", "tjenesteinntekt", "Fjord Design AS", 15000, 0, 0, ""},
		{"02-20", "Foredrag", "NOK", "NO", "honorar", "Bergen Kommune", 20000, 0, 0, ""},
		{"03-10", "Radgivning", "NOK", "NO", "konsulent", "Nordic Tech AS", 12000, 0, 0, ""},
		{"04-05", "Lisensinntekt app", "NOK", "NO", "royalty", "App Store", 8000, 0, 0, ""},
		{"05-12", "Stort prosjekt", "NOK", "NO", "tjenesteinntekt", "Equinor ASA", 25000, 0, 0, ""},
		{"06-01", "Småjobb", "NOK", "NO", "annet", "", 5000, 0, 0, ""},
		{"09-15", "Konsulentoppdrag", "NOK", "NO", "konsulent", "Fjord Design AS", 30000, 0, 0, ""},
		{"11-20", "Fagartikkel", "NOK", "NO", "honorar", "Teknisk Ukeblad", 11000, 0, 0, ""},
		// Brasilianske inntekter (kurs 1,85):
		{"03-18", "Tjeneste til brasiliansk klient", "BRL", "BR", "tjenesteinntekt", "Sao Paulo Digital", 10000, ForeignTaxYes, 1500, "IRRF"},
		{"07-22", "Honorar Brasil", "BRL", "BR", "tjenesteinntekt", "Rio Web Ltda", 5000, ForeignTaxYes, 750, "IRRF"},
		{"10-08", "Brasiliansk oppdrag", "BRL", "BR", "honorar", "Bahia Studio", 7500, ForeignTaxNo, 0, ""},
	}

	count := 0
	for _, in := range incomes {
		_, err := a.AddIncome(ctx, actor, IncomeInput{
			Date: d(in.mmdd), Description: in.desc, Currency: in.cur, CountryCode: in.country,
			Category: in.cat, Client: in.client, AmountOrig: in.amount, TaxYear: year,
			ForeignTaxPaid: in.ftPaid, ForeignTaxOrig: in.ftAmount, ForeignTaxCurrency: "BRL",
			ForeignTaxType: in.ftType,
		})
		if err != nil {
			return count, fmt.Errorf("testinntekt %q: %w", in.desc, err)
		}
		count++
	}

	// En eksempelkvittering, knyttet til PC-kjøpet.
	var receiptID *int64
	if rec, err := a.SaveReceipt(ctx, actor, ReceiptInput{
		OriginalName: "kvittering-pc.png", MimeType: "image/png", Data: dummyPNG, TaxYear: year,
	}); err == nil {
		receiptID = &rec.ID
	}

	type exp struct {
		mmdd, desc, cat string
		amount, pct     float64
		withReceipt     bool
	}
	expenses := []exp{
		{"01-31", "Hjemmekontor (sjablong)", "hjemmekontor", 2192, 0, false},
		{"02-10", "Kontorrekvisita", "kontorrekvisita", 1500, 0, false},
		{"03-01", "Mobil og bredband", "telefon_internett", 6000, 50, false},
		{"04-12", "Reise til kunde", "reise", 4000, 0, false},
		{"05-20", "Yrkeskjoring", "bil_km", 3500, 0, false},
		{"06-15", "Fagkurs", "kurs_faglitteratur", 2500, 0, false},
		{"08-01", "Regnskapsprogram", "regnskapsprogram", 1200, 0, false},
		{"09-10", "Barbar PC", "små_driftsmidler", 18000, 0, true},
	}
	for _, ex := range expenses {
		input := ExpenseInput{
			Date: d(ex.mmdd), Description: ex.desc, Category: ex.cat, AmountNOK: ex.amount, TaxYear: year,
		}
		if ex.pct > 0 {
			input.DeductiblePct = ex.pct
			input.HasDeductiblePct = true
		}
		if ex.withReceipt {
			input.ReceiptID = receiptID
		}
		if _, err := a.AddExpense(ctx, actor, input); err != nil {
			return count, fmt.Errorf("testutgift %q: %w", ex.desc, err)
		}
		count++
	}
	return count, nil
}
