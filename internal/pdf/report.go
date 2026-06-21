// Package pdf genererer PDF-rapporter (aarsrapport og naeringsspesifikasjon)
// for ENK Regnskap via go-pdf/fpdf.
package pdf

import (
	"fmt"
	"io"

	"github.com/go-pdf/fpdf"

	"github.com/kkollsga/enk-regnskap/internal/core"
)

// doc er en liten innpakning rundt fpdf med norsk tegnsett-oversetter.
type doc struct {
	pdf *fpdf.Fpdf
	tr  func(string) string
}

func newDoc() *doc {
	p := fpdf.New("P", "mm", "A4", "")
	p.SetMargins(18, 18, 18)
	p.SetAutoPageBreak(true, 18)
	p.AddPage()
	return &doc{pdf: p, tr: p.UnicodeTranslatorFromDescriptor("")}
}

func (d *doc) t(s string) string { return d.tr(s) }

func (d *doc) title(s string) {
	d.pdf.SetFont("Helvetica", "B", 18)
	d.pdf.CellFormat(0, 10, d.t(s), "", 1, "L", false, 0, "")
	d.pdf.Ln(2)
}

func (d *doc) heading(s string) {
	d.pdf.Ln(3)
	d.pdf.SetFont("Helvetica", "B", 13)
	d.pdf.CellFormat(0, 8, d.t(s), "", 1, "L", false, 0, "")
}

func (d *doc) small(s string) {
	d.pdf.SetFont("Helvetica", "", 9)
	d.pdf.SetTextColor(110, 110, 110)
	d.pdf.MultiCell(0, 5, d.t(s), "", "L", false)
	d.pdf.SetTextColor(0, 0, 0)
}

// row skriver en to-kolonne rad (etikett + hoyrejustert belop).
func (d *doc) row(label, value string, bold bool) {
	style := ""
	if bold {
		style = "B"
	}
	d.pdf.SetFont("Helvetica", style, 11)
	d.pdf.CellFormat(120, 7, d.t(label), "", 0, "L", false, 0, "")
	d.pdf.CellFormat(0, 7, d.t(value), "", 1, "R", false, 0, "")
}

// tableHeader skriver en kolonneoverskrift med tre kolonner.
func (d *doc) row3(c1, c2, c3 string, bold bool) {
	style := ""
	if bold {
		style = "B"
	}
	d.pdf.SetFont("Helvetica", style, 10)
	d.pdf.CellFormat(90, 6, d.t(c1), "", 0, "L", false, 0, "")
	d.pdf.CellFormat(45, 6, d.t(c2), "", 0, "R", false, 0, "")
	d.pdf.CellFormat(0, 6, d.t(c3), "", 1, "R", false, 0, "")
}

func (d *doc) output(w io.Writer) error { return d.pdf.Output(w) }

func nok(v float64) string { return core.FormatNOK(v) }

// AnnualReport skriver en fullstendig aarsrapport som PDF.
func AnnualReport(w io.Writer, rep core.Report, catName func(string) string) error {
	d := newDoc()
	name := rep.BusinessName
	if name == "" {
		name = "Enkeltpersonforetak"
	}
	d.title(fmt.Sprintf("Arsrapport %d", rep.Year))
	d.pdf.SetFont("Helvetica", "", 11)
	d.pdf.CellFormat(0, 6, d.t(name), "", 1, "L", false, 0, "")
	if rep.OrgNr != "" {
		d.pdf.CellFormat(0, 6, d.t("Org.nr. "+rep.OrgNr), "", 1, "L", false, 0, "")
	}

	d.heading("Driftsinntekter")
	d.row3("Kategori", "", "Belop (NOK)", true)
	for _, c := range rep.IncomeByCategory {
		d.row3(catName(c.Category), "", nok(c.Total), false)
	}
	d.row("Sum driftsinntekter", nok(rep.TotalIncome), true)

	d.heading("Driftskostnader")
	d.row3("Kategori", "Kostnad", "Fradrag (NOK)", true)
	for _, c := range rep.ExpenseByCategory {
		d.row3(catName(c.Category), nok(c.Total), nok(c.Deductible), false)
	}
	d.row("Sum fradragsberettiget", nok(rep.TotalDeductible), true)

	d.heading("Resultat")
	d.row("Naeringsresultat (inntekt - fradrag)", nok(rep.Result), true)
	if rep.Tax != nil {
		d.row("Estimert alminnelig inntektsskatt (22 %)", nok(rep.Tax.AlminneligInntektsskatt), false)
		d.row("Estimert trygdeavgift", nok(rep.Tax.Trygdeavgift), false)
		d.row("Estimert trinnskatt", nok(rep.Tax.Trinnskatt), false)
		d.row("Estimert sum skatt", nok(rep.Tax.SumSkatt), true)
	}

	if len(rep.ForeignCredits) > 0 {
		d.heading("Utenlandsinntekter og kreditfradrag")
		for _, c := range rep.ForeignCredits {
			basis := "intern norsk rett (sktl. § 16-20 flg.)"
			if c.LegalBasis.String == "treaty" {
				basis = "skatteavtale (kreditmetoden)"
			}
			d.small(fmt.Sprintf("%s: inntekt %s, betalt utenlandsk skatt %s. Rettsgrunnlag: %s. Kreditfradrag fylles inn i RF-1147.",
				c.CountryName, nok(c.IncomeNok), nok(c.ForeignTaxNok), basis))
		}
		d.small("Maksimalt kreditfradrag begrenses av norsk skatt paa samme inntekt (sktl. § 16-21). Ubenyttet kredit kan fremfoeres i inntil 5 aar (sktl. § 16-22).")
	}

	if len(rep.Income) > 0 || len(rep.Expenses) > 0 {
		d.heading("Vedlagte kvitteringer (referanseliste)")
		count := 0
		for _, in := range rep.Income {
			if in.ReceiptID.Valid {
				count++
				d.small(fmt.Sprintf("Inntekt %s - %s: kvittering #%d", in.Date, in.Description, in.ReceiptID.Int64))
			}
		}
		for _, ex := range rep.Expenses {
			if ex.ReceiptID.Valid {
				count++
				d.small(fmt.Sprintf("Utgift %s - %s: kvittering #%d", ex.Date, ex.Description, ex.ReceiptID.Int64))
			}
		}
		if count == 0 {
			d.small("Ingen kvitteringer tilknyttet transaksjoner.")
		}
	}

	d.pdf.Ln(6)
	d.small("Generert av ENK Regnskap. Dette er et stottedokument, ikke en bindende skattefastsettelse.")
	return d.output(w)
}

// TaxSummary skriver naeringsspesifikasjonen (Skatteetaten-poster) som PDF.
func TaxSummary(w io.Writer, rep core.Report, catName func(string) string) error {
	d := newDoc()
	d.title(fmt.Sprintf("Naeringsspesifikasjon %d", rep.Year))
	d.small("Strukturert etter Skatteetatens poster for ENK. Klar til aa taste inn i skattemeldingen paa skatteetaten.no.")

	d.heading("Driftsinntekter")
	d.row3("Post / kategori", "", "Belop (NOK)", true)
	for _, c := range rep.IncomeByCategory {
		d.row3(catName(c.Category), "", nok(c.Total), false)
	}
	d.row("Sum driftsinntekter", nok(rep.TotalIncome), true)

	d.heading("Driftskostnader (fradrag)")
	d.row3("Post / kategori", "Kostnad", "Fradrag (NOK)", true)
	for _, c := range rep.ExpenseByCategory {
		d.row3(catName(c.Category), nok(c.Total), nok(c.Deductible), false)
	}
	d.row("Sum fradrag", nok(rep.TotalDeductible), true)

	d.heading("Naeringsresultat")
	d.row("Skattemessig naeringsresultat", nok(rep.Result), true)

	if len(rep.ForeignCredits) > 0 {
		d.heading("Kreditfradrag for utenlandsk skatt (RF-1147)")
		for _, c := range rep.ForeignCredits {
			d.row(fmt.Sprintf("%s - betalt utenlandsk skatt", c.CountryName), nok(c.ForeignTaxNok), false)
		}
	}
	return d.output(w)
}
