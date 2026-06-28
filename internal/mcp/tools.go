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
				Treatment: la.str("treatment"),
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

// incomeInputFromArgs bygger en IncomeInput fra verktoyargumentene (delt av
// add_income og update_income).
func incomeInputFromArgs(a Args) core.IncomeInput {
	return core.IncomeInput{
		Date: a.str("date"), Description: a.str("description"),
		Category: a.str("category"), Client: a.str("client"),
		CountryCode: a.str("country_code"), Currency: a.str("currency"),
		AmountOrig: a.num("amount"), TaxYear: a.intval("tax_year"),
		Notes: a.str("notes"), ForeignTaxPaid: a.intval("foreign_tax_paid"),
		ForeignTaxes: parseForeignTaxes(a),
	}
}

// expenseInputFromArgs bygger en ExpenseInput fra verktoyargumentene (delt av
// add_expense og update_expense).
func expenseInputFromArgs(a Args) core.ExpenseInput {
	amount := a.num("amount")
	if amount == 0 {
		amount = a.num("amount_nok") // bakoverkompatibelt alias
	}
	in := core.ExpenseInput{
		Date: a.str("date"), Description: a.str("description"),
		Category: a.str("category"), AmountOrig: amount,
		Currency: a.str("currency"), CountryCode: a.str("country_code"),
		TaxYear: a.intval("tax_year"), Notes: a.str("notes"),
	}
	if v := a.intval("income_id"); v > 0 {
		id := int64(v)
		in.IncomeID = &id
	}
	if _, ok := a["deductible_pct"]; ok {
		in.DeductiblePct = a.num("deductible_pct")
		in.HasDeductiblePct = true
	}
	return in
}

func (s *Server) buildTools() []Tool {
	app := s.app
	tools := []Tool{
		{
			Name:        "add_income",
			Description: "Registrer en inntekt. For utenlandsk valuta hentes Norges Bank-kurs automatisk og NOK-beløp beregnes. Sett country_code og foreign_tax_* for utenlandsinntekt.",
			InputSchema: obj(map[string]any{
				"date":             prop("string", "Dato (AAAA-MM-DD)"),
				"description":      prop("string", "Beskrivelse"),
				"amount":           prop("number", "Beløp i valgt valuta"),
				"currency":         prop("string", "Valutakode (NOK, USD, EUR, BRL, ...). Default NOK."),
				"country_code":     prop("string", "Kildeland ISO 3166-1 (default NO)"),
				"category":         prop("string", "Inntektskategori (tjenesteinntekt, honorar, konsulent, royalty, annet)"),
				"client":           prop("string", "Klient (valgfritt)"),
				"foreign_tax_paid": prop("integer", "0=nei, 1=ja, 2=vet ikke (utenlandsk kildeskatt)"),
				"foreign_taxes": map[string]any{
					"type":        "array",
					"description": "Utenlandsk skatt brutt ned per type. Hvert element: {type, amount, currency?}. Currency default = inntektens valuta.",
					"items": obj(map[string]any{
						"type":      prop("string", "Skattetype, f.eks. IRRF, ISS, CSLL"),
						"amount":    prop("number", "Beløp i utenlandsk valuta"),
						"currency":  prop("string", "Valuta (default = inntektens valuta)"),
						"treatment": prop("string", "Styrer beregningen: 'credit' = krediterbar inntektsskatt, trekkes fra norsk skatt (kreditfradrag §16-20, tak §16-21); 'deduct' = indirekte skatt, føres som fradragsberettiget kostnad (reduserer næringsresultatet); 'none' = ingen lettelse, kun referanse. Tom = utled fra katalog."),
					}, "type", "amount"),
				},
				"foreign_tax_amount": prop("number", "Enkel variant (én skattetype): beløp i utenlandsk valuta. Bruk foreign_taxes for flere."),
				"foreign_tax_type":   prop("string", "Enkel variant (én skattetype): skattetype, f.eks. IRRF"),
				"tax_year":           prop("integer", "Inntektsar (default utledes fra dato)"),
				"notes":              prop("string", "Notater"),
			}, "date", "description", "amount", "category"),
			Run: func(ctx context.Context, a Args) (string, error) {
				res, err := app.AddIncome(ctx, core.ActorMCP, incomeInputFromArgs(a))
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
			Description: "Registrer en utgift. For utenlandsk valuta hentes Norges Bank-kurs automatisk og NOK-beløp beregnes. Fradragsprosent hentes fra kategoriens standard hvis ikke oppgitt.",
			InputSchema: obj(map[string]any{
				"date":           prop("string", "Dato (AAAA-MM-DD)"),
				"description":    prop("string", "Beskrivelse"),
				"amount":         prop("number", "Beløp i valgt valuta"),
				"currency":       prop("string", "Valutakode (NOK, BRL, ...). Default NOK."),
				"country_code":   prop("string", "Land ISO 3166-1 (default NO). Styrer landspesifikke kategorier."),
				"category":       prop("string", "Fradragskategori (se tax_info for gyldige nøkler)"),
				"deductible_pct": prop("number", "Fradragsprosent (valgfritt; default fra kategori)"),
				"income_id":      prop("integer", "Knytt utgiften til en inntekt (valgfritt, kun gruppering; samme land+valuta kreves)"),
				"tax_year":       prop("integer", "Inntektsar (default fra dato)"),
				"notes":          prop("string", "Notater"),
			}, "date", "description", "amount", "category"),
			Run: func(ctx context.Context, a Args) (string, error) {
				exp, err := app.AddExpense(ctx, core.ActorMCP, expenseInputFromArgs(a))
				if err != nil {
					return "", err
				}
				return toJSON(map[string]any{
					"id": exp.ID, "deductible_nok": exp.DeductibleNok, "deductible_pct": exp.DeductiblePct,
				}), nil
			},
		},
		{
			Name:        "get_income",
			Description: "Hent en enkelt inntekt med id.",
			InputSchema: obj(map[string]any{"id": prop("integer", "Inntekts-id")}, "id"),
			Run: func(ctx context.Context, a Args) (string, error) {
				in, err := app.GetIncome(ctx, int64(a.intval("id")))
				if err != nil {
					return "", err
				}
				return toJSON(incomeDTO(in)), nil
			},
		},
		{
			Name:        "update_income",
			Description: "Endre en eksisterende inntekt (id). Feltene fra add_income gjelder; oppgitte verdier erstatter de gamle. foreign_taxes settes på nytt.",
			InputSchema: obj(map[string]any{
				"id":               prop("integer", "Inntekts-id som skal endres"),
				"date":             prop("string", "Dato (AAAA-MM-DD)"),
				"description":      prop("string", "Beskrivelse"),
				"amount":           prop("number", "Beløp i valgt valuta"),
				"currency":         prop("string", "Valutakode (NOK, USD, EUR, BRL, ...)"),
				"country_code":     prop("string", "Kildeland ISO 3166-1"),
				"category":         prop("string", "Inntektskategori"),
				"client":           prop("string", "Klient"),
				"foreign_tax_paid": prop("integer", "0=nei, 1=ja, 2=vet ikke"),
				"foreign_taxes": map[string]any{
					"type":        "array",
					"description": "Utenlandsk skatt per type: {type, amount, currency?, treatment?}. Erstatter eksisterende linjer.",
					"items": obj(map[string]any{
						"type":      prop("string", "Skattetype, f.eks. IRRF, ISS, CSLL"),
						"amount":    prop("number", "Beløp i utenlandsk valuta"),
						"currency":  prop("string", "Valuta (default = inntektens valuta)"),
						"treatment": prop("string", "credit = krediterbar inntektsskatt → kreditfradrag (§16-20/§16-21); deduct = indirekte skatt → fradragsberettiget kostnad; none = kun referanse. Tom = utled fra katalog."),
					}, "type", "amount"),
				},
				"tax_year": prop("integer", "Inntektsar"),
				"notes":    prop("string", "Notater"),
			}, "id", "date", "description", "amount", "category"),
			Run: func(ctx context.Context, a Args) (string, error) {
				res, err := app.UpdateIncome(ctx, core.ActorMCP, int64(a.intval("id")), incomeInputFromArgs(a))
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
			Name:        "delete_income",
			Description: "Slett en inntekt med id (kan rulles tilbake via list_changes/rollback).",
			InputSchema: obj(map[string]any{"id": prop("integer", "Inntekts-id")}, "id"),
			Run: func(ctx context.Context, a Args) (string, error) {
				id := int64(a.intval("id"))
				if err := app.DeleteIncome(ctx, core.ActorMCP, id); err != nil {
					return "", err
				}
				return fmt.Sprintf("Inntekt #%d slettet.", id), nil
			},
		},
		{
			Name:        "get_expense",
			Description: "Hent en enkelt utgift med id.",
			InputSchema: obj(map[string]any{"id": prop("integer", "Utgifts-id")}, "id"),
			Run: func(ctx context.Context, a Args) (string, error) {
				e, err := app.GetExpense(ctx, int64(a.intval("id")))
				if err != nil {
					return "", err
				}
				return toJSON(expenseDTO(e)), nil
			},
		},
		{
			Name:        "update_expense",
			Description: "Endre en eksisterende utgift (id). Feltene fra add_expense gjelder; oppgitte verdier erstatter de gamle. income_id=0 fjerner koblingen.",
			InputSchema: obj(map[string]any{
				"id":             prop("integer", "Utgifts-id som skal endres"),
				"date":           prop("string", "Dato (AAAA-MM-DD)"),
				"description":    prop("string", "Beskrivelse"),
				"amount":         prop("number", "Beløp i valgt valuta"),
				"currency":       prop("string", "Valutakode"),
				"country_code":   prop("string", "Land ISO 3166-1"),
				"category":       prop("string", "Fradragskategori"),
				"deductible_pct": prop("number", "Fradragsprosent"),
				"income_id":      prop("integer", "Knytt til inntekt (0 = ingen kobling)"),
				"tax_year":       prop("integer", "Inntektsar"),
				"notes":          prop("string", "Notater"),
			}, "id", "date", "description", "amount", "category"),
			Run: func(ctx context.Context, a Args) (string, error) {
				exp, err := app.UpdateExpense(ctx, core.ActorMCP, int64(a.intval("id")), expenseInputFromArgs(a))
				if err != nil {
					return "", err
				}
				return toJSON(map[string]any{
					"id": exp.ID, "deductible_nok": exp.DeductibleNok, "deductible_pct": exp.DeductiblePct,
				}), nil
			},
		},
		{
			Name:        "delete_expense",
			Description: "Slett en utgift med id (kan rulles tilbake via list_changes/rollback).",
			InputSchema: obj(map[string]any{"id": prop("integer", "Utgifts-id")}, "id"),
			Run: func(ctx context.Context, a Args) (string, error) {
				id := int64(a.intval("id"))
				if err := app.DeleteExpense(ctx, core.ActorMCP, id); err != nil {
					return "", err
				}
				return fmt.Sprintf("Utgift #%d slettet.", id), nil
			},
		},
		{
			Name:        "list_income",
			Description: "List inntekter for et inntektsar. Bruk filtre (country_code/category/month) for å hente kun det du trenger, ikke hele året. For totaler/snitt, bruk aggregate i stedet for å laste rader.",
			InputSchema: obj(map[string]any{
				"year":         prop("integer", "Inntektsar"),
				"country_code": prop("string", "Filtrer på kildeland (valgfritt)"),
				"category":     prop("string", "Filtrer på kategori (valgfritt)"),
				"month":        prop("string", "Filtrer på måned AAAA-MM (valgfritt)"),
				"limit":        prop("integer", "Maks antall rader (valgfritt)"),
			}, "year"),
			Run: func(ctx context.Context, a Args) (string, error) {
				rows, err := app.ListIncome(ctx, a.intval("year"))
				if err != nil {
					return "", err
				}
				rows = filterIncome(rows, a)
				return toJSON(incomeDTOs(rows)), nil
			},
		},
		{
			Name:        "list_expenses",
			Description: "List utgifter for et inntektsar. Bruk filtre (country_code/category/month) for å hente kun det du trenger. For totaler/snitt, bruk aggregate.",
			InputSchema: obj(map[string]any{
				"year":         prop("integer", "Inntektsar"),
				"country_code": prop("string", "Filtrer på land (valgfritt)"),
				"category":     prop("string", "Filtrer på kategori (valgfritt)"),
				"month":        prop("string", "Filtrer på måned AAAA-MM (valgfritt)"),
				"income_id":    prop("integer", "Kun utgifter koblet til denne inntekten (valgfritt)"),
				"limit":        prop("integer", "Maks antall rader (valgfritt)"),
			}, "year"),
			Run: func(ctx context.Context, a Args) (string, error) {
				rows, err := app.ListExpenses(ctx, a.intval("year"))
				if err != nil {
					return "", err
				}
				rows = filterExpenses(rows, a)
				return toJSON(expenseDTOs(rows)), nil
			},
		},
		{
			Name:        "aggregate",
			Description: "Summer og snitt uten å laste rader. kind=income|expenses, group_by=category|country|month|total. Returnerer per gruppe: count, sum_nok, avg_nok (+ deductible_nok for utgifter). Bruk dette for nøkkeltall.",
			InputSchema: obj(map[string]any{
				"kind":         prop("string", "income eller expenses"),
				"year":         prop("integer", "Inntektsar"),
				"group_by":     prop("string", "category | country | month | total (default total)"),
				"country_code": prop("string", "Filtrer på land (valgfritt)"),
				"category":     prop("string", "Filtrer på kategori (valgfritt)"),
				"month":        prop("string", "Filtrer på måned AAAA-MM (valgfritt)"),
			}, "kind", "year"),
			Run: func(ctx context.Context, a Args) (string, error) {
				return aggregateRun(ctx, app, a)
			},
		},
		{
			Name:        "dashboard",
			Description: "Hent nøkkeltall (inntekt, fradrag, resultat, skatteestimat, kreditfradrag, netto skatt) for et inntektsar. Kompakt – ingen rader.",
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
				return toJSON(foreignOverviewDTOs(ov)), nil
			},
		},
		{
			Name:        "generate_report",
			Description: "Rapport for et år: totaler per kategori, resultat, skatteestimat og kreditfradrag. Returnerer KUN sammendrag som standard. Sett include_rows=true bare hvis du faktisk trenger hver inntekts-/utgiftsrad.",
			InputSchema: obj(map[string]any{
				"year":         prop("integer", "Inntektsar"),
				"include_rows": prop("boolean", "Ta med alle inntekts-/utgiftsrader (default false – hold svaret lite)"),
			}, "year"),
			Run: func(ctx context.Context, a Args) (string, error) {
				year := a.intval("year")
				rep, err := app.BuildReport(ctx, year)
				if err != nil {
					return "", err
				}
				foreign, _ := app.ForeignTaxForYear(ctx, year)
				out := reportDTO(rep, foreign)
				if b, ok := a["include_rows"].(bool); !ok || !b {
					out.Income = nil
					out.Expenses = nil
				}
				return toJSON(out), nil
			},
		},
		{
			Name:        "selvangivelse",
			Description: "Strukturert hjelp til skattemeldingen for ENK (RF-skjema): næringsresultat, personinntekt, trygdeavgift/trinnskatt, kreditfradrag (RF-1147) og estimert skatt etter kredit. Kompakt – ingen rader.",
			InputSchema: obj(map[string]any{"year": prop("integer", "Inntektsar")}, "year"),
			Run: func(ctx context.Context, a Args) (string, error) {
				return selvangivelseRun(ctx, app, a.intval("year"))
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
				return toJSON(changeDTOs(rows)), nil
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
	if s.ws != nil {
		tools = append(tools, s.companyTools()...)
	}
	return tools
}
