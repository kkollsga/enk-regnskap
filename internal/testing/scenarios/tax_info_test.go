package scenarios

import (
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/core"
	apptest "github.com/kkollsga/enk-regnskap/internal/testing"
)

// Steg 6: skatteinfo og landoversikt.

func TestTaxInfoShowsBrazilTaxTypes(t *testing.T) {
	h := apptest.Start(t)
	doc := h.Browser().Get("/tax-info")
	apptest.AssertStatus(t, doc, 200)
	// Alle brasilianske skattetyper fra seed-data.
	for _, code := range []string{"IRRF", "ISS", "CSLL", "PIS", "COFINS"} {
		apptest.AssertBodyContains(t, doc, code)
	}
	// Landoversikten gjelder kun utland – Norge (hjemstat) skal ikke vises.
	apptest.AssertBodyNotContains(t, doc, "TRYGDEAVGIFT")
}

func TestTaxInfoShowsTreatyDate(t *testing.T) {
	h := apptest.Start(t)
	doc := h.Browser().Get("/tax-info")
	// Ikrafttredelsesdato for skatteavtalen Norge-Brasil, lesbart format.
	apptest.AssertBodyContains(t, doc, "30. desember 2024")
	apptest.AssertBodyContains(t, doc, "Prop. 13 S (2022-2023)")
}

func TestTaxInfoShowsDeductionRates(t *testing.T) {
	h := apptest.Start(t)
	h.App.SetConfig(h.Context(), core.ConfigActiveYear, "2025")
	doc := h.Browser().Get("/tax-info")
	// Hjemmekontor sjablong 2025 = 2 192.
	apptest.AssertBodyContains(t, doc, "2 192")
	// Trygdeavgift næring 2025 = 10,9 %.
	apptest.AssertBodyContains(t, doc, "10,9")
}

func TestTaxInfoRatesPerYear(t *testing.T) {
	h := apptest.Start(t)
	h.App.SetConfig(h.Context(), core.ConfigActiveYear, "2024")
	doc := h.Browser().Get("/tax-info")
	// Hjemmekontor sjablong 2024 = 2 128.
	apptest.AssertBodyContains(t, doc, "2 128")
}
