# Build Prompt: ENK Regnskap

## Hva vi bygger

En lokal desktop-web-applikasjon for norske enkeltpersonforetak (ENK) som nylig er startet opp. Appen kjører som én enkelt Go-binær, åpner automatisk i nettleseren, og lagrer alt lokalt i en mappe som synkroniseres via OneDrive. Ingen sky, ingen abonnement, ingen installasjon utover å dobbeltklikke én fil.

Målgruppen er en ikke-teknisk bruker som driver et lite tjenestebasert ENK, gjerne med inntekter i utenlandsk valuta, og som ikke har behov for et fullskala regnskapssystem.

Designfilosofien er Apple, ikke Microsoft: færrest mulig steg for å logge en transaksjon, rent hvitt grensesnitt, god typografi, ingen unødvendige valg.

------

## Første steg før du skriver kode

**Les og analyser årets regler for ENK-selvangivelse i Norge.**

Gjør følgende:

1. Hent gjeldende skattemelding og næringsspesifikasjon for ENK fra Skatteetaten:
   - `https://www.skatteetaten.no/person/skatt/skattemelding/naringsdrivende/`
   - `https://github.com/Skatteetaten/skattemeldingen` (XSD-skjemaer og eksempelfiler)
2. Identifiser alle poster i næringsspesifikasjonen som er relevante for et lite tjenestebasert ENK uten ansatte og uten varelager: driftsinntekter, driftskostnader, fradragsberettigede utgifter, hjemmekontor, bil, reise, telefon/internett, og eventuelle utenlandsinntekter.
3. Map disse postene til appens interne kategorier. Bruk Skatteetatens offisielle postbetegnelser og RF-skjemanummer der de finnes.
4. Lagre denne mappingen som `internal/tax/rules_ÅÅÅÅ.go` (ett fil per inntektsår). Filen skal inneholde:
   - Alle godkjente fradragskategorier med navn, beskrivelse og maksbeløp der det gjelder
   - Satser for hjemmekontor, kilometergodtgjørelse og andre sjablongfradrag
   - Referanse til hvilket RF-skjema posten tilhører
   - Gyldig inntektsår

Gjenta dette for hvert inntektsår appen skal støtte. Reglene lastes automatisk basert på hvilke år som finnes i databasen.

------

## Tech stack

| Lag           | Valg                                                         |
| ------------- | ------------------------------------------------------------ |
| Språk         | Go 1.22+                                                     |
| Database      | SQLite via `modernc.org/sqlite` (ingen CGo)                  |
| SQL-lag       | `sqlc` for typesikre queries, rå SQL-migrasjonsfiler         |
| HTTP-server   | `net/http` + `go-chi/chi` router                             |
| Frontend      | `html/template` embeddet i binæren via `embed.FS`, vanilla JS |
| PDF           | `go-pdf/fpdf`                                                |
| Excel-eksport | `xuri/excelize`                                              |
| Valuta        | Norges Bank XML-API (caches i SQLite)                        |
| Årsregler     | Én Go-fil per inntektsår i `internal/tax/`                   |

Crosskompilering: bygg for `windows/amd64`, `darwin/arm64`, og `linux/amd64` fra samme kodebase. Ingen CGo, ingen system-dependencies.

------

## Mappestruktur

```
paula-regnskap/
  cmd/
    server/
      main.go                 # starter HTTP-server, åpner nettleser
  internal/
    db/
      schema.sql              # migrasjoner
      queries/
        income.sql
        expenses.sql
        receipts.sql
        currency.sql
        reports.sql
      db.go                   # sqlc-generert
    handlers/
      income.go
      expenses.go
      receipts.go
      reports.go
      currency.go
      tax_info.go             # viser årsregler i appen
    currency/
      norgesbank.go           # henter dagskurs, cacher i SQLite
    tax/
      rules_2024.go
      rules_2025.go           # legg til hvert år
      loader.go               # laster riktig år automatisk
    pdf/
      annual_report.go
      tax_summary.go
    export/
      excel.go
  web/
    static/
      app.js
      style.css
    templates/
      layout.html
      dashboard.html
      income.html
      expenses.html
      receipts.html
      report.html
      tax_info.html
  data/                       # denne mappen synkroniseres til OneDrive
    data.db
    receipts/
      2024/
      2025/
    tax_rules/
      2024/                   # nedlastede PDF-er fra Skatteetaten
      2025/
  build.sh                    # crosskompilering
  sqlc.yaml
  go.mod
```

`data/`-mappen er den eneste mappen som trenger å ligge i OneDrive. Binæren kan ligge hvor som helst. Ved første oppstart spør appen om hvor `data/`-mappen skal ligge, og lagrer valget i en lokal config.

------

## Databasemodell

```sql
-- Inntekter
CREATE TABLE income (
  id           INTEGER PRIMARY KEY,
  date         TEXT NOT NULL,          -- ISO 8601
  description  TEXT NOT NULL,
  amount_orig  REAL NOT NULL,          -- beløp i original valuta
  currency     TEXT NOT NULL DEFAULT 'NOK',
  exchange_rate REAL,                  -- hentet fra Norges Bank, NULL hvis NOK
  amount_nok   REAL NOT NULL,          -- amount_orig * exchange_rate
  category     TEXT NOT NULL,          -- 'tjenesteinntekt', 'honorar', etc.
  client       TEXT,
  receipt_id   INTEGER REFERENCES receipts(id),
  tax_year     INTEGER NOT NULL,
  notes        TEXT,
  created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Utgifter
CREATE TABLE expenses (
  id           INTEGER PRIMARY KEY,
  date         TEXT NOT NULL,
  description  TEXT NOT NULL,
  amount_nok   REAL NOT NULL,
  category     TEXT NOT NULL,          -- kobles til tax/rules_ÅÅÅÅ.go
  deductible_pct REAL NOT NULL DEFAULT 100.0,
  deductible_nok REAL NOT NULL,        -- amount_nok * deductible_pct / 100
  receipt_id   INTEGER REFERENCES receipts(id),
  tax_year     INTEGER NOT NULL,
  notes        TEXT,
  created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Kvitteringer
CREATE TABLE receipts (
  id           INTEGER PRIMARY KEY,
  filename     TEXT NOT NULL,          -- relativ sti fra data/receipts/
  original_name TEXT NOT NULL,
  mime_type    TEXT NOT NULL,
  uploaded_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Valutakurser (cache)
CREATE TABLE exchange_rates (
  currency     TEXT NOT NULL,
  date         TEXT NOT NULL,          -- ISO 8601
  rate_nok     REAL NOT NULL,          -- 1 enhet valuta = rate_nok NOK
  source       TEXT NOT NULL DEFAULT 'norges-bank',
  PRIMARY KEY (currency, date)
);

-- Skatteregler per land (lastes ned og vedlikeholdes i appen)
CREATE TABLE country_tax_rules (
  id               INTEGER PRIMARY KEY,
  country_code     TEXT NOT NULL,          -- ISO 3166-1, f.eks. 'BR', 'NO'
  country_name     TEXT NOT NULL,
  effective_from   INTEGER NOT NULL,       -- inntektsår regelen gjelder fra
  effective_to     INTEGER,                -- NULL = fortsatt gjeldende
  has_tax_treaty   INTEGER NOT NULL DEFAULT 0,  -- 1 = skatteavtale med Norge
  treaty_in_force_date TEXT,              -- ISO 8601, dato avtalen trådte i kraft
  treaty_method    TEXT,                  -- 'credit', 'exemption', NULL hvis ingen avtale
  treaty_reference TEXT,                  -- f.eks. 'Prop. 13 S (2022-2023)'
  treaty_source_url TEXT,                 -- lenke til avtaleteksten på Lovdata
  standard_withholding_pct REAL,          -- standard kildeskattesats landet bruker
  notes            TEXT,                  -- fritekst om særregler
  last_updated     TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Skattetyper per land (IRRF, ISS, CSLL, etc.)
CREATE TABLE country_tax_types (
  id               INTEGER PRIMARY KEY,
  country_code     TEXT NOT NULL,
  tax_type_code    TEXT NOT NULL,         -- f.eks. 'IRRF', 'ISS', 'CSLL'
  tax_type_name    TEXT NOT NULL,         -- fullt navn
  description      TEXT,                  -- forklaring på norsk
  applies_to       TEXT,                  -- 'tjenester', 'lønn', 'utbytte', etc.
  is_creditable_in_norway INTEGER DEFAULT 1, -- 1 = godkjent for kreditfradrag i Norge
  basis            TEXT,                  -- 'netto' eller 'brutto'
  typical_rate_pct REAL,
  effective_from   INTEGER NOT NULL,
  effective_to     INTEGER,
  UNIQUE(country_code, tax_type_code, effective_from)
);

-- App-konfigurasjon
CREATE TABLE config (
  key          TEXT PRIMARY KEY,
  value        TEXT NOT NULL
);
```

------

## Valutakonvertering

Integrasjonen mot Norges Bank skal:

- Bruke endepunktet `https://data.norges-bank.no/api/data/EXR/B.{VALUTA}.NOK.SP?startPeriod={DATO}&endPeriod={DATO}&format=sdmx-json`
- Støtte alle vanlige valutaer (USD, EUR, BRL, GBP, SEK, DKK)
- Cache kursen i `exchange_rates`-tabellen slik at historiske transaksjoner aldri trenger nett
- Hvis kursen for en dato ikke finnes (helg, helligdag), bruk nærmeste foregående bankdag
- Vise hvilken kurs som ble brukt og hvilken dato kursen er fra, direkte i inntektsskjemaet

Når bruker velger valuta og dato, hentes kursen automatisk og beløpet i NOK beregnes og vises umiddelbart. Bruker trenger bare å bekrefte.

------

## Appens sider og brukerflyt

### Dashboard

- Årets inntekter hittil (NOK)
- Årets fradragsberettigede utgifter hittil (NOK)
- Estimert skattemessig resultat
- Antall ubehandlede kvitteringer (uten tilknyttet transaksjon)
- Snarvei til "Legg til inntekt" og "Legg til utgift"

### Legg til inntekt

1. Dato (default: i dag)
2. Beskrivelse
3. Klient (valgfritt, fritekst med autocomplete fra tidligere)
4. Valuta (dropdown, default NOK)
5. Beløp i valgt valuta
6. Beløp i NOK vises automatisk med kursinfo
7. Kategori (dropdown fra årets regler)
8. Kvittering / dokumentasjon (drag-and-drop eller filvelger, valgfritt)
9. Lagre

Maksimalt 9 felter, maks 3 klikk fra åpen app til lagret transaksjon.

### Legg til utgift

1. Dato (default: i dag)
2. Beskrivelse
3. Beløp (alltid NOK for utgifter)
4. Kategori (dropdown med beskrivelse og fradragsprosent fra årets regler)
5. Fradragsberettiget beløp vises automatisk
6. Kvittering (drag-and-drop eller filvelger, valgfritt)
7. Lagre

### Kvitteringer

- Oversikt over alle opplastede kvitteringer
- Filtrer på tilknyttet / ikke tilknyttet transaksjon
- Forhåndsvisning inline (bilde eller PDF)
- Knytt kvittering til eksisterende transaksjon

### Skatteinfo (årsregler)

En dedikert side som viser:

- Gjeldende fradragskategorier og satser for valgt inntektsår
- Direkte lenker til Skatteetatens veiledninger
- Lokal kopi av nedlastede PDF-er fra Skatteetaten (åpnes i nettleser)
- Knapp for å oppdatere/laste ned årets regler fra Skatteetaten

Denne siden er appens "hjelp"-seksjon og skal gjøre det unødvendig å lete etter informasjon andre steder under selvangivelsen.

### Rapporter

- **Årsrapport**: fullstendig oversikt over inntekter og utgifter per kategori, totaler, vedlagte kvitteringer som referanseliste. Eksporteres som PDF.
- **Næringsspesifikasjon**: strukturert etter Skatteetatens poster for ENK, klar til å taste inn i skattemeldingen på skatteetaten.no. Eksporteres som PDF og Excel.
- **Transaksjonslogg**: alle inntekter og utgifter kronologisk med kursinformasjon for valutainntekter. Eksporteres som Excel og CSV.

------

## Design og UX

### Prinsipp: Apple, ikke Microsoft

- Én oppgave per skjerm
- Default-verdier er alltid riktige for den vanligste situasjonen
- Ingen obligatoriske felt utover dato, beløp og beskrivelse
- Feil vises inline, ikke i popup
- Lagring bekreftes med én diskret animasjon, ingen modal
- Tilbake-knapp finnes alltid

### Visuelt

- Rent hvitt bakgrunn (#FFFFFF) med lys grå separator (#F5F5F7)
- Primærfarge: dyp marineblå (#1D3557) for norsk/profesjonell stemning
- Aksent: varm gull (#C9A84C) for norske kroner og positive tall
- Rød for utgifter og negative tall (#D62828)
- Skrift: Inter (via Google Fonts) eller systemfonten `-apple-system, BlinkMacSystemFont`
- Ingen border-radius over 8px
- Ingen shadows utover én subtil card-shadow: `0 1px 3px rgba(0,0,0,0.08)`
- Tabeller er rene og luftige, ingen zebra-striping

### Responsivitet

Appen er primært desktop (1280px+), men skal ikke bryte på 768px (nettbrett). Mobil er ikke prioritert.

------

## Flerspråklig

All tekst i UI skal ligge i én `i18n`-fil per språk:

- `web/i18n/nb.json` (norsk bokmål, primær)
- `web/i18n/pt.json` (portugisisk, sekundær)
- `web/i18n/en.json` (engelsk, tertiær)

Språkvalg lagres i `config`-tabellen. Bytte av språk skjer uten å restarte appen.

------

## Første oppstart

Ved første kjøring skal appen:

1. Vise en enkel velkomstskjerm på norsk
2. Be om å velge (eller opprette) en `data/`-mappe (gjerne i OneDrive)
3. Opprette SQLite-database og mapper automatisk
4. Laste ned årets skatteinfo fra Skatteetaten og lagre lokalt
5. Vise dashboardet

Hele prosessen skal ta under 2 minutter og kreve null teknisk kunnskap.

------

## Bygg og distribusjon

```bash
# bygg alle plattformer
GOOS=windows GOARCH=amd64 go build -o dist/enk-regnskap-windows.exe ./cmd/server
GOOS=darwin  GOARCH=arm64 go build -o dist/enk-regnskap-mac     ./cmd/server
GOOS=linux   GOARCH=amd64 go build -o dist/enk-regnskap-linux    ./cmd/server
```

På macOS og Windows: ved oppstart sjekker appen om port 7331 er ledig, starter HTTP-serveren, og åpner `http://localhost:7331` i standard nettleser automatisk.

En enkel `Makefile` skal inneholde `make build`, `make run`, og `make dev` (med hot reload via `air`).

------

## Utenlandsk beskatning og kreditfradrag (skatteavtalen Norge-Brasil + sktl. § 16-20)

Dette er et kjerneområde for denne appen siden Paula har inntekter fra Brasil som allerede er beskattet der.

**Norge og Brasil fikk en ny skatteavtale som trådte i kraft 30. desember 2024.** Avtalen ble undertegnet 4. november 2022 (Prop. 13 S (2022-2023)) og gjelder fra inntektsåret 2025. For inntektsår før 2025 gjelder intern norsk rett alene (sktl. § 16-20 til § 16-28). Appen må håndtere begge regimer.

Datamodellen og rapportene må skille tydelig mellom:

- Inntektsår 2024 og tidligere: kreditfradrag etter sktl. § 16-20 (ingen skatteavtale)
- Inntektsår 2025 og senere: kreditfradrag via skatteavtalen Norge-Brasil (kreditmetoden)

### Hva agenten må sette seg inn i før koding

Les og analyser følgende før du designer datamodellen:

- Skatteavtalen mellom Norge og Brasil, undertegnet 4. november 2022, i kraft 30. desember 2024. Tilgjengelig på Lovdata: søk etter "overenskomst Norge Brasil 2022"
- Avtalens artikler om virksomhetsinntekt og selvstendig personlig virksomhet (frilansere og ENK-eiere), herunder hvilken stat som har beskatningsrett og vilkår for dette
- Skatteloven § 16-20 til § 16-28 (gjelder for inntektsår 2024 og tidligere)
- Skatteetatens veiledning om kreditfradrag: `https://www.skatteetaten.no/rettskilder/type/uttalelser/prinsipputtalelser/kreditfradrag-for-inntektsskatt-betalt-i-fremmed-stat-iht.-sktl.--16-20-flg.-og-krav-til-dokumentasjon/`
- RF-1147 (kreditfradrag for skatt betalt i utlandet, personlig skattyter)
- Maksimalt kreditfradrag beregnes per land og kan ikke overstige den norske skatten som faller forholdsmessig på samme inntekt (sktl. § 16-21). Overskytende kan fremføres i inntil 10 år (sktl. § 16-22).

### Hva appen må håndtere

**Per inntektstransaksjon fra Brasil:**

- Hvilket land inntekten stammer fra (landkode ISO 3166-1, f.eks. BR)
- Om det er betalt skatt på inntekten i kildestaten (ja/nei)
- Brasiliansk skattetype (f.eks. IRRF/kildeskatt, ISS for tjenester, CSLL)
- Hvilket rettsgrunnlag kreditfradraget bygger på ('treaty' for 2025+, 'internal' for 2024-)
- Beløp betalt i utenlandsk skatt (BRL), konvertert til NOK via Norges Bank-kurs
- Dokumentasjonstype som foreligger (skattekvittering, arbeidsgiverbekreftelse, Receita Federal-utskrift)
- Om endelig brasiliansk skatt er fastsatt (relevant for § 16-22 om fremføring)

**Separat tabell for utenlandsk skatt per inntektsår og land:**

```sql
CREATE TABLE foreign_tax_credits (
  id                INTEGER PRIMARY KEY,
  tax_year          INTEGER NOT NULL,
  country_code      TEXT NOT NULL,              -- ISO 3166-1, f.eks. 'BR'
  country_name      TEXT NOT NULL,              -- 'Brasil'
  income_nok        REAL NOT NULL,              -- total utenlandsinntekt fra dette landet (NOK)
  foreign_tax_orig  REAL NOT NULL,              -- betalt skatt i utenlandsk valuta
  foreign_currency  TEXT NOT NULL,              -- 'BRL'
  foreign_tax_nok   REAL NOT NULL,              -- konvertert til NOK
  max_credit_nok    REAL,                       -- beregnet tak (fylles inn ved årsavslutning)
  utilized_nok      REAL,                       -- faktisk benyttet kreditfradrag
  carryforward_nok  REAL DEFAULT 0,             -- fremført til neste år (§ 16-22)
  tax_finalized_abroad INTEGER DEFAULT 0,       -- 1 = endelig fastsatt i utlandet
  documentation_type TEXT,                      -- 'kvittering', 'arbeidsgiverbekreftelse', etc.
  rf1147_ready      INTEGER DEFAULT 0,          -- 1 = klar for RF-1147
  notes             TEXT,
  created_at        TEXT NOT NULL DEFAULT (datetime('now'))
);
```

Linken mellom `income`-tabellen og `foreign_tax_credits` skjer via `country_code` og `tax_year`. Alle inntektstransaksjoner fra Brasil aggregeres automatisk per år og summeres inn i denne tabellen.

### UX for utenlandsk beskatning

**Ved registrering av brasiliansk inntekt:**

Når bruker velger land = Brasil (eller hvilken som helst ikke-norsk valuta), vises automatisk et tilleggsfelt:

> "Er det trukket skatt på denne inntekten i Brasil?" [Ja, med beløp] / [Nei] / [Vet ikke ennå]

Hvis "Ja": legg inn beløp i BRL. Kursen hentes automatisk fra Norges Bank for inntektsdatoen.

**Egen side: "Utenlandsk skatt"**

En dedikert side (ikke gjemt i innstillinger) med:

- Oversikt per land og år: inntekt, betalt skatt, beregnet maksimalt kreditfradrag, benyttet, fremført
- Statusindikator per år: "Dokumentasjon mangler" / "Klar for RF-1147" / "Levert"
- Sjekkliste for hva som trengs av dokumentasjon til Skatteetaten (tilpasset Brasil):
  - Kvittering eller erklæring fra Receita Federal eller arbeidsgiver
  - Dokumentasjon på skattetype (IRRF, ISS, etc.) og om den er beregnet på netto eller brutto
  - Bekreftelse på at skatten er endelig fastsatt (eller status hvis ikke)
- Nedlastbar oppsummering som vedlegg til selvangivelsen (PDF)
- Lenke til RF-1147 på Skatteetatens sider

**I årsrapporten:**

Et eget kapittel "Utenlandsinntekter og kreditfradrag" som viser:

- Samlet inntekt per land i NOK
- Betalt utenlandsk skatt per land i NOK
- Beregnet maksimalt kreditfradrag (§ 16-21)
- Faktisk benyttet kreditfradrag
- Fremført beløp til neste år (§ 16-22) med frist (10 år)
- Referanse til RF-1147 og hvilke poster som skal fylles inn

### Viktige advarsler å bygge inn i appen

Appen skal vise synlige varsler (ikke modal, men inline "info-kort") i relevante kontekster:

1. Ved registrering av utenlandsinntekt uten dokumentert skatt:

   > "Inntekt fra Brasil uten trukket kildeskatt er fortsatt fullt skattepliktig i Norge. Du kan ikke kreve kreditfradrag uten dokumentasjon på betalt brasiliansk skatt."

2. Når endelig brasiliansk skatt ikke er fastsatt ved norsk selvangivelsesfrist:

   > "Du kan kreve kreditfradrag senest 6 måneder etter at endelig skatt er fastsatt i Brasil (sktl. § 16-28). Merk dette i appen så du ikke går glipp av fristen."

3. Generell advarsel på utenlandsk skatt-siden:

   > "Reglene for kreditfradrag uten skatteavtale er komplekse. For større beløp anbefales det å konsultere en skatterådgiver."

Appen skal aldri beregne eller garantere det endelige kreditfradraget, men gi bruker all informasjon og dokumentasjon de trenger til å fylle inn RF-1147 selv eller gi til en regnskapsfører.

------

## Landdata: skatteregler og avtaler i databasen

Appen lagrer skatteregler og avtaleinformasjon for relevante land direkte i SQLite. Dette gjør at appen kan vise kontekstsensitiv informasjon, generere riktige advarsler og tilpasse rapportene uten å måtte hente data fra nett under selvangivelsen.

### Seed-data som må ligge i databasen fra starten

Agenten skal generere og kjøre en `seed_country_data.sql`-migrasjon som populerer `country_tax_rules` og `country_tax_types` for alle land appen initialt støtter.

**Minstekrav ved første versjon: Norge og Brasil.**

For hvert land skal følgende ligge i databasen:

**`country_tax_rules` (én rad per land per periode):**

- Landkode og navn
- Om skatteavtale med Norge eksisterer, fra hvilken dato og hvilken metode (kredit/unntak)
- Referanse til avtaledokumentet (Prop.-nummer, Lovdata-URL)
- Standard kildeskattesats landet bruker på tjenesteinntekter til utlandet
- Notater om særregler relevante for ENK og frilansere

**`country_tax_types` (én rad per skattetype per land):**

For Brasil, som minimum:

| Kode   | Navn                                                   | Gjelder                   | Krediterbar i NO    |
| ------ | ------------------------------------------------------ | ------------------------- | ------------------- |
| IRRF   | Imposto de Renda Retido na Fonte                       | Tjenester, honorar        | Ja                  |
| ISS    | Imposto Sobre Serviços                                 | Kommunal tjenesteskatt    | Vurderes (se note)  |
| CSLL   | Contribuição Social sobre o Lucro Líquido              | Selskapsskatt             | Normalt nei for ENK |
| PIS    | Programa de Integração Social                          | Bidragsskatt på omsetning | Normalt nei         |
| COFINS | Contribuição para o Financiamento da Seguridade Social | Bidragsskatt              | Normalt nei         |

For Norge, som minimum:

| Kode          | Navn                                       | Gjelder                    |
| ------------- | ------------------------------------------ | -------------------------- |
| INNTEKTSSKATT | Alminnelig inntektsskatt                   | All inntekt                |
| TRINNSKATT    | Trinnskatt på personinntekt                | Personinntekt over terskel |
| TRYGDEAVGIFT  | Trygdeavgift (selvstendig næringsdrivende) | Personinntekt              |

### Vedlikehold og oppdatering

En dedikert side i appen under Skatteinfo viser landoversikten:

- Alle registrerte land med siste oppdateringsdato
- Skatteavtalestatus og metode
- Liste over skattetyper med krediterbarhetsstatus
- Knapp: "Sjekk for oppdateringer" (agenten henter fra Skatteetatens nettside og Lovdata, sammenligner med databasen og foreslår endringer som bruker må bekrefte)

Nye land kan legges til manuelt av bruker via et enkelt skjema, eller agenten kan utvides til å hente data automatisk.

------

## Agent-drevet testing: bygg, kjør og test løpende

Agenten skal ikke bare skrive kode, men aktivt verifisere at appen fungerer korrekt etter hvert steg. Dette er spesielt viktig siden appen har et grafisk grensesnitt som må valideres visuelt og funksjonelt, ikke bare enhetstestes.

### Prinsipp: test-driven development for GUI-apper

For hvert nytt skjermbilde eller brukerflyt som implementeres skal agenten:

1. Starte appen lokalt (`go run ./cmd/server` eller den kompilerte binæren)
2. Navigere til relevant side via HTTP
3. Verifisere at siden returnerer 200 OK og forventet HTML-innhold
4. Kjøre relevante integrasjonstester mot HTTP-endepunktene
5. Rapportere hva som ble testet og hva resultatet var

### Selvtestingsystem agenten skal bygge

Agenten skal implementere et **eget testbibliotek** i `internal/testing/` som lar agenten kjøre ende-til-ende-tester mot den kjørende appen. Dette er ikke Playwright eller Selenium, men et lettvekts Go-basert system spesielt tilpasset denne appen.

```
internal/
  testing/
    harness.go        # starter appen på tilfeldig port, returnerer base URL
    browser.go        # HTTP-klient som simulerer nettleserinteraksjon
    forms.go          # hjelpefunksjoner for å fylle ut og sende HTML-skjemaer
    assert.go         # enkle assertionsfunksjoner med god feilmelding
    fixtures.go       # testdata: eksempel-ENK, transaksjoner, valutakurser
    scenarios/
      onboarding_test.go      # første oppstart, datamappevalg
      income_test.go          # legg til inntekt i NOK og BRL
      expense_test.go         # legg til utgift med kvittering
      currency_test.go        # valutakonvertering mot mock Norges Bank
      foreign_tax_test.go     # brasiliansk inntekt med IRRF-dokumentasjon
      report_test.go          # generer PDF-årsrapport
      tax_info_test.go        # skatteinfo-side og landoversikt
```

**`harness.go`** starter appen på en tilfeldig ledig port (`:0`), venter til den svarer på `/health`, og returnerer base URL. Ved avslutning rydder den opp testdatabasen.

**`browser.go`** er en tynn wrapper rundt `net/http` som:

- Holder en cookie jar (session-state)
- Følger redirects
- Returnerer både HTTP-statuskode og parset HTML (via `golang.org/x/net/html`)
- Kan hente ut spesifikke DOM-elementer via CSS-selektor-lignende funksjoner

**`forms.go`** kan:

- Finne et HTML-skjema på en side ved navn eller action-URL
- Fylle inn feltverdier
- Sende skjemaet (POST) og returnere responssiden

**`fixtures.go`** inneholder realistisk testdata:

- En ENK kalt "Testforetak" med org.nr. 000000000
- 12 inntektstransaksjoner fordelt på NOK og BRL gjennom et kalenderår
- 8 utgiftstransaksjoner med ulike fradragskategorier
- Mock-valutakurser for BRL/NOK for hele testperioden (ingen nettverkskall under test)
- To brasilianske inntekter med IRRF-dokumentasjon og én uten

### Mock-tjenester

Agenten skal implementere mock-versjoner av alle eksterne tjenester slik at tester kan kjøres uten internettilgang:

```
internal/
  testing/
    mocks/
      norgesbank.go     # returnerer forhåndsdefinerte kurser uten HTTP-kall
      skatteetaten.go   # mock for nedlasting av skatteinfo-sider
```

Mock-serverne registreres via dependency injection i app-konfigurasjonen. Appen bruker et `ExchangeRateProvider`-interface, ikke en konkret implementasjon, slik at mock og ekte tjeneste er utbyttbare.

### Testscenarioer agenten skal kjøre etter hvert steg

Agenten skal kjøre følgende scenarioer i rekkefølge etter hvert som funksjonalitet bygges:

**Steg 1: Grunnleggende app starter**

- [ ] `GET /` returnerer 200 og inneholder `<nav>`
- [ ] `GET /health` returnerer `{"status":"ok"}`
- [ ] Databasen opprettes med alle tabeller ved første oppstart
- [ ] Seed-data for Norge og Brasil er tilstede i `country_tax_rules`

**Steg 2: Inntektsregistrering**

- [ ] `GET /income/new` returnerer skjema med alle påkrevde felter
- [ ] POST med gyldig NOK-inntekt oppretter rad i `income`-tabellen
- [ ] POST med BRL-inntekt henter valutakurs og beregner NOK-beløp korrekt
- [ ] POST med BRL-inntekt trigger spørsmål om brasiliansk skatt
- [ ] Valideringsfeil vises inline, siden reloades ikke

**Steg 3: Utgiftsregistrering**

- [ ] Fradragsprosent beregnes korrekt for alle kategorier
- [ ] Kvitteringsopplasting lagrer fil i riktig mappestruktur

**Steg 4: Utenlandsk skatt**

- [ ] `GET /foreign-tax` viser korrekt aggregert brasiliansk inntekt for inntektsåret
- [ ] Kreditbasis vises korrekt ('treaty' for 2025, 'internal' for 2024)
- [ ] Sjekkliste for dokumentasjon vises med riktige krav

**Steg 5: Rapporter**

- [ ] PDF-årsrapport genereres uten feil og er en gyldig PDF-fil (sjekk magic bytes)
- [ ] Næringsspesifikasjon-PDF inneholder alle poster med korrekte totaler
- [ ] Excel-eksport er en gyldig .xlsx-fil som kan åpnes

**Steg 6: Skatteinfo og landoversikt**

- [ ] Brasil-siden viser alle skattetyper med krediterbarhetsstatus
- [ ] Skatteavtale-informasjon vises med korrekt ikrafttredelsesdato (30. des. 2024)

### Rapportering etter testing

Etter hvert teststeg skriver agenten en kort oppsummering:

```
TESTSTEG 2 FULLFØRT
Kjørte: 5 scenarioer
Bestod: 4
Feilet: 1 → "BRL-inntekt trigger ikke brasiliansk skatt-spørsmål"
Neste steg: fiks handler/income.go linje 87, re-kjør scenario
```

Agenten skal ikke gå videre til neste funksjonalitet før alle tester for inneværende steg er grønne.

### Visuell validering via HTML-snapshot

Siden agenten ikke kan se skjermbilder direkte, bruker den HTML-snapshots for å validere layout og innhold:

- Etter hver side er implementert, lagres en `testdata/snapshots/SIDENAVN.html` med forventet HTML-struktur (uten dynamisk data)
- `assert.HTMLContains(t, resp, "selector", "expected text")` sjekker at et element med gitt ID eller klasse inneholder forventet tekst
- Kritiske UI-elementer som "Legg til inntekt"-knappen, navigasjonsmenyen og feilmeldingsdisplay verifiseres eksplisitt i hvert scenario

------

## GitHub og versjonskontroll

Prosjektet lever i et **offentlig GitHub-repository**. Dette har to kritiske konsekvenser som agenten må respektere absolutt gjennom hele prosjektet.

### Absolutt regel: ingen sensitiv informasjon i repoet

Følgende skal aldri committes, uansett kontekst:

- Personopplysninger (navn, adresse, fødselsnummer, org.nr. for reelle personer)
- API-nøkler, tokens eller passord av noe slag
- Faktiske transaksjonsdata, kvitteringer eller regnskapsdata
- Databasefiler (`*.db`, `*.sqlite`)
- Lokal konfigurasjon med stier eller brukernavn
- Noe som kan identifisere den faktiske brukeren av appen

`.gitignore` skal opprettes i fase 0 og aldri endres uten eksplisitt instruksjon.

Testdata og fixtures bruker **alltid fiktive verdier**: fiktivt firma, fiktive beløp, fiktive klientnavn. Ingen testdata skal ligne på reelle data fra brukeren.

### Commit-disiplin

Hvert commit skal:

- Tilhøre én konkret fase eller et konkret teststeg
- Ha en beskrivende melding på engelsk: `feat:`, `test:`, `fix:`, `docs:`, `chore:`
- Kun inneholde filer som er relevante for det steget
- Alltid kompilere (`go build` skal ikke feile)
- Alltid ha grønne tester for alle faser som er fullført

Agenten committer aldri "work in progress" som ikke kompilerer eller har røde tester.

### Branch-strategi

```
main          # alltid stabil, kun ferdig og testet kode
dev           # aktiv utviklingsbranch, agenten jobber her
fase/XX-navn  # kortlivede feature-branches per fase (valgfritt)
```

Agenten åpner en pull request fra `dev` til `main` ved slutten av hver fase, med en kort beskrivelse av hva som er implementert og hvilke tester som ble kjørt.

------

## Faseinndelt utvikling

Agenten bygger appen i strikt rekkefølge. Hver fase avsluttes med: alle tester grønne, koden kompilerer, commit til GitHub, kort statusrapport til brukeren.

Agenten skal ikke starte en ny fase uten at forrige fase er fullstendig avsluttet og bekreftet av brukeren.

------

### Fase 0: Infrastruktur og oppsett (gjøres sammen med brukeren)

**Mål:** alt nødvendig er på plass før én linje applikasjonskode skrives.

Dette er den eneste fasen som krever aktiv input fra brukeren underveis.

**Agenten gjør følgende, i rekkefølge:**

1. **Spør brukeren** om følgende og vent på svar før neste steg:

   - GitHub-brukernavn og ønsket repo-navn (f.eks. `enk-regnskap`)
   - Hvilket operativsystem utvikles på (macOS/Windows/Linux)
   - Om Go allerede er installert (og versjon)
   - Om `git` er konfigurert med navn og e-post

2. **Verifiser Go-installasjon:**

   ```bash
   go version   # må være 1.22+
   ```

   Hvis ikke installert: gi installasjonsinstruksjoner for brukerens OS.

3. **Installer nødvendige verktøy:**

   ```bash
   go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
   go install github.com/air-verse/air@latest          # hot reload under utvikling
   go install golang.org/x/tools/cmd/goimports@latest
   ```

4. **Opprett lokal mappestruktur:**

   ```bash
   mkdir enk-regnskap && cd enk-regnskap
   git init
   git checkout -b dev
   ```

5. **Opprett `.gitignore` som det aller første:**

   ```gitignore
   # Data og database - skal aldri i repoet
   data/
   *.db
   *.sqlite
   *.sqlite3
   
   # Lokale konfigurasjonsfiler
   .env
   *.env
   config.local.*
   enk-regnskap.config.json
   
   # Bygg-output
   dist/
   bin/
   tmp/
   
   # OS-filer
   .DS_Store
   Thumbs.db
   desktop.ini
   
   # IDE
   .vscode/settings.json
   .idea/
   *.swp
   
   # Test-output
   testdata/snapshots/*.actual.html
   coverage.out
   ```

6. **Initialiser Go-modulen:**

   ```bash
   go mod init github.com/BRUKERNAVN/enk-regnskap
   ```

7. **Opprett GitHub-repoet:**

   - Agenten gir brukeren eksakt URL og kommandoer for å opprette et tomt offentlig repo på github.com (agenten kan ikke gjøre dette selv)
   - Brukeren oppretter repoet uten README, uten .gitignore, uten lisens (alt dette legges til av agenten)

8. **Koble lokalt repo til GitHub:**

   ```bash
   git remote add origin https://github.com/BRUKERNAVN/enk-regnskap.git
   git push -u origin dev
   ```

9. **Opprett README.md** med:

   - Prosjektbeskrivelse (ingen personopplysninger)
   - Teknisk stack-oversikt
   - Instruksjoner for å bygge og kjøre lokalt
   - Tydelig merknad om at dette er et personlig verktøy og ikke juridisk rådgivning

10. **Opprett `go.sum`-fil og last ned avhengigheter:**

    ```bash
    go get modernc.org/sqlite
    go get github.com/go-chi/chi/v5
    go get github.com/go-pdf/fpdf
    go get github.com/xuri/excelize/v2
    go mod tidy
    ```

11. **Første commit og push:**

    ```bash
    git add .gitignore go.mod go.sum README.md
    git commit -m "chore: initial project setup with dependencies"
    git push
    ```

12. **Verifiser at repoet ser riktig ut på GitHub** (agenten ber brukeren bekrefte at ingen sensitiv info er synlig).

**Fase 0 er fullført når:** repoet er offentlig tilgjengelig på GitHub, `.gitignore` er på plass, alle avhengigheter er installert, og brukeren har bekreftet at alt ser korrekt ut.

------

### Fase 1: Databaselag og dataskjema

**Mål:** SQLite-database med alle tabeller, migrasjoner og seed-data. Ingen HTTP, ingen UI.

**Agenten gjør:**

- Oppretter `internal/db/schema.sql` med alle tabeller fra databasemodellen
- Oppretter `internal/db/seed_country_data.sql` med Norge og Brasil
- Implementerer `internal/db/db.go` med migrasjonssystem (kjør schema ved oppstart)
- Implementerer `internal/db/seed.go` som populerer country-tabellene
- Kjører `sqlc generate` og verifiserer at koden kompilerer

**Tester:**

- Alle tabeller opprettes uten feil
- Seed-data for Norge og Brasil er korrekt etter kjøring
- Migrasjonskjøring er idempotent (kan kjøres flere ganger uten feil)

**Commit:** `feat: database schema, migrations and country seed data`

------

### Fase 2: Valuta og Norges Bank-integrasjon

**Mål:** hent og cache valutakurser. Ingen UI, men testbart via HTTP-endepunkt.

**Agenten gjør:**

- Implementerer `internal/currency/norgesbank.go` med `ExchangeRateProvider`-interface
- Implementerer `internal/testing/mocks/norgesbank.go`
- Eksponerer `GET /api/exchange-rate?currency=BRL&date=2025-01-15` for testing

**Tester:**

- Mock returnerer forventet kurs for BRL/NOK
- Cache-logikk: kurs hentes én gang, andre kall bruker cache
- Helg/helligdag: returnerer nærmeste foregående bankdag-kurs

**Commit:** `feat: exchange rate provider with Norges Bank integration and mock`

------

### Fase 3: HTTP-server, routing og grunnleggende UI-rammeverk

**Mål:** appen starter, åpner nettleser, viser en tom men korrekt layoutet forside.

**Agenten gjør:**

- Implementerer `cmd/server/main.go`
- Setter opp Chi-router med alle ruter (handlers returnerer foreløpig 501)
- Implementerer `web/templates/layout.html` med navigasjon
- Implementerer `GET /health`
- Embedder `web/`-mappen i binæren

**Tester (første bruk av testharness):**

- `GET /` returnerer 200 og korrekt HTML-struktur
- `GET /health` returnerer `{"status":"ok"}`
- Navigasjonslenker til alle planlagte sider er til stede i DOM
- Appen starter og stopper rent

**Commit:** `feat: HTTP server, routing and base UI layout`

------

### Fase 4: Inntektsregistrering

**Mål:** bruker kan logge en inntekt i NOK eller utenlandsk valuta.

**Agenten gjør:**

- Implementerer `handlers/income.go` (GET og POST)
- Implementerer `web/templates/income.html`
- Valuta-dropdown med automatisk kursoppslag via AJAX mot `/api/exchange-rate`
- Spørsmål om brasiliansk skatt vises dynamisk når land = BR
- Inline-validering

**Tester:**

- Alle scenarioer fra Steg 2 i testplanen
- Edge case: BRL-inntekt på helg bruker riktig kurs
- Edge case: ugyldig valuta gir feilmelding

**Commit:** `feat: income registration with multi-currency support`

------

### Fase 5: Utgiftsregistrering og kvitteringer

**Mål:** bruker kan logge utgifter med fradragskategorier og laste opp kvitteringer.

**Agenten gjør:**

- Implementerer `handlers/expenses.go` og `handlers/receipts.go`
- Implementerer `web/templates/expenses.html` og `web/templates/receipts.html`
- Filhåndtering: lagrer til `data/receipts/ÅÅÅÅ/` med UUID-basert filnavn
- Inline forhåndsvisning av bilde/PDF

**Tester:**

- Alle scenarioer fra Steg 3 i testplanen
- Kvittering lagres på korrekt sti
- Feil filtype gir feilmelding

**Commit:** `feat: expense registration with receipt upload`

------

### Fase 6: Utenlandsk skatt og kreditfradrag

**Mål:** dedikert side for brasiliansk skatt, sjekkliste og aggregering per år.

**Agenten gjør:**

- Implementerer `handlers/foreign_tax.go`
- Implementerer `web/templates/foreign_tax.html`
- Logikk for å velge riktig rettsgrunnlag basert på inntektsår (2024- vs 2025+)
- Sjekkliste for dokumentasjon genereres dynamisk fra `country_tax_types`

**Tester:**

- Alle scenarioer fra Steg 4 i testplanen
- Korrekt rettsgrunnlag vises for 2024 vs 2025
- Aggregering summerer alle BRL-inntekter korrekt

**Commit:** `feat: foreign tax tracking and credit deduction overview`

------

### Fase 7: Dashboard

**Mål:** oppsummeringsside med nøkkeltall og snarveier.

**Tester:**

- Totaler reflekterer alle registrerte transaksjoner
- Knapper navigerer til riktige sider

**Commit:** `feat: dashboard with YTD summary`

------

### Fase 8: Skatteinfo og landoversikt

**Mål:** bruker kan se skatteregler per land og laste ned skatteinfo fra Skatteetaten.

**Tester:**

- Brasil-siden viser alle skattetyper fra seed-data
- Skatteavtale-dato vises korrekt

**Commit:** `feat: tax info page with country overview`

------

### Fase 9: Rapporter og PDF-eksport

**Mål:** årsrapport og næringsspesifikasjon som PDF og Excel.

**Tester:**

- Alle scenarioer fra Steg 5 i testplanen
- PDF magic bytes er korrekte (`%PDF`)
- Excel er gyldig `.xlsx`
- Totaler i rapporten stemmer med databasen

**Commit:** `feat: annual report and tax summary PDF/Excel export`

------

### Fase 10: Første oppstart og onboarding

**Mål:** ny bruker kan sette opp appen på under 2 minutter.

**Tester:**

- Onboarding-flyt fra Steg 1 i testplanen
- Datamappen opprettes korrekt
- Appen starter normalt etter onboarding

**Commit:** `feat: first-run onboarding flow`

------

### Fase 11: Flerspråklig støtte

**Mål:** norsk, portugisisk og engelsk via i18n-filer.

**Commit:** `feat: i18n support (nb, pt, en)`

------

### Fase 12: Bygg og distribusjon

**Mål:** kompilerte binærer for Windows, macOS og Linux.

**Agenten gjør:**

- Skriver `Makefile` med `make build`, `make run`, `make dev`, `make test`
- Setter opp GitHub Actions workflow for automatisk bygg ved push til `main`
- CI kjører `go test ./...` og verifiserer at binærene kompilerer

**Viktig om GitHub Actions:** workflow-filen skal ikke inneholde noen secrets, tokens eller konfigurasjon som er spesifikk for brukeren. Bare standard Go build/test.

**Commit:** `chore: Makefile and GitHub Actions CI pipeline` **Pull request:** `dev` → `main` med fullstendig endringslogg

------

## Ikke i scope (første versjon)

- MVA-melding og MVA-regnskap
- Lønn og arbeidsgiveravgift
- Fakturagenerering
- Bank-API-integrasjon
- Flerbrukerstøtte
- Skylagring

Disse kan legges til i en senere versjon uten å endre kjernearkitekturen.