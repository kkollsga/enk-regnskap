# ENK Regnskap MCP — operating guide

You are connected to a running ENK Regnskap app (Norwegian sole‑proprietorship / *enkeltpersonforetak* accounting). Everything below is callable as MCP tools. Your changes hit the same audit log and live‑update the open window.

## State model (important)
- The app has ONE **active company** and ONE **active income year** at a time.
- Most tools take `year` explicitly — pass it. Some default to the active year.
- Call `status` to see the active company/year; `set_active_year {year}` and `open_company` change state.
- **Per‑call company:** any data/read tool also accepts an optional `company` (company name, org.nr, or folder). If given and different from the active one, the app switches to it (and its window navigates there) for that call and onward. Lets you target a company without a separate `open_company`.

## Retrieving figures (prefer these over listing rows)
- `status` — active company, year, and income/expense counts. Call this first to orient.
- `dashboard {year}` — income, deductible costs, result (næringsresultat), estimated tax (alminnelig inntektsskatt / trygdeavgift / trinnskatt), foreign tax credit, net tax.
- `aggregate {kind:"income"|"expenses", year, group_by:"category"|"country"|"month"|"total", country_code?, category?, month?}` — `count` / `sum_nok` / `avg_nok` per group, no rows.
- `selvangivelse {year}` — RF‑form figures (næringsresultat, personinntekt, trygdeavgift/trinnskatt, RF‑1147 credit, net tax).
- `foreign_tax_overview {year}` — per‑country credit, `max_credit_est` (§16‑21 cap), `docs_missing`.
- `generate_report {year, include_rows?}` — summary by default; pass `include_rows:true` only if you need every line.
- `tax_info {year}` — valid `category` keys and the year's rates.

## Listing / finding rows
- `list_income {year, country_code?, category?, month?(YYYY-MM), limit?}`
- `list_expenses {year, country_code?, category?, month?, income_id?, limit?}`
- `get_income {id}` / `get_expense {id}`

## Adding
- `add_income {date, description, amount, currency, country_code, category, client?, tax_year, foreign_tax_paid?, foreign_taxes?[]}` — foreign `currency` auto‑fetches the Norges Bank rate for `date` and returns the NOK amount.
- `add_expense {date, description, amount, currency, country_code, category, deductible_pct?, income_id?, tax_year}` — `income_id` links to an income (same country+currency required; grouping only).

## Editing — partial, non‑destructive
- `update_income {id, …}` / `update_expense {id, …}` change **only the fields you supply**; everything else is preserved. For income, `foreign_taxes` are kept untouched unless you pass `foreign_taxes` explicitly. You do **not** need to re‑send the whole record.
- `add_foreign_tax {income_id, type, amount, currency?, treatment?}` — **appends** one foreign‑tax line to an income without touching the others. Prefer this over re‑sending the full `foreign_taxes` array.
- `delete_income {id}` / `delete_expense {id}`.

## Foreign tax treatment (what each does to the calc)
Each `foreign_taxes` line has a `treatment`:
- `credit` — creditable foreign **income** tax (e.g. IRRF, IRPF, CSLL). Reduces Norwegian tax as a credit (kreditfradrag, sktl. §16‑20), capped per country by the §16‑21 max.
- `deduct` — indirect tax (e.g. ISS, PIS, COFINS). Booked as a **deductible cost**, reducing næringsresultat (not a credit).
- `none` — no relief (e.g. INSS social security). Recorded for reference only; excluded from the tax calc.
- Empty `treatment` → derived from the country tax catalog.

## Attachments / documentation
- `attach_receipt {parent_kind:"income"|"expense", parent_id, filename, content_base64, mime_type?, title?, description?, tax_year?}` — attach a PDF/image (declaration, ticket, etc.). `mime_type` is inferred from the filename if omitted. Allowed: JPG/PNG/GIF/WEBP/HEIC/PDF.

## Undo / audit
Every mutation is logged: `list_changes {limit?}` then `rollback {change_id}` (restores deleted rows too).

## Companies (workspace mode)
`list_companies`, `create_company {company, org_nr?, language?}` (creates + activates + onboards), `open_company {company}` (name / org.nr / folder — switches the active company **and** navigates the app window to it). `/mcp` accepts calls even before a company exists, so you can bootstrap from nothing.

## Driving the app window (live)
You can change what the user sees, in real time:
- `navigate {page}` — switch the visible page. `page` = `dashboard | income | expenses | foreign-tax | tax-info | selvangivelse | reports | new-income | new-expense`, or any path starting with `/`.
- `set_language {lang}` — `nb` | `pt` | `en`.
- `ui_toggle {selector, mode?}` — expand/collapse `<details>` sections by CSS selector (e.g. `selector:"details.entry"` for all entries, `".estimate-details"` for the tax breakdown). `mode` = `toggle` (default) | `open` | `close`.
- Switching the active company (`open_company`, `create_company`, or a per‑call `company`) navigates the window to the new company automatically.

## Conventions
Money is NOK unless a foreign `currency` is given. Dates are `YYYY-MM-DD` and must fall in the entry's `tax_year`. Output is compact snake_case JSON. Rates/figures are for the requested income year (2025: 22% general income, 10.9% trygdeavgift on business income).
