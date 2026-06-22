package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kkollsga/enk-regnskap/internal/db"
)

// Mirror-laget skriver en menneskelesbar kopi av kjernedataene (inntekter,
// utgifter, kvitteringer, konfigurasjon) til data/mirror/ som JSON + filer.
// Dette er et ekstra sikkerhetsnett for beta-programvare: ingen
// angre-historikk, men appen kan importere mappen for å sette tilstand.
//
// Layout:
//   data/mirror/
//     income.json      (alle inntekter, lesbar JSON)
//     expenses.json    (alle utgifter)
//     receipts.json    (kvitteringsmetadata)
//     config.json      (appinnstillinger)
//     receipts/...     (kopier av kvitteringsfilene)
//     README.txt

// MirrorDir returnerer stien til mirror-mappen (tom hvis ingen datamappe).
func (a *App) MirrorDir() string {
	if a.DataDir == "" {
		return ""
	}
	return filepath.Join(a.DataDir, "mirror")
}

// --- lesbare DTO-er (uten sql.Null-stoy) ---

type mirrorIncome struct {
	ID                 int64    `json:"id"`
	Date               string   `json:"date"`
	Description        string   `json:"description"`
	AmountOriginal     float64  `json:"amount_original"`
	Currency           string   `json:"currency"`
	ExchangeRate       *float64 `json:"exchange_rate"`
	RateDate           *string  `json:"rate_date"`
	AmountNOK          float64  `json:"amount_nok"`
	Category           string   `json:"category"`
	Client             string   `json:"client"`
	CountryCode        string   `json:"country_code"`
	ForeignTaxPaid     int64    `json:"foreign_tax_paid"`
	ForeignTaxOriginal *float64 `json:"foreign_tax_original"`
	ForeignTaxCurrency *string  `json:"foreign_tax_currency"`
	ForeignTaxNOK      *float64 `json:"foreign_tax_nok"`
	ForeignTaxType     *string  `json:"foreign_tax_type"`
	ReceiptID          *int64   `json:"receipt_id"`
	TaxYear            int64    `json:"tax_year"`
	Notes              string   `json:"notes"`
	CreatedAt          string   `json:"created_at"`
}

type mirrorExpense struct {
	ID            int64   `json:"id"`
	Date          string  `json:"date"`
	Description   string  `json:"description"`
	AmountNOK     float64 `json:"amount_nok"`
	Category      string  `json:"category"`
	DeductiblePct float64 `json:"deductible_pct"`
	DeductibleNOK float64 `json:"deductible_nok"`
	ReceiptID     *int64  `json:"receipt_id"`
	TaxYear       int64   `json:"tax_year"`
	Notes         string  `json:"notes"`
	CreatedAt     string  `json:"created_at"`
}

type mirrorReceipt struct {
	ID           int64  `json:"id"`
	Filename     string `json:"filename"`
	OriginalName string `json:"original_name"`
	MimeType     string `json:"mime_type"`
	TaxYear      *int64 `json:"tax_year"`
	UploadedAt   string `json:"uploaded_at"`
}

// SyncMirror skriver en fersk, fullstendig mirror fra databasen.
func (a *App) SyncMirror(ctx context.Context) error {
	dir := a.MirrorDir()
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Join(dir, "receipts"), 0o755); err != nil {
		return err
	}

	income, err := a.Q.ListAllIncome(ctx)
	if err != nil {
		return err
	}
	mi := make([]mirrorIncome, 0, len(income))
	for _, in := range income {
		mi = append(mi, mirrorIncome{
			ID: in.ID, Date: in.Date, Description: in.Description,
			AmountOriginal: in.AmountOrig, Currency: in.Currency,
			ExchangeRate: nfPtr(in.ExchangeRate), RateDate: nsPtr(in.RateDate),
			AmountNOK: in.AmountNok, Category: in.Category, Client: nsVal(in.Client),
			CountryCode: in.CountryCode, ForeignTaxPaid: in.ForeignTaxPaid,
			ForeignTaxOriginal: nfPtr(in.ForeignTaxOrig), ForeignTaxCurrency: nsPtr(in.ForeignTaxCurrency),
			ForeignTaxNOK: nfPtr(in.ForeignTaxNok), ForeignTaxType: nsPtr(in.ForeignTaxType),
			ReceiptID: niPtr(in.ReceiptID), TaxYear: in.TaxYear, Notes: nsVal(in.Notes),
			CreatedAt: in.CreatedAt,
		})
	}
	if err := writeJSONFile(filepath.Join(dir, "income.json"), mi); err != nil {
		return err
	}

	expenses, err := a.Q.ListAllExpenses(ctx)
	if err != nil {
		return err
	}
	me := make([]mirrorExpense, 0, len(expenses))
	for _, ex := range expenses {
		me = append(me, mirrorExpense{
			ID: ex.ID, Date: ex.Date, Description: ex.Description, AmountNOK: ex.AmountNok,
			Category: ex.Category, DeductiblePct: ex.DeductiblePct, DeductibleNOK: ex.DeductibleNok,
			ReceiptID: niPtr(ex.ReceiptID), TaxYear: ex.TaxYear, Notes: nsVal(ex.Notes),
			CreatedAt: ex.CreatedAt,
		})
	}
	if err := writeJSONFile(filepath.Join(dir, "expenses.json"), me); err != nil {
		return err
	}

	receipts, err := a.Q.ListReceipts(ctx)
	if err != nil {
		return err
	}
	mr := make([]mirrorReceipt, 0, len(receipts))
	for _, rc := range receipts {
		mr = append(mr, mirrorReceipt{
			ID: rc.ID, Filename: rc.Filename, OriginalName: rc.OriginalName,
			MimeType: rc.MimeType, TaxYear: niPtr(rc.TaxYear), UploadedAt: rc.UploadedAt,
		})
		// Kopier kvitteringsfilen til mirror hvis den ikke alt finnes.
		src := filepath.Join(a.DataDir, "receipts", filepath.FromSlash(rc.Filename))
		dst := filepath.Join(dir, "receipts", filepath.FromSlash(rc.Filename))
		if err := copyFileIfMissing(src, dst); err != nil {
			return err
		}
	}
	if err := writeJSONFile(filepath.Join(dir, "receipts.json"), mr); err != nil {
		return err
	}

	cfg, err := a.Q.ListConfig(ctx)
	if err != nil {
		return err
	}
	cfgMap := map[string]string{}
	for _, c := range cfg {
		cfgMap[c.Key] = c.Value
	}
	if err := writeJSONFile(filepath.Join(dir, "config.json"), cfgMap); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "README.txt"), []byte(mirrorReadme), 0o644)
}

// syncMirrorBestEffort oppdaterer mirror uten å feile selve mutasjonen.
func (a *App) syncMirrorBestEffort(ctx context.Context) {
	if a.MirrorDir() == "" {
		return
	}
	_ = a.SyncMirror(ctx)
}

// ImportMirror setter databasens tilstand fra en mirror-mappe. Eksisterende
// inntekter, utgifter, kvitteringer og kreditfradrag erstattes. Operasjonen
// er atomisk (transaksjon) for kjernetabellene.
func (a *App) ImportMirror(ctx context.Context, dir string) error {
	var income []mirrorIncome
	var expenses []mirrorExpense
	var receipts []mirrorReceipt
	cfg := map[string]string{}
	if err := readJSONFile(filepath.Join(dir, "income.json"), &income); err != nil {
		return fmt.Errorf("les income.json: %w", err)
	}
	if err := readJSONFile(filepath.Join(dir, "expenses.json"), &expenses); err != nil {
		return fmt.Errorf("les expenses.json: %w", err)
	}
	_ = readJSONFile(filepath.Join(dir, "receipts.json"), &receipts) // valgfri
	_ = readJSONFile(filepath.Join(dir, "config.json"), &cfg)        // valgfri

	// Kopier kvitteringsfiler inn i datamappen for filene refereres.
	for _, rc := range receipts {
		src := filepath.Join(dir, "receipts", filepath.FromSlash(rc.Filename))
		dst := filepath.Join(a.DataDir, "receipts", filepath.FromSlash(rc.Filename))
		if err := copyFileIfMissing(src, dst); err != nil {
			return fmt.Errorf("kopier kvittering %s: %w", rc.Filename, err)
		}
	}

	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, t := range []string{"income", "expenses", "receipts", "foreign_tax_credits", "change_log"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+t); err != nil {
			return fmt.Errorf("tom %s: %w", t, err)
		}
	}
	for _, rc := range receipts {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO receipts (id, filename, original_name, mime_type, tax_year, uploaded_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			rc.ID, rc.Filename, rc.OriginalName, rc.MimeType, ptrArg(rc.TaxYear), rc.UploadedAt); err != nil {
			return fmt.Errorf("importer kvittering: %w", err)
		}
	}
	for _, in := range income {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO income (id, date, description, amount_orig, currency, exchange_rate,
			   rate_date, amount_nok, category, client, country_code, foreign_tax_paid,
			   foreign_tax_orig, foreign_tax_currency, foreign_tax_nok, foreign_tax_type,
			   receipt_id, tax_year, notes, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			in.ID, in.Date, in.Description, in.AmountOriginal, in.Currency, ptrArg(in.ExchangeRate),
			ptrArg(in.RateDate), in.AmountNOK, in.Category, in.Client, in.CountryCode, in.ForeignTaxPaid,
			ptrArg(in.ForeignTaxOriginal), ptrArg(in.ForeignTaxCurrency), ptrArg(in.ForeignTaxNOK),
			ptrArg(in.ForeignTaxType), ptrArg(in.ReceiptID), in.TaxYear, in.Notes, in.CreatedAt); err != nil {
			return fmt.Errorf("importer inntekt: %w", err)
		}
	}
	for _, ex := range expenses {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO expenses (id, date, description, amount_nok, category, deductible_pct,
			   deductible_nok, receipt_id, tax_year, notes, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			ex.ID, ex.Date, ex.Description, ex.AmountNOK, ex.Category, ex.DeductiblePct,
			ex.DeductibleNOK, ptrArg(ex.ReceiptID), ex.TaxYear, ex.Notes, ex.CreatedAt); err != nil {
			return fmt.Errorf("importer utgift: %w", err)
		}
	}
	for k, v := range cfg {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO config (key, value) VALUES (?, ?)
			 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, k, v); err != nil {
			return fmt.Errorf("importer config: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	// Gjenoppbygg avledede kreditfradrag for alle berorte år.
	years := map[int]bool{}
	for _, in := range income {
		years[int(in.TaxYear)] = true
	}
	for y := range years {
		if err := a.RecomputeForeignTaxCredits(ctx, y); err != nil {
			return err
		}
	}

	// Loggfor importen (uten per-rad angre).
	_, _ = a.Q.CreateChangeLog(ctx, db.CreateChangeLogParams{
		Actor: ActorSystem, Operation: "import", Entity: "mirror",
		Description: fmt.Sprintf("Importerte tilstand fra mirror: %d inntekter, %d utgifter, %d kvitteringer",
			len(income), len(expenses), len(receipts)),
	})
	a.Events.Broadcast(Event{Type: "import", Action: "import"})
	a.syncMirrorBestEffort(ctx)
	return nil
}

const mirrorReadme = `ENK Regnskap - lesbar datakopi (mirror)

Denne mappen er en menneskelesbar sikkerhetskopi av kjernedataene dine:
  income.json    - alle inntekter
  expenses.json  - alle utgifter
  receipts.json  - kvitteringsmetadata
  config.json    - appinnstillinger
  receipts/      - kopier av kvitteringsfilene

Mappen oppdateres automatisk ved hver endring. Den har INGEN angre-historikk.
Du kan importere denne mappen i appen for å sette tilstanden tilbake til det
som ligger her (dette erstatter nåværende inntekter/utgifter/kvitteringer).
`

// --- hjelpere ---

func writeJSONFile(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func readJSONFile(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func copyFileIfMissing(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil // finnes alt
	}
	in, err := os.Open(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // kildefil mangler (f.eks. in-memory) - hopp over
		}
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func nsVal(n sql.NullString) string {
	if n.Valid {
		return n.String
	}
	return ""
}
func nsPtr(n sql.NullString) *string {
	if n.Valid {
		s := n.String
		return &s
	}
	return nil
}
func nfPtr(n sql.NullFloat64) *float64 {
	if n.Valid {
		f := n.Float64
		return &f
	}
	return nil
}
func niPtr(n sql.NullInt64) *int64 {
	if n.Valid {
		i := n.Int64
		return &i
	}
	return nil
}

// ptrArg gjør en peker om til et SQL-argument (nil hvis peker er nil).
func ptrArg[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}
