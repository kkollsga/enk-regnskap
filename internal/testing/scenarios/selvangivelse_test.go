package scenarios

import (
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/core"
	apptest "github.com/kkollsga/enk-regnskap/internal/testing"
)

func TestSelvangivelsePageRenders(t *testing.T) {
	h := apptest.Start(t)
	ctx := h.Context()
	h.App.SetConfig(ctx, core.ConfigActiveYear, "2025")
	h.Mock.AddRate("BRL", "2025-03-10", 2.00)
	// En BR-inntekt med IRRF (credit) gir RF-1147-seksjonen innhold.
	h.App.AddIncome(ctx, core.ActorWeb, core.IncomeInput{
		Date: "2025-03-10", Description: "BR", Currency: "BRL", CountryCode: "BR",
		AmountOrig: 10000, Category: "tjenesteinntekt", TaxYear: 2025,
		ForeignTaxPaid: core.ForeignTaxYes,
		ForeignTaxes:   []core.ForeignTaxLine{{Type: "IRRF", AmountOrig: 1500, Currency: "BRL"}},
	})
	doc := h.Browser().Get("/selvangivelse")
	apptest.AssertStatus(t, doc, 200)
	apptest.AssertBodyContains(t, doc, "Næringsspesifikasjon")
	apptest.AssertBodyContains(t, doc, "RF-1224")
	apptest.AssertBodyContains(t, doc, "RF-1147")
	apptest.AssertBodyContains(t, doc, "Estimert skatt")
}
