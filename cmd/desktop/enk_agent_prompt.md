Create a Claude Code slash command called `/enk` so I can easily ask an AI agent to talk to my running **ENK Regnskap** app — a local Norwegian sole‑proprietorship (*enkeltpersonforetak*) accounting app. It exposes a local **MCP (JSON‑RPC 2.0)** endpoint, so an agent can read its figures and add/edit/delete income & expenses with **no config and no install**.

Please:
1. Create the file `.claude/commands/enk.md` (make the `.claude/commands/` folder if it doesn't exist).
2. Put **exactly** the content between the `=== BEGIN ===` / `=== END ===` markers below into that file (do not include the markers).
3. Confirm it's saved, then show me one example, e.g. `/enk what was my 2025 revenue and estimated tax?`

=== BEGIN ===
---
description: Talk to the running ENK Regnskap accounting app over its local MCP interface (read figures, add/edit/delete income & expenses)
---

You can control a running local **ENK Regnskap** app (Norwegian *enkeltpersonforetak* accounting) through its **MCP server (JSON‑RPC 2.0)**. Connect with no config — discover the address from a file and `curl` it. Your changes go through the app's audit log and live‑update its open window.

## 1. Connect
The running app writes its address to `~/ENK-Regnskap/.mcp-endpoint.json` (or under `$ENK_HOME`); the desktop app uses a random port, so always read this file.

```bash
URL=$(python3 -c "import json,os;p=os.path.expanduser(os.environ.get('ENK_HOME','~/ENK-Regnskap')+'/.mcp-endpoint.json');print(json.load(open(p))['mcp_url'])")
curl -s "$URL" -H 'content-type: application/json' -d '{"jsonrpc":"2.0","id":0,"method":"ping"}'   # expect {"result":{}}
```
If ping fails, the app isn't running — ask the user to open ENK Regnskap and stop. Then paste this helper once:

```bash
enk() { # usage: enk <tool> [json-args]
  local tool="$1" args="$2"; [ -z "$args" ] && args='{}'
  local body
  body=$(python3 -c 'import json,sys;print(json.dumps({"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":sys.argv[1],"arguments":json.loads(sys.argv[2])}}))' "$tool" "$args")
  curl -s "$URL" -H 'content-type: application/json' -d "$body" \
    | python3 -c 'import json,sys;r=json.load(sys.stdin);e=r.get("error");print("RPC-feil:",e["message"]) if e else print(r["result"]["content"][0]["text"])'
}
```
List every tool + its argument schema any time with:
`curl -s "$URL" -H 'content-type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'`

## 2. Working rules (read before acting)
- **Pick the active year.** Most tools take `year`. If the user doesn't say one, ask or use the current income year. `enk set_active_year '{"year":2025}'` changes the year the app's UI shows.
- **For numbers, don't dump rows.** Use the summary tools below; only list rows when the user wants specific entries.
- **Never invent an id.** To edit or delete, first *find* the record (list/filter or read) and use its real `id`.
- **`update_*` REPLACES the fields you send** — it is not a partial patch. Send the complete desired state. For income, `foreign_taxes` are re‑set on every update, so re‑send them or they're cleared.
- **Validate before writing.** Valid `category` keys and rates come from `enk tax_info '{"year":Y}'`. Dates are `YYYY-MM-DD` and must fall inside the entry's `tax_year`. Money is NOK unless you pass a foreign `currency` (then the Norges Bank rate is fetched automatically).
- **Confirm after writing.** Re‑read (`dashboard`/`aggregate`/`get_*`) and tell the user the resulting figures. Every change is auditable and reversible — see Undo.

## 3. Retrieving statistics
- Key figures: `enk dashboard '{"year":2025}'` → income, deductible costs, result (næringsresultat), estimated tax (alminnelig inntektsskatt / trygdeavgift / trinnskatt), foreign tax credit, net tax.
- Sums & averages **without loading rows**: `enk aggregate '{"kind":"income","year":2025,"group_by":"country"}'` — `group_by` = `category`|`country`|`month`|`total`; optional `country_code`/`category`/`month` filters. Returns `count`, `sum_nok`, `avg_nok` (and `deductible_nok` for expenses) per group. Use this for "total/average revenue, expenses by category", etc.
- Tax‑return helper: `enk selvangivelse '{"year":2025}'` → RF‑form figures (næringsresultat, personinntekt, trygdeavgift/trinnskatt, RF‑1147 credit, net tax).
- Foreign tax per country (credit, §16‑21 max‑credit estimate, docs status): `enk foreign_tax_overview '{"year":2025}'`.
- Full report (summary by default; add `"include_rows":true` **only** if you truly need every line): `enk generate_report '{"year":2025}'`.

## 4. Listing / finding entries (to inspect, or to get an id before edit/delete)
- `enk list_income '{"year":2025}'` — filter with `country_code`, `category`, `month` (`YYYY-MM`), `limit`. e.g. `'{"year":2025,"country_code":"BR"}'`.
- `enk list_expenses '{"year":2025}'` — same filters plus `income_id` (expenses linked to an income).
- `enk get_income '{"id":N}'` / `enk get_expense '{"id":N}'` for one record.

## 5. Adding income
```
enk add_income '{"date":"2025-05-15","description":"Konsulentoppdrag","amount":10000,
  "currency":"NOK","country_code":"NO","category":"konsulent","client":"Acme AS","tax_year":2025}'
```
- `category`: one of `tjenesteinntekt, honorar, konsulent, royalty, annet`.
- Foreign‑currency income: set `currency` (e.g. `BRL`) and the NOK amount is computed from the Norges Bank rate on `date`.
- **Foreign tax withheld abroad:** add `"foreign_tax_paid":1` and a `foreign_taxes` array, one item per tax type with the correct `treatment`:
  - `"credit"` — creditable income taxes (e.g. **IRRF, IRPF, CSLL**) → reduce Norwegian tax (kreditfradrag).
  - `"deduct"` — indirect taxes (e.g. **ISS, PIS, COFINS**) → deductible cost, not a credit.
  - `"none"` — e.g. **INSS** (social security) → recorded for reference only.
  ```
  "foreign_tax_paid":1,
  "foreign_taxes":[{"type":"IRRF","amount":1500,"treatment":"credit"},
                   {"type":"ISS","amount":500,"treatment":"deduct"}]
  ```

## 6. Adding expenses
```
enk add_expense '{"date":"2025-06-10","description":"Flyreise","amount":1200,
  "currency":"NOK","country_code":"NO","category":"reise","tax_year":2025}'
```
- `deductible_pct` is taken from the category default unless you pass it.
- Optional `"income_id":N` groups the expense under an income (reporting only). The expense and that income must have the **same country and currency**, or it's rejected.

## 7. Editing (remember: update REPLACES the sent fields)
1. Find the record and its `id` (Section 4).
2. Optionally `get_income`/`get_expense` to see current values.
3. Send the full desired state:
```
enk update_income '{"id":7,"date":"2025-05-15","description":"Konsulent (justert)","amount":12000,
  "currency":"NOK","country_code":"NO","category":"konsulent","tax_year":2025}'
```
   For income that had foreign taxes, **re‑include** `foreign_tax_paid` + `foreign_taxes` or they are wiped.
`enk update_expense '{"id":3,...}'` works the same (`"income_id":0` removes a link).

## 8. Deleting
Find the `id`, then `enk delete_income '{"id":7}'` or `enk delete_expense '{"id":3}'`. Confirm what you deleted.

## 9. Undo / audit
`enk list_changes '{"limit":10}'` shows recent changes with ids; `enk rollback '{"change_id":N}'` reverts one (including restoring a deleted record).

## 10. Set up a company (only if none exists)
If tools report no active company: `enk list_companies '{}'`, then `enk create_company '{"company":"Mitt Foretak","org_nr":"999111222"}'` (creates, activates, onboards), or `enk open_company '{"folder":"..."}'` to switch.

---

Now carry out the user's request below against the running app. If it's a question, read the relevant figures first and answer in NOK. If it's an edit, validate inputs, make the change, then confirm the result by re‑reading.

User request: $ARGUMENTS
=== END ===
