package scenarios

import (
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/core"
	apptest "github.com/kkollsga/enk-regnskap/internal/testing"
)

// Redigering av inntekt/utgift + flere vedlegg per post.

func TestEditIncomeViaForm(t *testing.T) {
	h := apptest.Start(t)
	inc, _ := h.App.AddIncome(h.Context(), core.ActorWeb, core.IncomeInput{
		Date: "2025-01-10", Description: "Opprinnelig", Currency: "NOK",
		CountryCode: "NO", AmountOrig: 5000, Category: "tjenesteinntekt",
	})
	id := inc.Income.ID

	// Endre via POST /income/{id}.
	res := h.Browser().PostForm("/income/"+strconv.FormatInt(id, 10), url.Values{
		"date": {"2025-01-10"}, "description": {"Endret beskrivelse"},
		"currency": {"NOK"}, "country_code": {"NO"}, "amount_orig": {"7500"},
		"category": {"honorar"}, "foreign_tax_paid": {"0"},
	})
	apptest.AssertStatus(t, res, 200)

	got, _ := h.App.GetIncome(h.Context(), id)
	if got.Description != "Endret beskrivelse" || got.AmountNok != 7500 || got.Category != "honorar" {
		t.Errorf("inntekt ikke oppdatert: %+v", got)
	}
	// Fortsatt bare én rad (oppdatering, ikke ny).
	rows, _ := h.App.ListIncome(h.Context(), 2025)
	if len(rows) != 1 {
		t.Errorf("forventet 1 inntekt etter redigering, fikk %d", len(rows))
	}
}

func TestEditExpenseViaForm(t *testing.T) {
	h := apptest.Start(t)
	exp, _ := h.App.AddExpense(h.Context(), core.ActorWeb, core.ExpenseInput{
		Date: "2025-02-01", Description: "PC", Category: "små_driftsmidler", AmountNOK: 10000,
	})
	res := h.Browser().PostForm("/expenses/"+strconv.FormatInt(exp.ID, 10), url.Values{
		"date": {"2025-02-01"}, "description": {"PC (oppdatert)"},
		"amount_nok": {"12000"}, "category": {"små_driftsmidler"},
	})
	apptest.AssertStatus(t, res, 200)
	got, _ := h.App.GetExpense(h.Context(), exp.ID)
	if got.Description != "PC (oppdatert)" || got.AmountNok != 12000 {
		t.Errorf("utgift ikke oppdatert: %+v", got)
	}
}

func TestEditFormPrefilled(t *testing.T) {
	h := apptest.Start(t)
	inc, _ := h.App.AddIncome(h.Context(), core.ActorWeb, core.IncomeInput{
		Date: "2025-03-03", Description: "Forhåndsutfylt", Currency: "NOK",
		CountryCode: "NO", AmountOrig: 4321, Category: "konsulent",
	})
	doc := h.Browser().Get("/income/" + strconv.FormatInt(inc.Income.ID, 10) + "/edit")
	apptest.AssertStatus(t, doc, 200)
	apptest.AssertBodyContains(t, doc, "Forhåndsutfylt")
	apptest.AssertBodyContains(t, doc, "4321")
	// Skjemaet skal poste til oppdaterings-URL.
	apptest.AssertHas(t, doc, "form[action=/income/"+strconv.FormatInt(inc.Income.ID, 10)+"]")
}

func TestInlineEditFormOnList(t *testing.T) {
	h := apptest.Start(t)
	h.App.SetConfig(h.Context(), core.ConfigActiveYear, "2025")
	inc, _ := h.App.AddIncome(h.Context(), core.ActorWeb, core.IncomeInput{
		Date: "2025-03-03", Description: "Inline-post", Currency: "NOK",
		CountryCode: "NO", AmountOrig: 4321, Category: "konsulent",
	})
	doc := h.Browser().Get("/income")
	apptest.AssertStatus(t, doc, 200)
	// Inline-redigeringsskjemaet ligger i listen og poster til oppdaterings-URL.
	apptest.AssertHas(t, doc, "form[action=/income/"+strconv.FormatInt(inc.Income.ID, 10)+"]")
	apptest.AssertBodyContains(t, doc, "js-edit-toggle")
	apptest.AssertBodyContains(t, doc, "4321") // forhåndsutfylt beløp
}

func TestAttachmentListingHasThumbAndCroppedDesc(t *testing.T) {
	h := apptest.Start(t)
	h.App.SetConfig(h.Context(), core.ConfigActiveYear, "2025")
	longDesc := strings.Repeat("a", 250)
	res := h.Browser().PostMultipartFiles("/expenses",
		map[string]string{
			"date": "2025-05-01", "description": "Med vedlegg",
			"amount_nok": "500", "category": "kontorrekvisita",
		},
		"attachment",
		[]apptest.UploadFile{{Name: "kvittering.png", ContentType: "image/png", Data: onePixelPNG()}},
		map[string][]string{
			"attachment_title": {"Kvittering"},
			"attachment_desc":  {longDesc},
		})
	apptest.AssertStatus(t, res, 200)

	doc := h.Browser().Get("/expenses")
	// Miniatyrbilde + tittel vises i listevisningen.
	apptest.AssertHas(t, doc, "a.attach-row")
	apptest.AssertBodyContains(t, doc, "Kvittering")
	// Undertittelen (beskrivelsen) kuttes til 200 tegn med ellipsis.
	apptest.AssertHas(t, doc, ".attach-sub")
	apptest.AssertHTMLContains(t, doc, ".attach-sub", "…")
	sub := strings.Repeat("a", 200) + "…"
	apptest.AssertHTMLContains(t, doc, ".attach-sub", sub)
}

func TestMultipleAttachmentsPerEntry(t *testing.T) {
	h := apptest.Start(t)
	b := h.Browser()
	// To vedlegg i samme post.
	res := b.PostMultipartFiles("/income",
		map[string]string{
			"date": "2025-04-01", "description": "To vedlegg", "currency": "NOK",
			"country_code": "NO", "amount_orig": "9000", "category": "tjenesteinntekt",
			"foreign_tax_paid": "0",
		},
		"attachment",
		[]apptest.UploadFile{
			{Name: "a.png", ContentType: "image/png", Data: onePixelPNG()},
			{Name: "b.png", ContentType: "image/png", Data: onePixelPNG()},
		},
		map[string][]string{
			"attachment_title": {"Faktura", "Kvittering"},
			"attachment_desc":  {"del 1", "del 2"},
		})
	apptest.AssertStatus(t, res, 200)

	rows, _ := h.App.ListIncome(h.Context(), 2025)
	att, _ := h.App.ReceiptsFor(h.Context(), "income", rows[0].ID)
	if len(att) != 2 {
		t.Fatalf("forventet 2 vedlegg, fikk %d", len(att))
	}
	if att[0].Title.String != "Faktura" || att[1].Title.String != "Kvittering" {
		t.Errorf("vedleggstitler feil: %q, %q", att[0].Title.String, att[1].Title.String)
	}
}
