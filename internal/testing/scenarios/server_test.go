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

	// Alle planlagte sider skal vaere lenket i navigasjonen.
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
	// Forsiden skal vise inntektskortet (live-element) selv naar tomt.
	apptest.AssertHas(t, doc, ".card-value")
	apptest.AssertHTMLContains(t, doc, ".card-value", "0")
}

func TestStubsReturn501(t *testing.T) {
	h := apptest.Start(t)
	b := h.Browser()
	for _, path := range []string{"/expenses", "/receipts", "/reports"} {
		status, _, _ := b.GetRaw(path)
		apptest.AssertEqual(t, status, 501, "stub "+path)
	}
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
