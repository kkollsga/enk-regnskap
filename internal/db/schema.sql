-- ENK Regnskap - databaseskjema
-- Alle CREATE-setninger er idempotente (IF NOT EXISTS) slik at migrasjonen
-- trygt kan kjores ved hver oppstart.

-- Kvitteringer (opprettes for income/expenses som refererer den)
CREATE TABLE IF NOT EXISTS receipts (
  id            INTEGER PRIMARY KEY,
  filename      TEXT NOT NULL,          -- relativ sti fra data/receipts/
  original_name TEXT NOT NULL,
  mime_type     TEXT NOT NULL,
  title         TEXT,                   -- valgfri tittel på vedlegget
  description   TEXT,                   -- valgfri beskrivelse
  parent_kind   TEXT,                   -- 'income' | 'expense' (hvilken post)
  parent_id     INTEGER,                -- id i income/expenses
  tax_year      INTEGER,                -- året kvitteringen ble lastet opp for
  uploaded_at   TEXT NOT NULL DEFAULT (datetime('now'))
);
-- MERK: indeksen på (parent_kind, parent_id) opprettes i Go ETTER at
-- kolonnene er lagt til (ensureColumn), slik at eldre databaser uten disse
-- kolonnene ikke feiler her.

-- Inntekter
CREATE TABLE IF NOT EXISTS income (
  id            INTEGER PRIMARY KEY,
  date          TEXT NOT NULL,          -- ISO 8601
  description   TEXT NOT NULL,
  amount_orig   REAL NOT NULL,          -- beløp i original valuta
  currency      TEXT NOT NULL DEFAULT 'NOK',
  exchange_rate REAL,                   -- hentet fra Norges Bank, NULL hvis NOK
  rate_date     TEXT,                   -- hvilken dato kursen gjelder for
  amount_nok    REAL NOT NULL,          -- amount_orig * exchange_rate
  category      TEXT NOT NULL,          -- 'tjenesteinntekt', 'honorar', etc.
  client        TEXT,
  country_code  TEXT NOT NULL DEFAULT 'NO', -- ISO 3166-1, kildeland for inntekten
  foreign_tax_paid     INTEGER NOT NULL DEFAULT 0, -- 0=nei, 1=ja, 2=vet ikke enna
  receipt_id    INTEGER REFERENCES receipts(id),
  tax_year      INTEGER NOT NULL,
  notes         TEXT,
  created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Utenlandsk skatt trukket på en inntekt, brutt ned per skattetype (f.eks.
-- IRRF, ISS, CSLL). En inntekt kan ha flere linjer. Beløpet konverteres til
-- NOK på inntektens kursdato. Dette er kilden til alle aggregater av betalt
-- utenlandsk skatt (fullt normalisert; income har ingen flate skattekolonner).
CREATE TABLE IF NOT EXISTS income_foreign_taxes (
  id          INTEGER PRIMARY KEY,
  income_id   INTEGER NOT NULL REFERENCES income(id) ON DELETE CASCADE,
  tax_type    TEXT NOT NULL,            -- f.eks. 'IRRF', 'ISS', 'CSLL'
  amount_orig REAL NOT NULL,            -- betalt skatt i utenlandsk valuta
  currency    TEXT NOT NULL,            -- valuta for skattebeløpet
  amount_nok  REAL NOT NULL,            -- konvertert til NOK
  treatment   TEXT NOT NULL DEFAULT 'credit' -- 'credit'=kreditfradrag, 'deduct'=fradragsberettiget kostnad, 'none'=kun referanse
);

-- Utgifter
CREATE TABLE IF NOT EXISTS expenses (
  id             INTEGER PRIMARY KEY,
  date           TEXT NOT NULL,
  description    TEXT NOT NULL,
  amount_orig    REAL NOT NULL DEFAULT 0, -- beløp i original valuta
  currency       TEXT NOT NULL DEFAULT 'NOK',
  exchange_rate  REAL,                  -- Norges Bank-kurs, NULL hvis NOK
  rate_date      TEXT,                  -- dato kursen gjelder for
  country_code   TEXT NOT NULL DEFAULT 'NO',
  amount_nok     REAL NOT NULL,         -- amount_orig * exchange_rate
  category       TEXT NOT NULL,         -- kobles til tax/rules_AAAA.go
  deductible_pct REAL NOT NULL DEFAULT 100.0,
  deductible_nok REAL NOT NULL,         -- amount_nok * deductible_pct / 100
  receipt_id     INTEGER REFERENCES receipts(id),
  tax_year       INTEGER NOT NULL,
  notes          TEXT,
  created_at     TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Valutakurser (cache)
CREATE TABLE IF NOT EXISTS exchange_rates (
  currency  TEXT NOT NULL,
  date      TEXT NOT NULL,              -- ISO 8601
  rate_nok  REAL NOT NULL,              -- 1 enhet valuta = rate_nok NOK
  source    TEXT NOT NULL DEFAULT 'norges-bank',
  PRIMARY KEY (currency, date)
);

-- Skatteregler per land (lastes ned og vedlikeholdes i appen)
CREATE TABLE IF NOT EXISTS country_tax_rules (
  id                   INTEGER PRIMARY KEY,
  country_code         TEXT NOT NULL,   -- ISO 3166-1, f.eks. 'BR', 'NO'
  country_name         TEXT NOT NULL,
  effective_from       INTEGER NOT NULL, -- inntektsår regelen gjelder fra
  effective_to         INTEGER,          -- NULL = fortsatt gjeldende
  has_tax_treaty       INTEGER NOT NULL DEFAULT 0, -- 1 = skatteavtale med Norge
  treaty_in_force_date TEXT,             -- ISO 8601, dato avtalen tradte i kraft
  treaty_method        TEXT,             -- 'credit', 'exemption', NULL hvis ingen avtale
  treaty_reference     TEXT,             -- f.eks. 'Prop. 13 S (2022-2023)'
  treaty_source_url    TEXT,             -- lenke til avtaleteksten på Lovdata
  standard_withholding_pct REAL,         -- standard kildeskattesats landet bruker
  notes                TEXT,             -- fritekst om særregler
  last_updated         TEXT NOT NULL DEFAULT (datetime('now')),
  UNIQUE(country_code, effective_from)
);

-- Skattetyper per land (IRRF, ISS, CSLL, etc.)
CREATE TABLE IF NOT EXISTS country_tax_types (
  id               INTEGER PRIMARY KEY,
  country_code     TEXT NOT NULL,
  tax_type_code    TEXT NOT NULL,        -- f.eks. 'IRRF', 'ISS', 'CSLL'
  tax_type_name    TEXT NOT NULL,        -- fullt navn
  description      TEXT,                 -- forklaring på norsk
  applies_to       TEXT,                 -- 'tjenester', 'lønn', 'utbytte', etc.
  is_creditable_in_norway INTEGER DEFAULT 1, -- 1 = godkjent for kreditfradrag i Norge
  basis            TEXT,                 -- 'netto' eller 'brutto'
  typical_rate_pct REAL,
  effective_from   INTEGER NOT NULL,
  effective_to     INTEGER,
  UNIQUE(country_code, tax_type_code, effective_from)
);

-- Utenlandsk skatt per inntektsår og land (kreditfradrag)
CREATE TABLE IF NOT EXISTS foreign_tax_credits (
  id                   INTEGER PRIMARY KEY,
  tax_year             INTEGER NOT NULL,
  country_code         TEXT NOT NULL,    -- ISO 3166-1, f.eks. 'BR'
  country_name         TEXT NOT NULL,    -- 'Brasil'
  income_nok           REAL NOT NULL,    -- total utenlandsinntekt fra dette landet (NOK)
  foreign_tax_orig     REAL NOT NULL,    -- betalt skatt i utenlandsk valuta
  foreign_currency     TEXT NOT NULL,    -- 'BRL'
  foreign_tax_nok      REAL NOT NULL,    -- konvertert til NOK
  max_credit_nok       REAL,             -- beregnet tak (fylles inn ved årsavslutning)
  utilized_nok         REAL,             -- faktisk benyttet kreditfradrag
  carryforward_nok     REAL DEFAULT 0,   -- fremfort til neste år (sktl. § 16-22)
  tax_finalized_abroad INTEGER DEFAULT 0,-- 1 = endelig fastsatt i utlandet
  documentation_type   TEXT,             -- 'kvittering', 'arbeidsgiverbekreftelse', etc.
  legal_basis          TEXT,             -- 'treaty' (2025+) eller 'internal' (2024-)
  rf1147_ready         INTEGER DEFAULT 0,-- 1 = klar for RF-1147
  notes                TEXT,
  created_at           TEXT NOT NULL DEFAULT (datetime('now')),
  UNIQUE(tax_year, country_code)
);

-- App-konfigurasjon
CREATE TABLE IF NOT EXISTS config (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

-- Endringslogg / revisjonsspor for angring (rollback).
-- Hver mutasjon (fra web eller MCP-agent) logges her med for- og etter-tilstand
-- slik at enhver endring kan rulles tilbake.
CREATE TABLE IF NOT EXISTS change_log (
  id          INTEGER PRIMARY KEY,
  ts          TEXT NOT NULL DEFAULT (datetime('now')),
  actor       TEXT NOT NULL,           -- 'web', 'mcp', 'system'
  operation   TEXT NOT NULL,           -- 'insert', 'update', 'delete'
  entity      TEXT NOT NULL,           -- tabellnavn, f.eks. 'income'
  entity_id   INTEGER,                 -- rad-id i entity-tabellen
  before_json TEXT,                    -- tilstand for endring (NULL ved insert)
  after_json  TEXT,                    -- tilstand etter endring (NULL ved delete)
  description TEXT NOT NULL,           -- lesbar beskrivelse av endringen
  rolled_back INTEGER NOT NULL DEFAULT 0, -- 1 = denne endringen er rullet tilbake
  rollback_of INTEGER                  -- id-en til endringen denne rullet tilbake (hvis noen)
);

CREATE INDEX IF NOT EXISTS idx_changelog_entity ON change_log(entity, entity_id);
CREATE INDEX IF NOT EXISTS idx_changelog_ts     ON change_log(ts);

-- Indekser for vanlige oppslag
CREATE INDEX IF NOT EXISTS idx_income_tax_year     ON income(tax_year);
CREATE INDEX IF NOT EXISTS idx_income_country      ON income(country_code, tax_year);
CREATE INDEX IF NOT EXISTS idx_expenses_tax_year   ON expenses(tax_year);
CREATE INDEX IF NOT EXISTS idx_income_receipt      ON income(receipt_id);
CREATE INDEX IF NOT EXISTS idx_inc_ftax_income     ON income_foreign_taxes(income_id);
CREATE INDEX IF NOT EXISTS idx_expenses_receipt    ON expenses(receipt_id);
CREATE INDEX IF NOT EXISTS idx_rates_lookup        ON exchange_rates(currency, date);
