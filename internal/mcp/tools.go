package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kkollsga/enk-regnskap/internal/core"
)

// Args er argumentkartet til et verktoykall med trygge oppslagshjelpere.
type Args map[string]any

func (a Args) str(key string) string {
	if v, ok := a[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func (a Args) num(key string) float64 {
	switch v := a[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	}
	return 0
}

func (a Args) intval(key string) int { return int(a.num(key)) }

// parseForeignTaxes leser utenlandske skattelinjer fra foreign_taxes-arrayet,
// med fallback til den enkle foreign_tax_amount/foreign_tax_type-varianten.
func parseForeignTaxes(a Args) []core.ForeignTaxLine {
	var out []core.ForeignTaxLine
	if raw, ok := a["foreign_taxes"].([]any); ok {
		for _, it := range raw {
			m, ok := it.(map[string]any)
			if !ok {
				continue
			}
			la := Args(m)
			out = append(out, core.ForeignTaxLine{
				Type: la.str("type"), AmountOrig: la.num("amount"), Currency: la.str("currency"),
			})
		}
	}
	if len(out) == 0 && a.num("foreign_tax_amount") > 0 {
		out = append(out, core.ForeignTaxLine{
			Type: a.str("foreign_tax_type"), AmountOrig: a.num("foreign_tax_amount"), Currency: a.str("currency"),
		})
	}
	return out
}

// Tool er et MCP-verktoy.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Run         func(ctx context.Context, args Args) (string, error)
}

// obj og prop er hjelpere for å bygge JSON Schema.
func obj(props map[string]any, required ...string) map[string]any {
	m := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}
func prop(typ, desc string) map[string]any { return map[string]any{"type": typ, "description": desc} }

func toJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

func (s *Server) buildTools() []Tool {
	app := s.app
	return []Tool{
		{
			Name:        "add_income",
			Description: "Registrer en inntekt. For utenlandsk valuta hentes Norges Bank-kurs automatisk og NOK-beløp beregnes. Sett country_code og foreign_tax_* for utenlandsinntekt.",
			InputSchema: obj(map[string]any{
				"date":               prop("string", "Dato (AAAA-MM-DD)"),
				"description":        prop("string", "Beskrivelse"),
				"amount":             prop("number", "Beløp i valgt valuta"),
				"currency":           prop("string", "Valutakode (NOK, USD, EUR, BRL, ...). Default NOK."),
				"country_code":       prop("string", "Kildeland ISO 3166-1 (default NO)"),
				"category":           prop("string", "Inntektskategori (tjenesteinntekt, honorar, konsulent, royalty, annet)"),
				"client":             prop("string", "Klient (valgfritt)"),
				"foreign_tax_paid":   prop("integer", "0=nei, 1=ja, 2=vet ikke (utenlandsk kildeskatt)"),
				"foreign_taxes": map[string]any{
					"type":        "array",
					"description": "Utenlandsk skatt brutt ned per type. Hvert element: {type, amount, currency?}. Currency default = inntektens valuta.",
					"items": obj(map[string]any{
						"type":     prop("string", "Skattetype, f.eks. IRRF, ISS, CSLL"),
						"amount":   prop("number", "Beløp i utenlandsk valuta"),
						"currency": prop("string", "Valuta (default = inntektens valuta)"),
					}, "type", "amount"),
				},
				"foreign_tax_amount": prop("number", "Enkel variant (én skattetype): beløp i utenlandsk valuta. Bruk foreign_taxes for flere."),
				"foreign_tax_type":   prop("string", "Enkel variant (én skattetype): skattetype, f.eks. IRRF"),
				"tax_year":           prop("integer", "Inntektsar (default utledes fra dato)"),
				"notes":              prop("string", "Notater"),
			}, "date", "description", "amount", "category"),
			Run: func(ctx context.Context, a Args) (string, error) {
				in := core.IncomeInput{
					Date: a.str("date"), Description: a.str("description"),
					Category: a.str("category"), Client: a.str("client"),
					CountryCode: a.str("country_code"), Currency: a.str("currency"),
					AmountOrig: a.num("amount"), TaxYear: a.intval("tax_year"),
					Notes: a.str("notes"), ForeignTaxPaid: a.intval("foreign_tax_paid"),
					ForeignTaxes: parseForeignTaxes(a),
				}
				res, err := app.AddIncome(ctx, core.ActorMCP, in)
				if err != nil {
					return "", err
				}
				return toJSON(map[string]any{
					"id": res.Income.ID, "amount_nok": res.Income.AmountNok,
					"rate_used": res.RateUsed, "rate_date": res.RateDate,
				}), nil
			},
		},
		{
			Name:        "add_expense",
			Description: "Registrer en utgift (alltid NOK). Fradragsprosent hentes fra kategoriens standard hvis ikke oppgitt.",
			InputSchema: obj(map[string]any{
				"date":           prop("string", "Dato (AAAA-MM-DD)"),
				"description":    prop("string", "Beskrivelse"),
				"amount_nok":     prop("number", "Beløp i NOK"),
				"category":       prop("string", "Fradragskategori (se tax_info for gyldige nøkler)"),
				"deductible_pct": prop("number", "Fradragsprosent (valgfritt; default fra kategori)"),
				"tax_year":       prop("integer", "Inntektsar (default fra dato)"),
				"notes":          prop("string", "Notater"),
			}, "date", "description", "amount_nok", "category"),
			Run: func(ctx context.Context, a Args) (string, error) {
				in := core.ExpenseInput{
					Date: a.str("date"), Description: a.str("description"),
					Category: a.str("category"), AmountNOK: a.num("amount_nok"),
					TaxYear: a.intval("tax_year"), Notes: a.str("notes"),
				}
				if _, ok := a["deductible_pct"]; ok {
					in.DeductiblePct = a.num("deductible_pct")
					in.HasDeductiblePct = true
				}
				exp, err := app.AddExpense(ctx, core.ActorMCP, in)
				if err != nil {
					return "", err
				}
				return toJSON(map[string]any{
					"id": exp.ID, "deductible_nok": exp.DeductibleNok, "deductible_pct": exp.DeductiblePct,
				}), nil
			},
		},
		{
			Name:        "list_income",
			Description: "List inntekter for et inntektsar.",
			InputSchema: obj(map[string]any{"year": prop("integer", "Inntektsar")}, "year"),
			Run: func(ctx context.Context, a Args) (string, error) {
				rows, err := app.ListIncome(ctx, a.intval("year"))
				if err != nil {
					return "", err
				}
				return toJSON(rows), nil
			},
		},
		{
			Name:        "list_expenses",
			Description: "List utgifter for et inntektsar.",
			InputSchema: obj(map[string]any{"year": prop("integer", "Inntektsar")}, "year"),
			Run: func(ctx context.Context, a Args) (string, error) {
				rows, err := app.ListExpenses(ctx, a.intval("year"))
				if err != nil {
					return "", err
				}
				return toJSON(rows), nil
			},
		},
		{
			Name:        "dashboard",
			Description: "Hent nøkkeltall (inntekt, fradrag, resultat, skatteestimat) for et inntektsar.",
			InputSchema: obj(map[string]any{"year": prop("integer", "Inntektsar (default aktivt år)")}),
			Run: func(ctx context.Context, a Args) (string, error) {
				year := a.intval("year")
				if year == 0 {
					year = app.ActiveYear(ctx)
				}
				d, err := app.Dashboard(ctx, year)
				if err != nil {
					return "", err
				}
				return toJSON(d), nil
			},
		},
		{
			Name:        "foreign_tax_overview",
			Description: "Oversikt over utenlandsinntekt og kreditfradrag per land for et ar (rettsgrunnlag, betalt skatt, estimert maks kredit).",
			InputSchema: obj(map[string]any{"year": prop("integer", "Inntektsar")}, "year"),
			Run: func(ctx context.Context, a Args) (string, error) {
				ov, err := app.ForeignTaxForYear(ctx, a.intval("year"))
				if err != nil {
					return "", err
				}
				return toJSON(ov), nil
			},
		},
		{
			Name:        "generate_report",
			Description: "Bygg en rapport (totaler per kategori, resultat, skatteestimat, kreditfradrag) for et ar.",
			InputSchema: obj(map[string]any{"year": prop("integer", "Inntektsar")}, "year"),
			Run: func(ctx context.Context, a Args) (string, error) {
				rep, err := app.BuildReport(ctx, a.intval("year"))
				if err != nil {
					return "", err
				}
				return toJSON(rep), nil
			},
		},
		{
			Name:        "tax_info",
			Description: "Skatteregler for et ar: fradragskategorier (gyldige nøkler) og satser.",
			InputSchema: obj(map[string]any{"year": prop("integer", "Inntektsar")}, "year"),
			Run: func(ctx context.Context, a Args) (string, error) {
				rules, err := app.TaxRulesFor(a.intval("year"))
				if err != nil {
					return "", err
				}
				return toJSON(rules), nil
			},
		},
		{
			Name:        "list_changes",
			Description: "List siste endringer (revisjonslogg) med id-er som kan rulles tilbake.",
			InputSchema: obj(map[string]any{"limit": prop("integer", "Maks antall (default 50)")}),
			Run: func(ctx context.Context, a Args) (string, error) {
				limit := int64(a.intval("limit"))
				if limit <= 0 {
					limit = 50
				}
				rows, err := app.Q.ListChangeLog(ctx, limit)
				if err != nil {
					return "", err
				}
				return toJSON(rows), nil
			},
		},
		{
			Name:        "rollback",
			Description: "Rull tilbake en endring (gjenoppretter tilstanden for endringen) ved change-id.",
			InputSchema: obj(map[string]any{"change_id": prop("integer", "ID fra list_changes")}, "change_id"),
			Run: func(ctx context.Context, a Args) (string, error) {
				id := int64(a.intval("change_id"))
				if err := app.Rollback(ctx, core.ActorMCP, id); err != nil {
					return "", err
				}
				return fmt.Sprintf("Endring #%d rullet tilbake.", id), nil
			},
		},
		{
			Name:        "set_active_year",
			Description: "Sett aktivt inntektsar for appen (påvirker nettgrensesnittet).",
			InputSchema: obj(map[string]any{"year": prop("integer", "Inntektsar")}, "year"),
			Run: func(ctx context.Context, a Args) (string, error) {
				year := a.intval("year")
				if err := app.SetConfig(ctx, core.ConfigActiveYear, fmt.Sprintf("%d", year)); err != nil {
					return "", err
				}
				app.Events.Broadcast(core.Event{Type: "config", Action: "updated", Year: year})
				return fmt.Sprintf("Aktivt ar satt til %d.", year), nil
			},
		},
		{
			Name:        "generate_dummy_data",
			Description: "Fyll appen med et fiktivt testdatasett (12 inntekter inkl. brasilianske, 8 utgifter, en kvittering) for det aktive inntektsåret.",
			InputSchema: obj(map[string]any{}),
			Run: func(ctx context.Context, a Args) (string, error) {
				n, err := app.GenerateDummyData(ctx, core.ActorMCP)
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("Opprettet %d fiktive transaksjoner.", n), nil
			},
		},
		{
			Name:        "rebuild_mirror",
			Description: "Skriv den lesbare datakopien (mirror-mappen) på nytt fra databasen.",
			InputSchema: obj(map[string]any{}),
			Run: func(ctx context.Context, a Args) (string, error) {
				if err := app.SyncMirror(ctx); err != nil {
					return "", err
				}
				return "Speilkopi oppdatert: " + app.MirrorDir(), nil
			},
		},
		{
			Name:        "import_mirror",
			Description: "Sett tilstanden fra en lesbar mirror-mappe. ERSTATTER nåværende inntekter, utgifter og kvitteringer. Uten 'dir' brukes appens egen mirror-mappe.",
			InputSchema: obj(map[string]any{
				"dir": prop("string", "Sti til mirror-mappen (valgfritt)"),
			}),
			Run: func(ctx context.Context, a Args) (string, error) {
				dir := a.str("dir")
				if dir == "" {
					dir = app.MirrorDir()
				}
				if err := app.ImportMirror(ctx, dir); err != nil {
					return "", err
				}
				return "Tilstand importert fra " + dir, nil
			},
		},
	}
}
