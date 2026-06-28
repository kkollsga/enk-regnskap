package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/core"
	"github.com/kkollsga/enk-regnskap/internal/testing/mocks"
)

func newTestServer(t *testing.T) (*Server, *core.App) {
	t.Helper()
	dir := t.TempDir()
	mock := mocks.NewNorgesBankMock()
	mock.AddRate("BRL", "2025-03-10", 1.85)
	app, err := core.New(dir, mock)
	if err != nil {
		t.Fatalf("core.New: %v", err)
	}
	t.Cleanup(func() { app.Close() })
	ctx := context.Background()
	_ = app.SetConfig(ctx, "onboarded", "1")
	_ = app.SetConfig(ctx, core.ConfigActiveYear, "2025")
	return New(app, nil), app
}

// call kjorer et verktoy gjennom tools/call-laget og returnerer tekstinnholdet.
func call(t *testing.T, s *Server, name string, args map[string]any) string {
	t.Helper()
	params, _ := json.Marshal(map[string]any{"name": name, "arguments": args})
	res, rerr := s.callTool(context.Background(), params)
	if rerr != nil {
		t.Fatalf("%s: rpc-feil: %s", name, rerr.Message)
	}
	m := res.(map[string]any)
	text := m["content"].([]map[string]any)[0]["text"].(string)
	if m["isError"].(bool) {
		t.Fatalf("%s: verktoyfeil: %s", name, text)
	}
	return text
}

func mustParse(t *testing.T, s string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(s), v); err != nil {
		t.Fatalf("kunne ikke parse JSON: %v\n%s", err, s)
	}
}

func TestInitializeAndToolsList(t *testing.T) {
	s, _ := newTestServer(t)
	resp := s.handle(context.Background(), rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "initialize"})
	if resp == nil || resp.Error != nil {
		t.Fatalf("initialize feilet: %+v", resp)
	}
	res := resp.Result.(map[string]any)
	if res["protocolVersion"] != protocolVersion {
		t.Fatalf("uventet protocolVersion: %v", res["protocolVersion"])
	}

	listResp := s.handle(context.Background(), rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`2`), Method: "tools/list"})
	tools := listResp.Result.(map[string]any)["tools"].([]map[string]any)
	have := map[string]bool{}
	for _, tl := range tools {
		have[tl["name"].(string)] = true
	}
	for _, want := range []string{
		"add_income", "add_expense", "get_income", "update_income", "delete_income",
		"list_income", "list_expenses", "aggregate", "dashboard", "generate_report",
		"selvangivelse", "foreign_tax_overview", "list_changes", "rollback",
	} {
		if !have[want] {
			t.Errorf("verktoy mangler i tools/list: %s", want)
		}
	}
	// Uten workspace skal foretaksverktoy IKKE finnes.
	if have["create_company"] {
		t.Error("create_company skal ikke finnes uten workspace")
	}
}

func TestCleanDTOOutput(t *testing.T) {
	s, _ := newTestServer(t)
	call(t, s, "add_income", map[string]any{
		"date": "2025-05-15", "description": "NO konsulent", "amount": 1000.0,
		"currency": "NOK", "country_code": "NO", "category": "konsulent", "tax_year": 2025,
	})
	out := call(t, s, "list_income", map[string]any{"year": 2025})
	var rows []map[string]any
	mustParse(t, out, &rows)
	if len(rows) != 1 {
		t.Fatalf("forventet 1 rad, fikk %d", len(rows))
	}
	// Skal være flat verdi, ikke {"String":..,"Valid":..}.
	if _, isObj := rows[0]["client"].(map[string]any); isObj {
		t.Error("client skal være flat streng, ikke sql.Null-innpakning")
	}
	if rows[0]["amount_nok"].(float64) != 1000 {
		t.Errorf("amount_nok=%v", rows[0]["amount_nok"])
	}
}

func TestAggregateAndFilters(t *testing.T) {
	s, _ := newTestServer(t)
	add := func(desc, country, cur string, amt float64) {
		call(t, s, "add_income", map[string]any{
			"date": "2025-03-10", "description": desc, "amount": amt,
			"currency": cur, "country_code": country, "category": "konsulent", "tax_year": 2025,
		})
	}
	add("NO a", "NO", "NOK", 1000)
	add("NO b", "NO", "NOK", 3000)
	add("BR", "BR", "BRL", 1000) // 1000 * 1.85 = 1850 NOK

	// aggregate group_by country
	out := call(t, s, "aggregate", map[string]any{"kind": "income", "year": 2025, "group_by": "country"})
	var buckets []map[string]any
	mustParse(t, out, &buckets)
	got := map[string]float64{}
	for _, b := range buckets {
		got[b["group"].(string)] = b["sum_nok"].(float64)
	}
	if got["NO"] != 4000 {
		t.Errorf("NO sum=%v, vil ha 4000", got["NO"])
	}
	if got["BR"] != 1850 {
		t.Errorf("BR sum=%v, vil ha 1850", got["BR"])
	}

	// total avg
	out = call(t, s, "aggregate", map[string]any{"kind": "income", "year": 2025, "group_by": "total"})
	var totals []map[string]any
	mustParse(t, out, &totals)
	if totals[0]["count"].(float64) != 3 {
		t.Errorf("count=%v", totals[0]["count"])
	}
	if totals[0]["avg_nok"].(float64) != round2(5850.0/3) {
		t.Errorf("avg=%v", totals[0]["avg_nok"])
	}

	// filter list_income by country
	out = call(t, s, "list_income", map[string]any{"year": 2025, "country_code": "BR"})
	var brRows []map[string]any
	mustParse(t, out, &brRows)
	if len(brRows) != 1 || brRows[0]["country_code"] != "BR" {
		t.Errorf("landfilter feilet: %s", out)
	}
}

func TestReportSummaryFirst(t *testing.T) {
	s, _ := newTestServer(t)
	call(t, s, "add_income", map[string]any{
		"date": "2025-05-15", "description": "x", "amount": 5000.0,
		"currency": "NOK", "country_code": "NO", "category": "konsulent", "tax_year": 2025,
	})
	// Default: ingen rader.
	out := call(t, s, "generate_report", map[string]any{"year": 2025})
	var rep map[string]any
	mustParse(t, out, &rep)
	if _, ok := rep["income"]; ok {
		t.Error("generate_report skal utelate rader som standard")
	}
	if rep["total_income"].(float64) != 5000 {
		t.Errorf("total_income=%v", rep["total_income"])
	}
	// include_rows=true: rader med.
	out = call(t, s, "generate_report", map[string]any{"year": 2025, "include_rows": true})
	mustParse(t, out, &rep)
	if _, ok := rep["income"]; !ok {
		t.Error("include_rows=true skal ta med rader")
	}
}

func TestUpdateDeleteRoundtrip(t *testing.T) {
	s, _ := newTestServer(t)
	out := call(t, s, "add_income", map[string]any{
		"date": "2025-05-15", "description": "før", "amount": 1000.0,
		"currency": "NOK", "country_code": "NO", "category": "konsulent", "tax_year": 2025,
	})
	var added map[string]any
	mustParse(t, out, &added)
	id := added["id"].(float64)

	call(t, s, "update_income", map[string]any{
		"id": id, "date": "2025-05-15", "description": "etter", "amount": 2000.0,
		"currency": "NOK", "country_code": "NO", "category": "konsulent", "tax_year": 2025,
	})
	out = call(t, s, "get_income", map[string]any{"id": id})
	var got map[string]any
	mustParse(t, out, &got)
	if got["description"] != "etter" || got["amount_nok"].(float64) != 2000 {
		t.Errorf("oppdatering slo ikke gjennom: %s", out)
	}

	call(t, s, "delete_income", map[string]any{"id": id})
	out = call(t, s, "list_income", map[string]any{"year": 2025})
	var rows []map[string]any
	mustParse(t, out, &rows)
	if len(rows) != 0 {
		t.Errorf("forventet tom liste etter sletting, fikk %d", len(rows))
	}
}

func TestSelvangivelseTool(t *testing.T) {
	s, _ := newTestServer(t)
	call(t, s, "add_income", map[string]any{
		"date": "2025-05-15", "description": "x", "amount": 100000.0,
		"currency": "NOK", "country_code": "NO", "category": "konsulent", "tax_year": 2025,
	})
	out := call(t, s, "selvangivelse", map[string]any{"year": 2025})
	var sv map[string]any
	mustParse(t, out, &sv)
	if sv["naeringsresultat"].(float64) != 100000 {
		t.Errorf("naeringsresultat=%v", sv["naeringsresultat"])
	}
	if sv["trygdeavgift_pct"].(float64) != 10.9 {
		t.Errorf("trygdeavgift_pct=%v (vil ha 10.9 for 2025)", sv["trygdeavgift_pct"])
	}
}

func TestCompanyToolsWithWorkspace(t *testing.T) {
	dir := t.TempDir()
	ws, err := core.NewWorkspace(dir, mocks.NewNorgesBankMock())
	if err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}
	t.Cleanup(func() { ws.Close() })
	s := New(ws.Current(), ws)

	// Foretaksverktoy skal finnes når workspace er satt.
	listResp := s.handle(context.Background(), rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/list"})
	tools := listResp.Result.(map[string]any)["tools"].([]map[string]any)
	have := map[string]bool{}
	for _, tl := range tools {
		have[tl["name"].(string)] = true
	}
	for _, want := range []string{"list_companies", "create_company", "open_company"} {
		if !have[want] {
			t.Fatalf("foretaksverktoy mangler: %s", want)
		}
	}

	out := call(t, s, "create_company", map[string]any{"company": "Test ENK", "org_nr": "999888777"})
	var created map[string]any
	mustParse(t, out, &created)
	if created["active"] != true {
		t.Errorf("nytt foretak skal være aktivt: %s", out)
	}
	if ws.Current() == nil {
		t.Fatal("workspace har ikke aktivt foretak etter create_company")
	}

	out = call(t, s, "list_companies", map[string]any{})
	var companies []map[string]any
	mustParse(t, out, &companies)
	if len(companies) != 1 || companies[0]["active"] != true {
		t.Errorf("list_companies uventet: %s", out)
	}
}
