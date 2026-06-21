package core

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/kkollsga/enk-regnskap/internal/db"
)

// allowedReceiptTypes er tillatte MIME-typer for kvitteringer.
var allowedReceiptTypes = map[string]string{
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
	"image/gif":       ".gif",
	"image/webp":      ".webp",
	"image/heic":      ".heic",
	"application/pdf": ".pdf",
}

// ReceiptInput er en opplastet fil.
type ReceiptInput struct {
	OriginalName string
	MimeType     string
	Data         []byte
	TaxYear      int
}

// SaveReceipt lagrer en kvitteringsfil under data/receipts/AAAA/<uuid>.<ext>
// og oppretter en rad i receipts. Filnavnet er UUID-basert for aa unnga
// kollisjoner og lekkasje av originalnavn.
func (a *App) SaveReceipt(ctx context.Context, actor string, in ReceiptInput) (*db.Receipt, error) {
	ext, ok := allowedReceiptTypes[in.MimeType]
	if !ok {
		ve := newValidation()
		ve.add("file", "Ugyldig filtype. Tillatt: bilde (JPG/PNG/GIF/WEBP/HEIC) eller PDF.")
		return nil, ve
	}
	if len(in.Data) == 0 {
		ve := newValidation()
		ve.add("file", "Tom fil.")
		return nil, ve
	}
	year := in.TaxYear
	if year == 0 {
		year = a.ActiveYear(ctx)
	}

	rel := filepath.Join(fmt.Sprintf("%d", year), uuid.NewString()+ext)
	if a.DataDir != "" {
		full := filepath.Join(a.DataDir, "receipts", rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return nil, fmt.Errorf("opprett kvitteringsmappe: %w", err)
		}
		if err := os.WriteFile(full, in.Data, 0o644); err != nil {
			return nil, fmt.Errorf("skriv kvittering: %w", err)
		}
	}

	created, err := a.Q.CreateReceipt(ctx, db.CreateReceiptParams{
		Filename:     filepath.ToSlash(rel),
		OriginalName: in.OriginalName,
		MimeType:     in.MimeType,
		TaxYear:      nullIntVal(int64(year)),
	})
	if err != nil {
		return nil, fmt.Errorf("lagre kvitteringspost: %w", err)
	}
	after, _ := a.snapshotRow(ctx, "receipts", created.ID)
	desc := fmt.Sprintf("Lastet opp kvittering: %s", in.OriginalName)
	if err := a.logChange(ctx, actor, "insert", "receipts", created.ID, nil, after, year, desc); err != nil {
		return nil, err
	}
	return &created, nil
}

// ReceiptPath returnerer den absolutte stien til en lagret kvittering.
func (a *App) ReceiptPath(rec db.Receipt) string {
	return filepath.Join(a.DataDir, "receipts", filepath.FromSlash(rec.Filename))
}

// GetReceipt henter en kvitteringspost.
func (a *App) GetReceipt(ctx context.Context, id int64) (db.Receipt, error) {
	return a.Q.GetReceipt(ctx, id)
}

// ListReceipts henter alle kvitteringer.
func (a *App) ListReceipts(ctx context.Context) ([]db.Receipt, error) {
	return a.Q.ListReceipts(ctx)
}

// ListUnlinkedReceipts henter kvitteringer uten tilknyttet transaksjon.
func (a *App) ListUnlinkedReceipts(ctx context.Context) ([]db.Receipt, error) {
	return a.Q.ListUnlinkedReceipts(ctx)
}

// LinkReceipt knytter en kvittering til en inntekt eller utgift.
// kind er "income" eller "expense".
func (a *App) LinkReceipt(ctx context.Context, actor, kind string, txID, receiptID int64) error {
	switch kind {
	case "income":
		before, err := a.snapshotRow(ctx, "income", txID)
		if err != nil || before == nil {
			return fmt.Errorf("inntekt %d finnes ikke", txID)
		}
		if err := a.Q.UpdateIncomeReceipt(ctx, db.UpdateIncomeReceiptParams{
			ReceiptID: nullIntVal(receiptID), ID: txID,
		}); err != nil {
			return err
		}
		after, _ := a.snapshotRow(ctx, "income", txID)
		return a.logChange(ctx, actor, "update", "income", txID, before, after, toInt(before["tax_year"]),
			fmt.Sprintf("Knyttet kvittering #%d til inntekt #%d", receiptID, txID))
	case "expense":
		before, err := a.snapshotRow(ctx, "expenses", txID)
		if err != nil || before == nil {
			return fmt.Errorf("utgift %d finnes ikke", txID)
		}
		if err := a.Q.UpdateExpenseReceipt(ctx, db.UpdateExpenseReceiptParams{
			ReceiptID: nullIntVal(receiptID), ID: txID,
		}); err != nil {
			return err
		}
		after, _ := a.snapshotRow(ctx, "expenses", txID)
		return a.logChange(ctx, actor, "update", "expenses", txID, before, after, toInt(before["tax_year"]),
			fmt.Sprintf("Knyttet kvittering #%d til utgift #%d", receiptID, txID))
	default:
		return fmt.Errorf("ukjent transaksjonstype %q", kind)
	}
}

// IsImageReceipt forteller om en kvittering kan forhaandsvises som bilde.
func IsImageReceipt(mime string) bool {
	return strings.HasPrefix(mime, "image/")
}

func nullIntVal(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: true}
}
