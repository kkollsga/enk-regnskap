// Package apptest er appens eget lettvekts ende-til-ende-testbibliotek. Det
// starter appen på en tilfeldig port, simulerer nettleserinteraksjon over
// HTTP, og tilbyr enkle assertions. Ikke Playwright/Selenium - bare net/http
// og golang.org/x/net/html.
package apptest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kkollsga/enk-regnskap/internal/core"
	"github.com/kkollsga/enk-regnskap/internal/server"
	"github.com/kkollsga/enk-regnskap/internal/testing/mocks"
)

// Harness holder en kjorende app-instans for en test.
type Harness struct {
	App     *core.App
	Mock    *mocks.NorgesBankMock
	Server  *httptest.Server
	BaseURL string
	DataDir string
	t       *testing.T
}

// Start starter appen ferdig onboardet (vanlig tilfelle for de fleste tester).
func Start(t *testing.T) *Harness {
	t.Helper()
	h := StartRaw(t)
	if err := h.App.SetConfig(h.Context(), "onboarded", "1"); err != nil {
		t.Fatalf("kunne ikke markere onboardet: %v", err)
	}
	return h
}

// StartRaw starter appen UTEN å fullføre onboarding (for onboarding-tester).
func StartRaw(t *testing.T) *Harness {
	t.Helper()
	dir := t.TempDir()
	mock := mocks.NewNorgesBankMock()
	app, err := core.New(dir, mock)
	if err != nil {
		t.Fatalf("core.New: %v", err)
	}
	srv, err := server.New(app)
	if err != nil {
		app.Close()
		t.Fatalf("server.New: %v", err)
	}
	ts := httptest.NewServer(srv)

	h := &Harness{
		App:     app,
		Mock:    mock,
		Server:  ts,
		BaseURL: ts.URL,
		DataDir: dir,
		t:       t,
	}
	h.waitHealthy()
	t.Cleanup(func() {
		ts.Close()
		app.Close()
	})
	return h
}

func (h *Harness) waitHealthy() {
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, h.BaseURL+"/health", nil)
		resp, err := http.DefaultClient.Do(req)
		cancel()
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	h.t.Fatal("appen ble aldri frisk (/health svarte ikke 200)")
}

// Browser lager en ny nettleser-simulator mot denne appen.
func (h *Harness) Browser() *Browser {
	return newBrowser(h.t, h.BaseURL)
}

// Context returnerer en bakgrunnskontekst (bekvemmelighet for kall mot core).
func (h *Harness) Context() context.Context { return context.Background() }
