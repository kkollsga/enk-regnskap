package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// toolErrors kjorer et tools/call og returnerer true hvis det ble en feil
// (RPC-feil eller isError). Brukt for negative tester.
func toolErrors(t *testing.T, s *Server, p map[string]any) bool {
	t.Helper()
	params, _ := json.Marshal(p)
	res, rerr := s.callTool(context.Background(), params)
	if rerr != nil {
		return true
	}
	return res.(map[string]any)["isError"].(bool)
}

// brIncomeWithIRRF oppretter en brasiliansk inntekt med én kreditlinje og
// returnerer id-en.
func brIncomeWithIRRF(t *testing.T, s *Server) float64 {
	t.Helper()
	out := call(t, s, "add_income", map[string]any{
		"date": "2025-03-10", "description": "BR", "amount": 1000.0,
		"currency": "BRL", "country_code": "BR", "category": "konsulent", "tax_year": 2025,
		"foreign_tax_paid": 1,
		"foreign_taxes":    []any{map[string]any{"type": "IRRF", "amount": 100.0, "treatment": "credit"}},
	})
	var added map[string]any
	mustParse(t, out, &added)
	return added["id"].(float64)
}

func TestPartialUpdatePreservesFields(t *testing.T) {
	s, app := newTestServer(t)
	id := brIncomeWithIRRF(t, s)

	// Endre KUN beskrivelsen.
	call(t, s, "update_income", map[string]any{"id": id, "description": "BR endret"})

	out := call(t, s, "get_income", map[string]any{"id": id})
	var g map[string]any
	mustParse(t, out, &g)
	if g["description"] != "BR endret" {
		t.Errorf("beskrivelse ble ikke endret: %v", g["description"])
	}
	if g["amount_nok"].(float64) != 1850 { // 1000 * 1.85, bevart
		t.Errorf("amount_nok skal være bevart 1850, fikk %v", g["amount_nok"])
	}
	if g["foreign_tax_paid"].(float64) != 1 {
		t.Errorf("foreign_tax_paid skal være bevart, fikk %v", g["foreign_tax_paid"])
	}
	// Skattelinjen skal fortsatt finnes (ikke nullstilt av partial update).
	lines, _ := app.IncomeForeignTaxes(context.Background(), int64(id))
	if len(lines) != 1 {
		t.Fatalf("forventet 1 bevart skattelinje, fikk %d", len(lines))
	}
}

func TestAddForeignTaxAppends(t *testing.T) {
	s, app := newTestServer(t)
	id := brIncomeWithIRRF(t, s)

	call(t, s, "add_foreign_tax", map[string]any{
		"income_id": id, "type": "ISS", "amount": 50.0, "treatment": "deduct",
	})
	lines, _ := app.IncomeForeignTaxes(context.Background(), int64(id))
	if len(lines) != 2 {
		t.Fatalf("forventet 2 linjer etter add_foreign_tax, fikk %d", len(lines))
	}
	types := map[string]string{}
	for _, l := range lines {
		types[l.TaxType] = l.Treatment
	}
	if types["IRRF"] != "credit" || types["ISS"] != "deduct" {
		t.Errorf("uventede linjer: %+v", types)
	}
}

func TestAttachReceipt(t *testing.T) {
	s, _ := newTestServer(t)
	id := brIncomeWithIRRF(t, s)
	content := base64.StdEncoding.EncodeToString([]byte("%PDF-1.4 dummy"))
	out := call(t, s, "attach_receipt", map[string]any{
		"parent_kind": "income", "parent_id": id,
		"filename": "selvangivelse.pdf", "content_base64": content,
	})
	var rec map[string]any
	mustParse(t, out, &rec)
	if _, ok := rec["id"]; !ok {
		t.Fatalf("attach_receipt returnerte ingen id: %s", out)
	}

	// Ugyldig parent_kind skal feile som verktoyfeil.
	params := map[string]any{"name": "attach_receipt", "arguments": map[string]any{
		"parent_kind": "bogus", "parent_id": id, "filename": "x.pdf", "content_base64": content,
	}}
	if !toolErrors(t, s, params) {
		t.Error("forventet feil for ugyldig parent_kind")
	}
}

func TestStatusAndGuide(t *testing.T) {
	s, _ := newTestServer(t)
	brIncomeWithIRRF(t, s)

	out := call(t, s, "status", map[string]any{})
	var st map[string]any
	mustParse(t, out, &st)
	if st["active_year"].(float64) != 2025 {
		t.Errorf("active_year=%v", st["active_year"])
	}
	if st["income_count"].(float64) != 1 {
		t.Errorf("income_count=%v", st["income_count"])
	}

	guide := call(t, s, "guide", map[string]any{})
	for _, want := range []string{"treatment", "add_foreign_tax", "State model", "attach_receipt"} {
		if !strings.Contains(guide, want) {
			t.Errorf("guide mangler %q", want)
		}
	}
}
