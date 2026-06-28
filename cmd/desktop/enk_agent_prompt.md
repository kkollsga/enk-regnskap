Create a Claude Code slash command called `/enk` so I can easily ask an AI agent to talk to my running **ENK Regnskap** app — a local Norwegian sole‑proprietorship (*enkeltpersonforetak*) accounting app. It exposes a local **MCP (JSON‑RPC 2.0)** endpoint, so an agent can read its figures and add/edit/delete income & expenses with **no config and no install**.

Please:
1. Create the file `.claude/commands/enk.md` (make the `.claude/commands/` folder if it doesn't exist).
2. Put **exactly** the content between the `=== BEGIN ===` / `=== END ===` markers below into that file (do not include the markers).
3. Confirm it's saved, then show me one example, e.g. `/enk what was my 2025 revenue and estimated tax?`

=== BEGIN ===
---
description: Talk to the running ENK Regnskap accounting app over its local MCP interface (read figures, add/edit/delete income & expenses)
---

You control a running local **ENK Regnskap** app (Norwegian *enkeltpersonforetak* accounting) through its **MCP server (JSON‑RPC 2.0)**. No config — discover the address from a file and `curl` it. Changes hit the app's audit log and live‑update its window.

## 1. Connect
The app writes its address to `~/ENK-Regnskap/.mcp-endpoint.json` (or under `$ENK_HOME`); the desktop app uses a random port, so always read this file.

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

## 2. Learn the API from the app itself
Run **`enk guide`** first — it returns the full, always‑current manual (state model, every tool, foreign‑tax `treatment` semantics, partial‑update rules, conventions). Then `enk status` to see the active company/year. Don't rely on this file for details; the app is the source of truth.

Quick cheat‑sheet (call `guide` for the rest):
- Figures: `enk status`, `enk dashboard '{"year":2025}'`, `enk aggregate '{"kind":"income","year":2025,"group_by":"country"}'`, `enk selvangivelse '{"year":2025}'`.
- Rows: `enk list_income '{"year":2025,"country_code":"BR"}'`, `enk get_income '{"id":1}'`.
- Add: `enk add_income '{...}'`, `enk add_expense '{...}'` (foreign `currency` auto‑converts via Norges Bank).
- Edit (partial — only supplied fields change): `enk update_income '{"id":1,"description":"…"}'`. Append one foreign tax: `enk add_foreign_tax '{"income_id":1,"type":"IRRF","amount":7500,"treatment":"credit"}'`.
- Docs: `enk attach_receipt '{"parent_kind":"income","parent_id":1,"filename":"x.pdf","content_base64":"…"}'`.
- Companies: `enk list_companies '{}'`, `enk open_company '{"company":"Acme"}'` (name/org.nr/folder); any tool also takes `"company"` to target one per call.
- Drive the window: `enk navigate '{"page":"selvangivelse"}'`, `enk set_language '{"lang":"en"}'`, `enk ui_toggle '{"selector":"details.entry","mode":"open"}'`.
- Undo: `enk list_changes '{}'` then `enk rollback '{"change_id":N}'`.

## 3. Conventions
Money is NOK unless a foreign `currency` is given. Dates are `YYYY-MM-DD`, inside the entry's `tax_year`. Valid `category` keys: `enk tax_info '{"year":2025}'`.

---

Now carry out the user's request below. For a question, read the relevant figures first and answer in NOK. For an edit, validate inputs (use `guide`/`tax_info` if unsure), make the change, then confirm by re‑reading.

User request: $ARGUMENTS
=== END ===
