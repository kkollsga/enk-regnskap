package scenarios

import (
	"net/url"
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/core"
	apptest "github.com/kkollsga/enk-regnskap/internal/testing"
)

// Steg 1: første oppstart / onboarding.

func TestFreshAppRedirectsToWelcome(t *testing.T) {
	h := apptest.StartRaw(t)
	doc := h.Browser().Get("/")
	apptest.AssertStatus(t, doc, 200)
	// Etter redirect skal velkomstskjermen vises.
	apptest.AssertBodyContains(t, doc, "Velkommen til ENK Regnskap")
}

func TestOnboardingCompletes(t *testing.T) {
	h := apptest.StartRaw(t)
	b := h.Browser()

	res := b.PostForm("/onboard", url.Values{
		"business_name": {"Testforetak"},
		"org_nr":        {"000000000"},
		"language":      {"nb"},
	})
	apptest.AssertStatus(t, res, 200)

	if !h.App.IsOnboarded(h.Context()) {
		t.Fatal("appen ble ikke markert som onboardet")
	}
	if got := h.App.GetConfig(h.Context(), core.ConfigBusinessName, ""); got != "Testforetak" {
		t.Errorf("business_name = %q, forventet Testforetak", got)
	}
	// Etter onboarding skal forsiden vise dashbordet (med nav).
	doc := b.Get("/")
	apptest.AssertHas(t, doc, "nav")
	apptest.AssertBodyContains(t, doc, "Oversikt")
}

func TestDatabaseAndFoldersCreated(t *testing.T) {
	h := apptest.StartRaw(t)
	// Databasen er opprettet (seed-data tilgjengelig).
	countries, err := h.App.Countries(h.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(countries) < 2 {
		t.Errorf("forventet minst Norge og Brasil i seed, fikk %d", len(countries))
	}
}

func TestOnboardedAppDoesNotRedirect(t *testing.T) {
	h := apptest.Start(t) // allerede onboardet
	status, _, _ := h.Browser().GetRaw("/income/new")
	apptest.AssertEqual(t, status, 200, "onboardet app skal nå /income/new direkte")
}
