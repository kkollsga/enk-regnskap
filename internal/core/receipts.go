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

// allowedReceiptTypes er tillatte MIME-typer for vedlegg.
var allowedReceiptTypes = map[string]string{
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
	"image/gif":       ".gif",
	"image/webp":      ".webp",
	"image/heic":      ".heic",
	"application/pdf": ".pdf",
}

// ReceiptInput er et opplastet vedlegg knyttet til en inntekt/utgift.
type ReceiptInput struct {
	OriginalName string
	MimeType     string
	Data         []byte
	Title        string
	Description  string
	ParentKind   string // "income" | "expense"
	ParentID     int64
	TaxYear      int
}

// SaveReceipt lagrer et vedlegg under data/receipts/AAAA/<uuid>.<ext> og
// oppretter en rad i receipts, koblet til en inntekt/utgift.
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
			return nil, fmt.Errorf("opprett vedleggsmappe: %w", err)
		}
		if err := os.WriteFile(full, in.Data, 0o644); err != nil {
			return nil, fmt.Errorf("skriv vedlegg: %w", err)
		}
	}

	created, err := a.Q.CreateReceipt(ctx, db.CreateReceiptParams{
		Filename:     filepath.ToSlash(rel),
		OriginalName: in.OriginalName,
		MimeType:     in.MimeType,
		Title:        nullString(in.Title),
		Description:  nullString(in.Description),
		ParentKind:   nullString(in.ParentKind),
		ParentID:     nullInt(optInt(in.ParentID)),
		TaxYear:      nullIntVal(int64(year)),
	})
	if err != nil {
		return nil, fmt.Errorf("lagre vedleggspost: %w", err)
	}
	after, _ := a.snapshotRow(ctx, "receipts", created.ID)
	desc := fmt.Sprintf("La til vedlegg: %s", receiptLabel(created))
	if err := a.logChange(ctx, actor, "insert", "receipts", created.ID, nil, after, year, desc); err != nil {
		return nil, err
	}
	return &created, nil
}

// UpdateReceiptMeta oppdaterer tittel og beskrivelse på et vedlegg.
func (a *App) UpdateReceiptMeta(ctx context.Context, actor string, id int64, title, description string) error {
	before, err := a.snapshotRow(ctx, "receipts", id)
	if err != nil || before == nil {
		return fmt.Errorf("vedlegg %d finnes ikke", id)
	}
	if err := a.Q.UpdateReceiptMeta(ctx, db.UpdateReceiptMetaParams{
		Title: nullString(title), Description: nullString(description), ID: id,
	}); err != nil {
		return err
	}
	after, _ := a.snapshotRow(ctx, "receipts", id)
	return a.logChange(ctx, actor, "update", "receipts", id, before, after, toInt(before["tax_year"]),
		fmt.Sprintf("Oppdaterte vedlegg #%d", id))
}

// DeleteReceipt sletter et vedlegg (post + fil) med revisjonsspor.
func (a *App) DeleteReceipt(ctx context.Context, actor string, id int64) error {
	rec, err := a.Q.GetReceipt(ctx, id)
	if err != nil {
		return fmt.Errorf("vedlegg %d finnes ikke", id)
	}
	before, _ := a.snapshotRow(ctx, "receipts", id)
	if err := a.Q.DeleteReceipt(ctx, id); err != nil {
		return err
	}
	if a.DataDir != "" {
		_ = os.Remove(a.ReceiptPath(rec))
	}
	return a.logChange(ctx, actor, "delete", "receipts", id, before, nil, int(rec.TaxYear.Int64),
		fmt.Sprintf("Slettet vedlegg #%d", id))
}

// ReceiptPath returnerer den absolutte stien til et lagret vedlegg.
func (a *App) ReceiptPath(rec db.Receipt) string {
	return filepath.Join(a.DataDir, "receipts", filepath.FromSlash(rec.Filename))
}

// GetReceipt henter en vedleggspost.
func (a *App) GetReceipt(ctx context.Context, id int64) (db.Receipt, error) {
	return a.Q.GetReceipt(ctx, id)
}

// ListReceipts henter alle vedlegg.
func (a *App) ListReceipts(ctx context.Context) ([]db.Receipt, error) {
	return a.Q.ListReceipts(ctx)
}

// ReceiptsFor henter vedleggene for en inntekt/utgift.
func (a *App) ReceiptsFor(ctx context.Context, kind string, id int64) ([]db.Receipt, error) {
	return a.Q.ListReceiptsByParent(ctx, db.ListReceiptsByParentParams{
		ParentKind: nullString(kind), ParentID: nullInt(optInt(id)),
	})
}

// receiptLabel gir en lesbar etikett (tittel eller originalnavn).
func receiptLabel(r db.Receipt) string {
	if r.Title.Valid && r.Title.String != "" {
		return r.Title.String
	}
	return r.OriginalName
}

// ReceiptLabel er den eksporterte varianten for visning.
func ReceiptLabel(r db.Receipt) string { return receiptLabel(r) }

// IsImageReceipt forteller om et vedlegg kan forhåndsvises som bilde.
func IsImageReceipt(mime string) bool {
	return strings.HasPrefix(mime, "image/")
}

// ReceiptTypeAllowed sjekker om en MIME-type er tillatt for vedlegg.
func ReceiptTypeAllowed(mime string) bool {
	_, ok := allowedReceiptTypes[mime]
	return ok
}

func nullIntVal(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: true}
}

func optInt(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}
