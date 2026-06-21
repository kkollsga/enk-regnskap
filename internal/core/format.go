package core

import (
	"strconv"
	"strings"
)

// FormatNOK formaterer et belop som "1 234,56 kr".
func FormatNOK(v float64) string {
	return FormatThousands(v, 2) + " kr"
}

// FormatThousands formaterer et tall med mellomrom som tusenskille og komma
// som desimalskille.
func FormatThousands(v float64, decimals int) string {
	neg := v < 0
	if neg {
		v = -v
	}
	s := strconv.FormatFloat(v, 'f', decimals, 64)
	intPart, decPart := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, decPart = s[:i], s[i+1:]
	}
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
