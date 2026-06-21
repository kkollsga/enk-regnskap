package scenarios

import (
	"net/url"
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/core"
	apptest "github.com/kkollsga/enk-regnskap/internal/testing"
)

// Endringslogg og rollback via UI.

func TestChangelogListsMutations(t *testing.T) {
	h := apptest.Start(t)
	h.App.AddIncome(h.Context(), core.ActorWeb, core.IncomeInput{
		Date: "2025-01-10", Description: "Loggtest", Currency: "NOK",
		CountryCode: "NO", AmountOrig: 1000, Category: "tjenesteinntekt",
	})
	doc := h.Browser().Get("/changelog")
	apptest.AssertStatus(t, doc, 200)
	apptest.AssertBodyContains(t, doc, "Loggtest")
	apptest.AssertHas(t, doc, "form[action=/changelog/rollback]")
}

func TestRollbackViaUI(t *testing.T) {
	h := apptest.Start(t)
	h.App.SetConfig(h.Context(), core.ConfigActiveYear, "2025")
	inc, _ := h.App.AddIncome(h.Context(), core.ActorWeb, core.IncomeInput{
		Date: "2025-01-10", Description: "Skal angres", Currency: "NOK",
		CountryCode: "NO", AmountOrig: 1000, Category: "tjenesteinntekt",
	})
	_ = inc

	// Finn change-id for innsettingen.
	logs, _ := h.App.Q.ListChangeLog(h.Context(), 10)
	var id int64
	for _, l := range logs {
		if l.Operation == "insert" && l.Entity == "income" {
			id = l.ID
		}
	}
	b := h.Browser()
	res := b.PostForm("/changelog/rollback", url.Values{"id": {itoa(id)}})
	apptest.AssertStatus(t, res, 200)

	rows, _ := h.App.ListIncome(h.Context(), 2025)
	if len(rows) != 0 {
		t.Errorf("inntekt skulle vaere rullet tilbake, fikk %d", len(rows))
	}
}
