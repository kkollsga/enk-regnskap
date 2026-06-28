package mcp

import (
	"path/filepath"
	"strings"

	"github.com/kkollsga/enk-regnskap/internal/core"
	"github.com/kkollsga/enk-regnskap/internal/db"
)

// incomeInputFromExisting bygger en IncomeInput fra en lagret inntekt + dens
// utenlandske skattelinjer. Brukes til ikke-destruktiv (partial) oppdatering og
// til add_foreign_tax, slik at felt som ikke oppgis beholdes uendret.
func incomeInputFromExisting(inc db.Income, lines []db.IncomeForeignTax) core.IncomeInput {
	in := core.IncomeInput{
		Date: inc.Date, Description: inc.Description, Category: inc.Category,
		Client: inc.Client.String, CountryCode: inc.CountryCode, Currency: inc.Currency,
		AmountOrig: inc.AmountOrig, TaxYear: int(inc.TaxYear), Notes: inc.Notes.String,
		ForeignTaxPaid: int(inc.ForeignTaxPaid),
	}
	for _, l := range lines {
		in.ForeignTaxes = append(in.ForeignTaxes, core.ForeignTaxLine{
			Type: l.TaxType, AmountOrig: l.AmountOrig, Currency: l.Currency, Treatment: l.Treatment,
		})
	}
	return in
}

// expenseInputFromExisting bygger en ExpenseInput fra en lagret utgift.
func expenseInputFromExisting(e db.Expense) core.ExpenseInput {
	in := core.ExpenseInput{
		Date: e.Date, Description: e.Description, Category: e.Category,
		CountryCode: e.CountryCode, Currency: e.Currency, AmountOrig: e.AmountOrig,
		Notes: e.Notes.String, DeductiblePct: e.DeductiblePct, HasDeductiblePct: true,
		TaxYear: int(e.TaxYear),
	}
	if e.IncomeID.Valid {
		id := e.IncomeID.Int64
		in.IncomeID = &id
	}
	return in
}

// mimeFromName gjetter MIME-type fra filnavnets endelse (for attach_receipt).
func mimeFromName(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".heic":
		return "image/heic"
	case ".pdf":
		return "application/pdf"
	}
	return ""
}
