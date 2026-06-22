package scenarios

import (
	"strings"
	"testing"

	apptest "github.com/kkollsga/enk-regnskap/internal/testing"
)

// Steg 1 (delvis): grunnleggende app starter.

func TestHealthEndpoint(t *testing.T) {
	h := apptest.Start(t)
	status, body, _ := h.Browser().GetRaw("/health")
	apptest.AssertEqual(t, status, 200, "health status")
	if !strings.Contains(body, `"status":"ok"`) {
		t.Errorf("health body = %q, forventet status ok", body)
	}
}

func TestFrontPageHasNav(t *testing.T) {
	h := apptest.Start(t)
	doc := h.Browser().Get("/")
	apptest.AssertStatus(t, doc, 200)
	apptest.AssertHas(t, doc, "nav")

	// Alle planlagte sider skal være lenket i navigasjonen.
	wantLinks := []string{"/", "/income", "/expenses", "/receipts",
		"/foreign-tax", "/tax-info", "/reports", "/changelog"}
	for _, href := range wantLinks {
		found := false
		for _, a := range doc.Find("a") {
			if apptest.Attr(a, "href") == href {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("navigasjonen mangler lenke til %s", href)
		}
	}
}

func TestDashboardShowsEmptyTotals(t *testing.T) {
	h := apptest.Start(t)
	doc := h.Browser().Get("/")
	apptest.AssertStatus(t, doc, 200)
	// Forsiden skal vise inntektskortet (live-element) selv når tomt.
	apptest.AssertHas(t, doc, ".card-value")
	apptest.AssertHTMLContains(t, doc, ".card-value", "0")
}

func TestStaticAssetsServed(t *testing.T) {
	h := apptest.Start(t)
	status, body, hdr := h.Browser().GetRaw("/static/style.css")
	apptest.AssertEqual(t, status, 200, "style.css status")
	if !strings.Contains(body, "--primary") {
		t.Error("style.css mangler forventet innhold")
	}
	_ = hdr
}
