package scenarios

import (
	"net/url"
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/core"
	apptest "github.com/kkollsga/enk-regnskap/internal/testing"
)

// Steg 2: inntektsregistrering.

func TestIncomeFormHasRequiredFields(t *testing.T) {
	h := apptest.Start(t)
	doc := h.Browser().Get("/income/new")
	apptest.AssertStatus(t, doc, 200)
	for _, id := range []string{"#date", "#description", "#amount_orig", "#category", "#currency", "#country_code"} {
		apptest.AssertHas(t, doc, id)
	}
}

func TestCreateNOKIncome(t *testing.T) {
	h := apptest.Start(t)
	b := h.Browser()
	doc := b.Get("/income/new")
	res := doc.Form("/income").
		Set("date", "2025-03-10").
		Set("description", "Konsulenttjeneste").
		Set("client", "Acme AS").
		Set("currency", "NOK").
		Set("country_code", "NO").
		Set("amount_orig", "12500").
		Set("category", "tjenesteinntekt").
		Submit()
	apptest.AssertStatus(t, res, 200)

	rows, err := h.App.ListIncome(h.Context(), 2025)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("forventet 1 inntekt, fikk %d", len(rows))
	}
	if rows[0].AmountNok != 12500 {
		t.Errorf("amount_nok = %v, forventet 12500", rows[0].AmountNok)
	}
	if rows[0].Currency != "NOK" {
		t.Errorf("currency = %q", rows[0].Currency)
	}
}

func TestCreateBRLIncomeConvertsToNOK(t *testing.T) {
	h := apptest.Start(t)
	// Mock-kurs: 1 BRL = 1.90 NOK på inntektsdatoen.
	h.Mock.AddRate("BRL", "2025-04-15", 1.90)
	b := h.Browser()
	doc := b.Get("/income/new")
	res := doc.Form("/income").
		Set("date", "2025-04-15").
		Set("description", "Tjeneste til brasiliansk klient").
		Set("currency", "BRL").
		Set("country_code", "BR").
		Set("amount_orig", "10000").
		Set("category", "tjenesteinntekt").
		Set("foreign_tax_paid", "0").
		Submit()
	apptest.AssertStatus(t, res, 200)

	rows, _ := h.App.ListIncome(h.Context(), 2025)
	if len(rows) != 1 {
		t.Fatalf("forventet 1 inntekt, fikk %d", len(rows))
	}
	in := rows[0]
	if !in.ExchangeRate.Valid || in.ExchangeRate.Float64 != 1.90 {
		t.Errorf("exchange_rate = %+v, forventet 1.90", in.ExchangeRate)
	}
	// 10000 BRL * 1.90 = 19000 NOK
	if in.AmountNok != 19000 {
		t.Errorf("amount_nok = %v, forventet 19000", in.AmountNok)
	}
	if !in.RateDate.Valid || in.RateDate.String != "2025-04-15" {
		t.Errorf("rate_date = %+v, forventet 2025-04-15", in.RateDate)
	}
}

func TestBRLIncomeWithForeignTax(t *testing.T) {
	h := apptest.Start(t)
	h.Mock.AddRate("BRL", "2025-05-20", 2.00)
	b := h.Browser()
	doc := b.Get("/income/new")
	res := doc.Form("/income").
		Set("date", "2025-05-20").
		Set("description", "Brasiliansk honorar med IRRF").
		Set("currency", "BRL").
		Set("country_code", "BR").
		Set("amount_orig", "10000").
		Set("category", "honorar").
		Set("foreign_tax_paid", "1").
		Set("foreign_tax_orig", "1500").
		Set("foreign_tax_type", "IRRF").
		Set("foreign_tax_currency", "BRL").
		Submit()
	apptest.AssertStatus(t, res, 200)

	rows, _ := h.App.ListIncome(h.Context(), 2025)
	in := rows[0]
	if in.ForeignTaxPaid != 1 {
		t.Errorf("foreign_tax_paid = %d, forventet 1", in.ForeignTaxPaid)
	}
	// 1500 BRL * 2.00 = 3000 NOK
	if !in.ForeignTaxNok.Valid || in.ForeignTaxNok.Float64 != 3000 {
		t.Errorf("foreign_tax_nok = %+v, forventet 3000", in.ForeignTaxNok)
	}
	if in.ForeignTaxType.String != "IRRF" {
		t.Errorf("foreign_tax_type = %q", in.ForeignTaxType.String)
	}

	// Kreditfradrag skal være aggregert for BR/2025 med rettsgrunnlag 'treaty'.
	credit, err := h.App.ForeignTaxForYear(h.Context(), 2025)
	if err != nil {
		t.Fatal(err)
	}
	if len(credit) != 1 {
		t.Fatalf("forventet 1 kreditfradrag, fikk %d", len(credit))
	}
	if credit[0].Credit.LegalBasis.String != core.LegalBasisTreaty {
		t.Errorf("legal_basis = %q, forventet treaty (2025)", credit[0].Credit.LegalBasis.String)
	}
	if credit[0].Credit.IncomeNok != 20000 {
		t.Errorf("aggregert inntekt = %v, forventet 20000", credit[0].Credit.IncomeNok)
	}
}

func TestForeignTaxLegalBasis2024IsInternal(t *testing.T) {
	h := apptest.Start(t)
	h.Mock.AddRate("BRL", "2024-06-10", 2.00)
	b := h.Browser()
	b.Get("/income/new").Form("/income").
		Set("date", "2024-06-10").
		Set("description", "Brasiliansk inntekt 2024").
		Set("currency", "BRL").
		Set("country_code", "BR").
		Set("amount_orig", "5000").
		Set("category", "tjenesteinntekt").
		Set("foreign_tax_paid", "1").
		Set("foreign_tax_orig", "750").
		Set("foreign_tax_type", "IRRF").
		Set("foreign_tax_currency", "BRL").
		Submit()

	credit, _ := h.App.ForeignTaxForYear(h.Context(), 2024)
	if len(credit) != 1 {
		t.Fatalf("forventet 1 kreditfradrag for 2024, fikk %d", len(credit))
	}
	if credit[0].Credit.LegalBasis.String != core.LegalBasisInternal {
		t.Errorf("legal_basis 2024 = %q, forventet internal (ingen avtale)", credit[0].Credit.LegalBasis.String)
	}
}

func TestIncomeWithReceipt(t *testing.T) {
	h := apptest.Start(t)
	res := h.Browser().PostMultipart("/income",
		map[string]string{
			"date": "2025-03-10", "description": "Tjeneste med kvittering",
			"currency": "NOK", "country_code": "NO", "amount_orig": "5000",
			"category": "tjenesteinntekt", "foreign_tax_paid": "0",
		},
		"attachment", "faktura.png", "image/png", onePixelPNG())
	apptest.AssertStatus(t, res, 200)

	rows, _ := h.App.ListIncome(h.Context(), 2025)
	if len(rows) != 1 {
		t.Fatalf("forventet 1 inntekt, fikk %d", len(rows))
	}
	att, _ := h.App.ReceiptsFor(h.Context(), "income", rows[0].ID)
	if len(att) != 1 {
		t.Errorf("inntekten mangler tilknyttet vedlegg, fikk %d", len(att))
	}
}

func TestIncomeValidationShownInline(t *testing.T) {
	h := apptest.Start(t)
	b := h.Browser()
	// Tomt skjema (mangler beskrivelse, beløp=0, ingen kategori).
	res := b.PostForm("/income", url.Values{
		"date":         {"2025-01-10"},
		"description":  {""},
		"currency":     {"NOK"},
		"country_code": {"NO"},
		"amount_orig":  {"0"},
		"category":     {""},
	})
	// Siden skal IKKE redirecte; den viser skjemaet med feil.
	apptest.AssertStatus(t, res, 200)
	apptest.AssertHas(t, res, ".field-error")
	apptest.AssertBodyContains(t, res, "Beskrivelse er påkrevd.")

	rows, _ := h.App.ListIncome(h.Context(), 2025)
	if len(rows) != 0 {
		t.Errorf("ugyldig inntekt ble lagret (%d rader)", len(rows))
	}
}

func TestIncomeRollback(t *testing.T) {
	h := apptest.Start(t)
	res, err := h.App.AddIncome(h.Context(), core.ActorWeb, core.IncomeInput{
		Date: "2025-02-02", Description: "Skal angres", Currency: "NOK",
		CountryCode: "NO", AmountOrig: 5000, Category: "tjenesteinntekt",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Finn endringsloggen for innsettingen.
	logs, err := h.App.Q.ListChangeLog(h.Context(), 10)
	if err != nil {
		t.Fatal(err)
	}
	var changeID int64
	for _, l := range logs {
		if l.Entity == "income" && l.Operation == "insert" {
			changeID = l.ID
			break
		}
	}
	if changeID == 0 {
		t.Fatal("fant ingen insert-logg for inntekt")
	}
	_ = res

	if err := h.App.Rollback(h.Context(), core.ActorWeb, changeID); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	rows, _ := h.App.ListIncome(h.Context(), 2025)
	if len(rows) != 0 {
		t.Errorf("etter rollback skulle inntekten være borte, fikk %d", len(rows))
	}
}
