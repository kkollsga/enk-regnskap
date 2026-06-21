// Package export genererer Excel- og CSV-eksport for ENK Regnskap.
package export

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/xuri/excelize/v2"

	"github.com/kkollsga/enk-regnskap/internal/core"
)

// TransactionsXLSX skriver transaksjonsloggen (inntekter og utgifter) som
// en gyldig .xlsx til w.
func TransactionsXLSX(w io.Writer, rep core.Report, catName func(string) string) error {
	f := excelize.NewFile()
	defer f.Close()

	// Ark: Inntekter
	inc := "Inntekter"
	f.SetSheetName("Sheet1", inc)
	incHeaders := []string{"Dato", "Beskrivelse", "Klient", "Kategori", "Land", "Valuta", "Belop", "Kurs", "Kursdato", "Belop NOK", "Utenlandsk skatt (NOK)"}
	writeRow(f, inc, 1, toAny(incHeaders)...)
	r := 2
	for _, in := range rep.Income {
		writeRow(f, inc, r,
			in.Date, in.Description, nullStr(in.Client), catName(in.Category), in.CountryCode,
			in.Currency, in.AmountOrig, nullFloat(in.ExchangeRate), nullStr(in.RateDate),
			in.AmountNok, nullFloat(in.ForeignTaxNok))
		r++
	}
	writeRow(f, inc, r, "", "", "", "", "", "", "", "", "", rep.TotalIncome, "")

	// Ark: Utgifter
	exp := "Utgifter"
	f.NewSheet(exp)
	expHeaders := []string{"Dato", "Beskrivelse", "Kategori", "Belop NOK", "Fradrag %", "Fradragsberettiget NOK"}
	writeRow(f, exp, 1, toAny(expHeaders)...)
	r = 2
	for _, ex := range rep.Expenses {
		writeRow(f, exp, r, ex.Date, ex.Description, catName(ex.Category), ex.AmountNok, ex.DeductiblePct, ex.DeductibleNok)
		r++
	}
	writeRow(f, exp, r, "", "", "", rep.TotalExpenses, "", rep.TotalDeductible)

	return f.Write(w)
}

// Naeringsspesifikasjon skriver naeringsspesifikasjonen som .xlsx.
func Naeringsspesifikasjon(w io.Writer, rep core.Report, catName func(string) string) error {
	f := excelize.NewFile()
	defer f.Close()
	sh := "Naeringsspesifikasjon"
	f.SetSheetName("Sheet1", sh)

	writeRow(f, sh, 1, fmt.Sprintf("Naeringsspesifikasjon %d", rep.Year))
	row := 3
	writeRow(f, sh, row, "Driftsinntekter")
	row++
	for _, c := range rep.IncomeByCategory {
		writeRow(f, sh, row, catName(c.Category), c.Total)
		row++
	}
	writeRow(f, sh, row, "Sum driftsinntekter", rep.TotalIncome)
	row += 2
	writeRow(f, sh, row, "Driftskostnader", "Kostnad", "Fradragsberettiget")
	row++
	for _, c := range rep.ExpenseByCategory {
		writeRow(f, sh, row, catName(c.Category), c.Total, c.Deductible)
		row++
	}
	writeRow(f, sh, row, "Sum fradrag", rep.TotalExpenses, rep.TotalDeductible)
	row += 2
	writeRow(f, sh, row, "Naeringsresultat", rep.Result)
	return f.Write(w)
}

// TransactionsCSV skriver transaksjonsloggen som CSV (semikolon-separert,
// vanlig i norsk Excel).
func TransactionsCSV(w io.Writer, rep core.Report, catName func(string) string) error {
	cw := csv.NewWriter(w)
	cw.Comma = ';'
	defer cw.Flush()

	header := []string{"Type", "Dato", "Beskrivelse", "Kategori", "Land", "Valuta", "Belop", "Belop NOK", "Fradragsberettiget NOK", "Utenlandsk skatt NOK"}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, in := range rep.Income {
		rec := []string{"inntekt", in.Date, in.Description, catName(in.Category), in.CountryCode,
			in.Currency, f2(in.AmountOrig), f2(in.AmountNok), "", f2nullable(in.ForeignTaxNok)}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	for _, ex := range rep.Expenses {
		rec := []string{"utgift", ex.Date, ex.Description, catName(ex.Category), "NO",
			"NOK", f2(ex.AmountNok), f2(ex.AmountNok), f2(ex.DeductibleNok), ""}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	return cw.Error()
}

// --- hjelpere ---

func writeRow(f *excelize.File, sheet string, row int, cols ...any) {
	for i, c := range cols {
		cell, _ := excelize.CoordinatesToCellName(i+1, row)
		_ = f.SetCellValue(sheet, cell, c)
	}
}

func toAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// nullFloat returnerer float-verdien, eller "" hvis NULL (for Excel-celle).
func nullFloat(nf sql.NullFloat64) any {
	if nf.Valid {
		return nf.Float64
	}
	return ""
}

func f2(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

func f2nullable(nf sql.NullFloat64) string {
	if nf.Valid {
		return f2(nf.Float64)
	}
	return ""
}
