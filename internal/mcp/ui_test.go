package mcp

import (
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/core"
	"github.com/kkollsga/enk-regnskap/internal/testing/mocks"
)

func TestUIControlBroadcasts(t *testing.T) {
	s, app := newTestServer(t)
	ch, unsub := app.Events.Subscribe()
	defer unsub()

	call(t, s, "navigate", map[string]any{"page": "income"})
	if ev := <-ch; ev.Type != "navigate" || ev.Path != "/income" {
		t.Errorf("navigate event: %+v", ev)
	}

	call(t, s, "set_language", map[string]any{"lang": "pt"})
	if ev := <-ch; ev.Type != "navigate" || ev.Path != "/set-lang?lang=pt" {
		t.Errorf("set_language event: %+v", ev)
	}

	call(t, s, "ui_toggle", map[string]any{"selector": ".estimate-details", "mode": "open"})
	if ev := <-ch; ev.Type != "ui" || ev.Action != "open" || ev.Selector != ".estimate-details" {
		t.Errorf("ui_toggle event: %+v", ev)
	}

	// Ukjent side skal feile.
	if !toolErrors(t, s, map[string]any{"name": "navigate", "arguments": map[string]any{"page": "bogus"}}) {
		t.Error("forventet feil for ukjent side")
	}
	// Ugyldig språk skal feile.
	if !toolErrors(t, s, map[string]any{"name": "set_language", "arguments": map[string]any{"lang": "de"}}) {
		t.Error("forventet feil for ugyldig språk")
	}
}

func TestPerCallCompanySwitchAndNavigate(t *testing.T) {
	dir := t.TempDir()
	ws, err := core.NewWorkspace(dir, mocks.NewNorgesBankMock())
	if err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}
	t.Cleanup(func() { ws.Close() })
	s := New(ws.Current(), ws)

	call(t, s, "create_company", map[string]any{"company": "Alpha ENK", "org_nr": "111111111"})
	call(t, s, "create_company", map[string]any{"company": "Beta ENK", "org_nr": "222222222"})
	if name := ws.CurrentName(); name != "Beta ENK - 222222222" {
		t.Fatalf("forventet Beta aktiv, fikk %q", name)
	}

	// Abonner på Beta-appen (den klienten ville vært koblet til) for å fange
	// navigate-eventet før byttet.
	bApp := ws.Current()
	ch, unsub := bApp.Events.Subscribe()
	defer unsub()

	// Per-kall 'company': målrett Alpha (delvis navn) uten eget open_company.
	out := call(t, s, "status", map[string]any{"company": "Alpha"})
	var st map[string]any
	mustParse(t, out, &st)
	if st["active_company"] != "Alpha ENK - 111111111" {
		t.Errorf("status skal vise Alpha aktiv, fikk %v", st["active_company"])
	}
	if name := ws.CurrentName(); name != "Alpha ENK - 111111111" {
		t.Errorf("workspace skal ha byttet til Alpha, fikk %q", name)
	}
	// Beta-vinduet skal ha fått beskjed om å navigere til forsiden.
	if ev := <-ch; ev.Type != "navigate" || ev.Path != "/" {
		t.Errorf("forventet navigate-event på Beta-appen, fikk %+v", ev)
	}
}
