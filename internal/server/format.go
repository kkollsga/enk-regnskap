package server

import (
	"strconv"
	"strings"

	"github.com/kkollsga/enk-regnskap/internal/core"
)

// formatNOK formaterer et beløp som norske kroner: "1 234,56 kr".
func formatNOK(v float64) string {
	return core.FormatNOK(v)
}

// formatOrgNr grupperer et 9-sifret organisasjonsnummer: "999 888 777".
func formatOrgNr(s string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
	if len(digits) == 9 {
		return digits[0:3] + " " + digits[3:6] + " " + digits[6:9]
	}
	return s
}

// formatPct formaterer en prosentverdi: "75 %" eller "12,5 %".
func formatPct(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	s = strings.Replace(s, ".", ",", 1)
	return s + " %"
}

// clip gjør en tekst om til én linje (uten linjeskift) og kutter den til maks
// 200 tegn, med ellipsis hvis den var lengre. Brukt for vedleggsbeskrivelser.
func clip(s string) string {
	s = strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
	r := []rune(s)
	if len(r) > 200 {
		return strings.TrimSpace(string(r[:200])) + "…"
	}
	return s
}
