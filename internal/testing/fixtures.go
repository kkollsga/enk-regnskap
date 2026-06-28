package apptest

import (
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/core"
)

// Fixtures er et fast, FIKTIVT testdatasett for "Testforetak" (org.nr.
// 000000000). Brukes til å verifisere at beregninger stemmer ende-til-ende.
// Alle verdier er oppdiktet og ligner ikke reelle brukerdata.
//
// Datasettet (inntektsår 2025):
//   - 9 NOK-inntekter (sum 136 000 NOK)
//   - 3 BRL-inntekter til kurs 2,00 (sum 45 000 NOK): to med IRRF, en uten
//   - 8 utgifter (sum 38 892 NOK, hvorav 35 892 NOK fradragsberettiget)
const (
	FixtureYear          = 2025
	FixtureBRLRate       = 2.00
	FixtureIncome        = 181000.0 // total inntekt NOK
	FixtureForeign       = 45000.0  // utenlandsinntekt (BR) NOK
	FixtureForeignTaxNOK = 4500.0   // betalt brasiliansk skatt NOK (IRRF)
	FixtureDeductible    = 35892.0
	FixtureResult        = 145108.0 // inntekt - fradragsberettiget
)

type fixtureIncome struct {
	date, desc, currency, country, category string
	amount                                  float64
	ftPaid                                  int
	ftAmount                                float64
	ftType                                  string
}

type fixtureExpense struct {
	date, desc, category string
	amount               float64
	pct                  float64 // 0 => kategoristandard
}

func fixtureIncomes() []fixtureIncome {
	return []fixtureIncome{
		{"2025-01-15", "Konsulenttjeneste A", "NOK", "NO", "tjenesteinntekt", 10000, 0, 0, ""},
		{"2025-02-03", "Konsulenttjeneste B", "NOK", "NO", "tjenesteinntekt", 15000, 0, 0, ""},
		{"2025-02-20", "Honorar foredrag", "NOK", "NO", "honorar", 20000, 0, 0, ""},
		{"2025-03-10", "Konsulenttjeneste C", "NOK", "NO", "konsulent", 12000, 0, 0, ""},
		{"2025-04-05", "Lisensinntekt", "NOK", "NO", "royalty", 8000, 0, 0, ""},
		{"2025-05-12", "Stort oppdrag", "NOK", "NO", "tjenesteinntekt", 25000, 0, 0, ""},
		{"2025-06-01", "Småjobb", "NOK", "NO", "annet", 5000, 0, 0, ""},
		{"2025-09-15", "Konsulenttjeneste D", "NOK", "NO", "konsulent", 30000, 0, 0, ""},
		{"2025-11-20", "Honorar artikkel", "NOK", "NO", "honorar", 11000, 0, 0, ""},
		// Brasilianske inntekter (kurs 2,00):
		{"2025-03-18", "Brasiliansk klient 1", "BRL", "BR", "tjenesteinntekt", 10000, core.ForeignTaxYes, 1500, "IRRF"},
		{"2025-07-22", "Brasiliansk klient 2", "BRL", "BR", "tjenesteinntekt", 5000, core.ForeignTaxYes, 750, "IRRF"},
		{"2025-10-08", "Brasiliansk klient 3", "BRL", "BR", "honorar", 7500, core.ForeignTaxNo, 0, ""},
	}
}

func fixtureExpenses() []fixtureExpense {
	return []fixtureExpense{
		{"2025-01-31", "Hjemmekontor sjablong", "hjemmekontor", 2192, 0},
		{"2025-02-10", "Kontorrekvisita", "kontorrekvisita", 1500, 0},
		{"2025-03-01", "Mobil og bredband (50% privat)", "telefon_internett", 6000, 50},
		{"2025-04-12", "Reise til kunde", "reise", 4000, 0},
		{"2025-05-20", "Yrkeskjoring", "bil_km", 3500, 0},
		{"2025-06-15", "Fagkurs", "kurs_faglitteratur", 2500, 0},
		{"2025-08-01", "Regnskapsprogram (abonnement)", "regnskapsprogram", 1200, 0},
		{"2025-09-10", "Bærbar PC", "små_driftsmidler", 18000, 0},
	}
}

// LoadFixtures fyller databasen med testdatasettet og setter aktivt år.
func (h *Harness) LoadFixtures(t *testing.T) {
	t.Helper()
	ctx := h.Context()
	if err := h.App.SetConfig(ctx, core.ConfigBusinessName, "Testforetak"); err != nil {
		t.Fatal(err)
	}
	h.App.SetConfig(ctx, core.ConfigOrgNr, "000000000")
	h.App.SetConfig(ctx, core.ConfigActiveYear, "2025")

	for _, in := range fixtureIncomes() {
		if in.currency == "BRL" {
			h.Mock.AddRate("BRL", in.date, FixtureBRLRate)
		}
		var taxes []core.ForeignTaxLine
		if in.ftPaid == core.ForeignTaxYes && in.ftAmount > 0 {
			taxes = []core.ForeignTaxLine{{Type: in.ftType, AmountOrig: in.ftAmount, Currency: in.currency}}
		}
		_, err := h.App.AddIncome(ctx, core.ActorSystem, core.IncomeInput{
			Date: in.date, Description: in.desc, Currency: in.currency,
			CountryCode: in.country, Category: in.category, AmountOrig: in.amount,
			TaxYear: FixtureYear, ForeignTaxPaid: in.ftPaid, ForeignTaxes: taxes,
		})
		if err != nil {
			t.Fatalf("fixture inntekt %q: %v", in.desc, err)
		}
	}
	for _, ex := range fixtureExpenses() {
		input := core.ExpenseInput{
			Date: ex.date, Description: ex.desc, Category: ex.category, AmountNOK: ex.amount,
			TaxYear: FixtureYear,
		}
		if ex.pct > 0 {
			input.DeductiblePct = ex.pct
			input.HasDeductiblePct = true
		}
		if _, err := h.App.AddExpense(ctx, core.ActorSystem, input); err != nil {
			t.Fatalf("fixture utgift %q: %v", ex.desc, err)
		}
	}
}
