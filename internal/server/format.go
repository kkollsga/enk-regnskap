package server

import (
	"strconv"
	"strings"
)

// formatNOK formaterer et belop som norske kroner: "1 234,56 kr".
func formatNOK(v float64) string {
	return formatThousands(v, 2) + " kr"
}

// formatPct formaterer en prosentverdi: "75 %" eller "12,5 %".
func formatPct(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	s = strings.Replace(s, ".", ",", 1)
	return s + " %"
}

// formatThousands formaterer et tall med mellomrom som tusenskille og komma
// som desimalskille.
func formatThousands(v float64, decimals int) string {
	neg := v < 0
	if neg {
		v = -v
	}
	s := strconv.FormatFloat(v, 'f', decimals, 64)
	intPart, decPart := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, decPart = s[:i], s[i+1:]
	}
	// Sett inn mellomrom hvert tredje siffer fra hoyre.
	var b strings.Builder
	n := len(intPart)
	for i, c := range intPart {
		if i > 0 && (n-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(c)
	}
	out := b.String()
	if decPart != "" {
		out += "," + decPart
	}
	if neg {
		out = "-" + out
	}
	return out
}
