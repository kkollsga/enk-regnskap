# Talking to ENK Regnskap as an AI agent

ENK Regnskap (a Norwegian sole-proprietorship / *enkeltpersonforetak* accounting app) runs a local **MCP server (JSON-RPC 2.0)** inside the running app. An agent connects with **no config and no install** — discover the address from a file and `curl` it. Changes go through the same audit log and **live-update the open UI instantly**.

> Point an agent at this file. It contains everything needed to connect and operate the app — no prior knowledge of ENK Regnskap required.

## 0. Prerequisite
The app must be **running** (the desktop app, or the `enk-regnskap` server). If nothing answers, ask the user to open ENK Regnskap.

## 1. Discover the endpoint
The running app writes its address to `~/ENK-Regnskap/.mcp-endpoint.json` (or under `$ENK_HOME`). The desktop app binds a **random port**, so always read this file — never assume a port.

```bash
URL=$(python3 -c "import json,os;p=os.path.expanduser(os.environ.get('ENK_HOME','~/ENK-Regnskap')+'/.mcp-endpoint.json');print(json.load(open(p))['mcp_url'])")
echo "$URL"   # e.g. http://127.0.0.1:54321/mcp
```

The file also has `pid` and `started_at`. If a call fails the file may be **stale** (app was force-killed) — verify liveness first:

```bash
curl -s "$URL" -H 'content-type: application/json' -d '{"jsonrpc":"2.0","id":0,"method":"ping"}'
# {"jsonrpc":"2.0","id":0,"result":{}}  → alive
```
(The app removes this file on clean exit / SIGTERM.)

## 2. The protocol
`initialize` → `tools/list` → `tools/call`. Every tool is self-describing — its `inputSchema` is in `tools/list`, so you never need to guess arguments.

```bash
# discover what you can do
curl -s "$URL" -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# call a tool
curl -s "$URL" -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call",
       "params":{"name":"dashboard","arguments":{"year":2025}}}'
```

A tool result comes back as `result.content[0].text` (a JSON string). On a tool-level failure `result.isError` is `true` and the text starts with `Feil:`.

### Optional copy-paste helper
Paste this into the shell to get a terse `enk` command:

```bash
URL=$(python3 -c "import json,os;p=os.path.expanduser(os.environ.get('ENK_HOME','~/ENK-Regnskap')+'/.mcp-endpoint.json');print(json.load(open(p))['mcp_url'])")
enk() { # usage: enk <tool> [json-args]
  local tool="$1" args="$2"; [ -z "$args" ] && args='{}'
  local body
  body=$(python3 -c 'import json,sys;print(json.dumps({"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":sys.argv[1],"arguments":json.loads(sys.argv[2])}}))' "$tool" "$args")
  curl -s "$URL" -H 'content-type: application/json' -d "$body" \
    | python3 -c 'import json,sys;r=json.load(sys.stdin);e=r.get("error");print("RPC-feil:",e["message"]) if e else print(r["result"]["content"][0]["text"])'
}
enk dashboard '{"year":2025}'
enk add_income '{"date":"2025-05-15","description":"Konsulent","amount":10000,"currency":"NOK","country_code":"NO","category":"konsulent","tax_year":2025}'
enk list_companies          # no-arg call works too
```

> **Tip:** call the `guide` tool for the full, always‑current manual (every tool, foreign‑tax `treatment` semantics, conventions), and `status` to see the active company/year + row counts. The app is the source of truth; this file is a quick start.

## 3. Get exactly what you need — don't pull row dumps
The surface is built so you fetch precise numbers without loading every row. Prefer the compact tools:

| Need | Tool | Returns |
|---|---|---|
| Orientation | `status` | active company, active year, income/expense counts |
| Key figures | `dashboard {year}` | income, deductions, result, estimated tax, foreign tax credit, net tax |
| **Sums & averages** | `aggregate {kind:"income"\|"expenses", year, group_by:"category"\|"country"\|"month"\|"total", country_code?, category?, month?}` | `count / sum_nok / avg_nok` per group — **use this instead of listing rows and summing** |
| Tax return helper | `selvangivelse {year}` | næringsresultat, personinntekt, trygdeavgift/trinnskatt, RF-1147 credit, net tax |
| Full report | `generate_report {year}` | **summary only by default**; pass `include_rows:true` *only* if you truly need every row |
| Foreign tax | `foreign_tax_overview {year}` | per-country credit, max credit estimate (§16-21), docs status |
| Rates / valid keys | `tax_info {year}` | deduction categories (valid `category` keys) and rates |

Rows when you must: `list_income` / `list_expenses {year, country_code?, category?, month? (YYYY-MM), income_id?, limit?}` — always filter server-side. `get_income` / `get_expense {id}` for one record.

## 4. Add / edit / delete
- `add_income {date, description, amount, currency, country_code, category, client?, tax_year, foreign_tax_paid?, foreign_taxes?[]}` — foreign currency auto-fetches the Norges Bank rate and computes NOK. `foreign_taxes` items: `{type, amount, currency?, treatment:"credit"|"deduct"|"none"}`.
- `add_expense {date, description, amount, currency, country_code, category, deductible_pct?, income_id?, tax_year}` — `income_id` links the expense to an income (same country+currency required; grouping only).
- `update_income` / `update_expense {id, ...}` — **partial**: only the fields you supply change; the rest is preserved. Income `foreign_taxes` are kept untouched unless you pass `foreign_taxes` (which replaces the whole list). `delete_income` / `delete_expense {id}`.
- `add_foreign_tax {income_id, type, amount, currency?, treatment?}` — **appends** one foreign‑tax line to an income without touching the others. Prefer this over re‑sending the full array.
- `attach_receipt {parent_kind:"income"|"expense", parent_id, filename, content_base64, mime_type?, title?, description?}` — attach a PDF/image (e.g. to document foreign tax for RF‑1147).
- Everything is auditable: `list_changes {limit?}` then `rollback {change_id}` to undo.
- `set_active_year {year}` changes the year the UI shows.

## 5. Set up a company (only if none exists yet)
In workspace mode you can bootstrap from nothing — `/mcp` accepts calls even before a company exists:
- `list_companies`
- `create_company {company, org_nr?, language?}` — creates, activates, and onboards it; afterwards the income/expense tools operate on it.
- `open_company {folder}` — switch active company.
- `generate_dummy_data` — fill a demo dataset for the active year.

## Conventions
- All money is NOK unless a foreign `currency` is given; rates are Norges Bank for the transaction date (weekends fall back to the prior business day).
- Dates are `YYYY-MM-DD` and must fall inside the entry's `tax_year`.
- Valid `category` keys come from `tax_info`; income categories are `tjenesteinntekt, honorar, konsulent, royalty, annet`.
- Output is compact snake_case JSON — no `sql.Null` wrappers, no noise.
- Norwegian rates/figures are for the requested income year (e.g. 2025: 22% general income, 10.9% trygdeavgift on business income).
