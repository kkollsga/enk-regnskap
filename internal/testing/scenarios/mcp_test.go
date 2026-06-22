package scenarios

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/core"
	apptest "github.com/kkollsga/enk-regnskap/internal/testing"
)

// Fase 13: MCP-grensesnitt.

func mcpCall(t *testing.T, h *apptest.Harness, method string, params any) map[string]any {
	t.Helper()
	body := map[string]any{"jsonrpc": "2.0", "id": 1, "method": method}
	if params != nil {
		body["params"] = params
	}
	b, _ := json.Marshal(body)
	resp, err := http.Post(h.BaseURL+"/mcp", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("dekod svar: %v", err)
	}
	return out
}

// callToolText returnerer tekstinnholdet fra et tools/call-svar.
func callToolText(t *testing.T, h *apptest.Harness, name string, args map[string]any) string {
	t.Helper()
	out := mcpCall(t, h, "tools/call", map[string]any{"name": name, "arguments": args})
	if out["error"] != nil {
		t.Fatalf("tools/call %s feilet: %v", name, out["error"])
	}
	result, _ := out["result"].(map[string]any)
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("tomt innhold fra %s", name)
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	if isErr, _ := result["isError"].(bool); isErr {
		t.Fatalf("verktoy %s ga feil: %s", name, text)
	}
	return text
}

func TestMCPInitialize(t *testing.T) {
	h := apptest.Start(t)
	out := mcpCall(t, h, "initialize", map[string]any{})
	result, ok := out["result"].(map[string]any)
	if !ok {
		t.Fatalf("mangler result: %v", out)
	}
	if result["protocolVersion"] == nil {
		t.Error("mangler protocolVersion")
	}
}

func TestMCPToolsList(t *testing.T) {
	h := apptest.Start(t)
	out := mcpCall(t, h, "tools/list", nil)
	result, _ := out["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	if len(tools) < 8 {
		t.Errorf("forventet flere verktoy, fikk %d", len(tools))
	}
	names := map[string]bool{}
	for _, tl := range tools {
		m, _ := tl.(map[string]any)
		names[m["name"].(string)] = true
	}
	for _, want := range []string{"add_income", "add_expense", "dashboard", "rollback"} {
		if !names[want] {
			t.Errorf("mangler verktoy %s", want)
		}
	}
}

func TestMCPAddIncomeAndQuery(t *testing.T) {
	h := apptest.Start(t)
	h.App.SetConfig(h.Context(), core.ConfigActiveYear, "2025")
	h.Mock.AddRate("BRL", "2025-04-10", 2.00)

	// Agenten legger til en brasiliansk inntekt.
	text := callToolText(t, h, "add_income", map[string]any{
		"date": "2025-04-10", "description": "MCP-inntekt", "amount": 10000.0,
		"currency": "BRL", "country_code": "BR", "category": "tjenesteinntekt",
		"foreign_tax_paid": 1.0, "foreign_tax_amount": 1500.0, "foreign_tax_type": "IRRF",
		"tax_year": 2025.0,
	})
	if !strings.Contains(text, "20000") {
		t.Errorf("forventet amount_nok 20000 i svar, fikk: %s", text)
	}

	// Inntekten skal finnes i databasen (samme core som web).
	rows, _ := h.App.ListIncome(h.Context(), 2025)
	if len(rows) != 1 {
		t.Fatalf("forventet 1 inntekt, fikk %d", len(rows))
	}

	// dashboard-verktoyet skal reflektere den.
	dash := callToolText(t, h, "dashboard", map[string]any{"year": 2025.0})
	if !strings.Contains(dash, "20000") {
		t.Errorf("dashboard mangler inntekt: %s", dash)
	}
}

func TestMCPRollback(t *testing.T) {
	h := apptest.Start(t)
	h.App.SetConfig(h.Context(), core.ConfigActiveYear, "2025")
	callToolText(t, h, "add_income", map[string]any{
		"date": "2025-01-10", "description": "Angres via MCP", "amount": 5000.0,
		"currency": "NOK", "country_code": "NO", "category": "tjenesteinntekt", "tax_year": 2025.0,
	})
	// Finn change-id via list_changes-verktoyet.
	changesText := callToolText(t, h, "list_changes", map[string]any{"limit": 10.0})
	var changes []map[string]any
	json.Unmarshal([]byte(changesText), &changes)
	var id float64
	for _, c := range changes {
		if c["operation"] == "insert" && c["entity"] == "income" {
			id = c["id"].(float64)
		}
	}
	if id == 0 {
		t.Fatal("fant ingen insert-endring")
	}
	callToolText(t, h, "rollback", map[string]any{"change_id": id})

	rows, _ := h.App.ListIncome(h.Context(), 2025)
	if len(rows) != 0 {
		t.Errorf("inntekt skulle være rullet tilbake, fikk %d", len(rows))
	}
}

func TestMCPValidationError(t *testing.T) {
	h := apptest.Start(t)
	// Ugyldig inntekt (beløp 0) -> isError, ikke protokollfeil.
	out := mcpCall(t, h, "tools/call", map[string]any{
		"name": "add_income",
		"arguments": map[string]any{
			"date": "2025-01-10", "description": "", "amount": 0.0, "category": "",
		},
	})
	result, _ := out["result"].(map[string]any)
	if isErr, _ := result["isError"].(bool); !isErr {
		t.Errorf("forventet isError=true for ugyldig inntekt, fikk: %v", out)
	}
}
