package server

import (
	"strconv"
	"strings"

	"github.com/kkollsga/enk-regnskap/internal/core"
)

// formatNOK formaterer et belop som norske kroner: "1 234,56 kr".
func formatNOK(v float64) string {
	return core.FormatNOK(v)
}

// formatPct formaterer en prosentverdi: "75 %" eller "12,5 %".
func formatPct(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	s = strings.Replace(s, ".", ",", 1)
	return s + " %"
}
