# ENK Regnskap

Et lett, lokalt regnskapsverktøy for norske enkeltpersonforetak (ENK). Appen
kjører som én enkelt Go-binær, åpner automatisk i nettleseren, og lagrer alt
lokalt i en mappe som kan synkroniseres via OneDrive. Ingen sky, ingen
abonnement, ingen installasjon utover å dobbeltklikke én fil.

Målgruppen er en ikke-teknisk bruker som driver et lite tjenestebasert ENK,
gjerne med inntekter i utenlandsk valuta, og som ikke trenger et fullskala
regnskapssystem.

> ⚠️ **Ansvarsfraskrivelse:** Dette er et personlig hjelpeverktøy, ikke
> juridisk eller skattefaglig rådgivning. Appen beregner eller garanterer
> aldri det endelige skatteoppgjøret. Ved tvil – kontakt en regnskapsfører
> eller skatterådgiver.

## Funksjoner

- Registrering av inntekter i NOK og utenlandsk valuta, med automatisk
  kursoppslag mot Norges Bank.
- Registrering av utgifter med fradragskategorier basert på Skatteetatens
  poster for ENK.
- Opplasting og kobling av kvitteringer.
- Egen håndtering av utenlandsinntekt og kreditfradrag (sktl. § 16-20 flg.
  og skatteavtalen Norge–Brasil).
- Årsrapport og næringsspesifikasjon som PDF og Excel.
- Flerspråklig grensesnitt (norsk, portugisisk, engelsk).
- Full endringslogg med angre-funksjon (rollback) – alle endringer kan reverseres.
- **AI-native:** appen eksponerer et MCP-grensesnitt slik at en AI-agent kan
  betjene regnskapet mens appen kjører. Endringer agenten gjør vises umiddelbart
  i nettleseren via live oppdatering (Server-Sent Events).

## Teknisk stack

| Lag           | Valg                                                  |
| ------------- | ----------------------------------------------------- |
| Språk         | Go 1.22+                                               |
| Database      | SQLite via `modernc.org/sqlite` (ingen CGo)           |
| SQL-lag       | `sqlc` for typesikre queries, rå SQL-migrasjoner      |
| HTTP-server   | `net/http` + `go-chi/chi`                             |
| Frontend      | `html/template` + vanilla JS, embeddet via `embed.FS` |
| PDF           | `go-pdf/fpdf`                                          |
| Excel-eksport | `xuri/excelize`                                        |
| Valuta        | Norges Bank XML/JSON-API (caches i SQLite)            |

Bygges uten CGo og uten system-avhengigheter, og krysskompileres til
`windows/amd64`, `darwin/arm64` og `linux/amd64`.

## Installere (ferdigbygd macOS-app)

Last ned siste **DMG** fra
[Releases](https://github.com/kkollsga/enk-regnskap/releases) – `arm64` for
Apple Silicon (M1/M2/M3), `intel` for eldre Intel-Macer. Åpne DMG-en og dra
**EnkRegnskap** til **Programmer**.

Appen er foreløpig ikke signert/notarisert (beta), så første gang må du
høyreklikke appen → **Åpne**, eller fjerne karantenflagget:

```bash
xattr -dr com.apple.quarantine /Applications/EnkRegnskap.app
```

En ny release lages ved å skyve en versjonstag, f.eks. `git tag v1.0.0 &&
git push origin v1.0.0` – GitHub Actions bygger og laster opp DMG-ene.

## Bygge og kjøre lokalt

Krever Go 1.22 eller nyere.

```bash
# Kjør i utviklingsmodus
go run ./cmd/server

# Bygg en binær for din egen plattform
go build -o enk-regnskap ./cmd/server

# Kjør tester
go test ./...
```

Appen starter en HTTP-server på `http://localhost:7331` og åpner den i
standard nettleser.

### Frittstående macOS-app (anbefalt på Mac)

```bash
make mac-app        # bygger dist/EnkRegnskap.app
open dist/EnkRegnskap.app

make dmg            # pakker appen i dist/EnkRegnskap.dmg (drag-til-Applications)
```

Dette gir et ekte macOS-vindu (native tittellinje) i stedet for en
nettleserfane – HTTP-serveren kjører in-process. Flytt `EnkRegnskap.app` til
`/Applications` om du vil ha den i Dock. (Bygges med WKWebView via CGo, kun
macOS.)

### Prosjekter (flere foretak)

Alle data ligger under `~/ENK-Regnskap/`, og hvert foretak er en egen mappe:

```
~/ENK-Regnskap/
  Acme AS - 999888777/      ← ett prosjekt
    data.db
    receipts/
    mirror/
  Fjord Design AS - 111222333/
    ...
```

Ved første oppstart velger eller oppretter du et foretak (firmanavn +
org.nr). Bytt foretak via «Mer → Bytt foretak». Endre basismappe med
`-home <sti>`, eller bruk én konkret prosjektmappe direkte med `-data <sti>`.

Vanlige `make`-mål: `make build`, `make run`, `make dev` (hot reload via
`air`), `make test`, `make dist` (krysskompilering), `make mac-app`.

### AI-agent (MCP)

Appen kan styres av en AI-agent over Model Context Protocol:

- **Mens appen kjører:** agenten snakker JSON-RPC mot `POST /mcp`. Endringer
  går gjennom samme kjerne som nettgrensesnittet, logges i endringsloggen, og
  oppdaterer åpne nettlesere live.
- **Som subprosess (stdio):** `enk-regnskap --mcp` – for klienter som Claude
  Code / Claude Desktop. Eksempel på konfig:

  ```json
  {
    "mcpServers": {
      "enk-regnskap": {
        "command": "/sti/til/enk-regnskap",
        "args": ["--mcp", "-data", "/sti/til/data"]
      }
    }
  }
  ```

  Tilgjengelige verktøy: `add_income`, `add_expense`, `list_income`,
  `list_expenses`, `dashboard`, `foreign_tax_overview`, `generate_report`,
  `tax_info`, `list_changes`, `rollback`, `set_active_year`.

## Data og sikkerhetskopi

All data ligger i `data/`-mappen (database, kvitteringer og eventuelle
nedlastede skatteregler). Denne mappen committes **aldri** til Git – du
kopierer din egen `data/`-mappe inn i arbeidsmappen når du kjører appen, og
kan trygt la den ligge i OneDrive for automatisk sikkerhetskopi.

Sikkerhetskopi finnes i to former:

- **Full backup (`/export/backup.zip`):** et konsistent øyeblikksbilde av hele
  databasen (alle tabeller, inkludert endringslogg/angre-historikk) pluss alle
  kvitteringsfiler. Gjenopprett ved å pakke ut i en `data/`-mappe.
- **Lesbar speilkopi (`data/mirror/`):** en menneskelesbar JSON-kopi av
  kjernedataene (inntekter, utgifter, kvitteringer, innstillinger) pluss
  kopier av kvitteringsfilene. Oppdateres automatisk ved hver endring – et
  ekstra sikkerhetsnett for beta-bruk, uten angre-historikk. Appen kan
  **importere** denne mappen for å sette tilstanden (erstatter gjeldende data).

### Krysskompilering

```bash
GOOS=windows GOARCH=amd64 go build -o dist/enk-regnskap-windows.exe ./cmd/server
GOOS=darwin  GOARCH=arm64 go build -o dist/enk-regnskap-mac        ./cmd/server
GOOS=linux   GOARCH=amd64 go build -o dist/enk-regnskap-linux      ./cmd/server
```

## Personvern

Repoet er offentlig. Faktiske regnskapsdata, kvitteringer, databasefiler og
personopplysninger committes **aldri** – `data/` og alle `*.db`-filer er
ekskludert via `.gitignore`. All testdata bruker fiktive verdier.

## Lisens

MIT – se [LICENSE](LICENSE).
