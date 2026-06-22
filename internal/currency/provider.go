// Package currency henter og cacher valutakurser mot Norges Bank, slik at
// historiske transaksjoner aldri trenger nett. Provider-interfacet gjør at
// ekte tjeneste og mock er utbyttbare (dependency injection).
package currency

import (
	"context"
	"strings"
	"time"
)

// Rate er en valutakurs: 1 enhet Currency = RateNOK norske kroner.
// Date er den faktiske bankdagen kursen stammer fra (kan være tidligere enn
// datoen det ble spurt om, ved helg/helligdag).
type Rate struct {
	Currency string  `json:"currency"`
	Date     string  `json:"date"`     // faktisk kursdato (ISO 8601)
	RateNOK  float64 `json:"rate_nok"` // NOK per 1 enhet valuta
	Source   string  `json:"source"`
}

// ExchangeRateProvider henter en kurs for en valuta på (eller nærmest for)
// en gitt dato. Implementeres av Norges Bank-klienten og av mocken.
type ExchangeRateProvider interface {
	// FetchRate returnerer kursen for currency som gjelder for date. Hvis date
	// faller på helg/helligdag returneres nærmeste foregående bankdag.
	FetchRate(ctx context.Context, currency, date string) (Rate, error)
}

const dateLayout = "2006-01-02"

// NormalizeCurrency gjør valutakoden stor og trimmet.
func NormalizeCurrency(c string) string {
	return strings.ToUpper(strings.TrimSpace(c))
}

// parseDate tolker en ISO-dato.
func parseDate(s string) (time.Time, error) {
	return time.Parse(dateLayout, strings.TrimSpace(s))
}

// daysBetween returnerer absolutt antall dager mellom to ISO-datoer.
func daysBetween(a, b string) (int, bool) {
	ta, err := parseDate(a)
	if err != nil {
		return 0, false
	}
	tb, err := parseDate(b)
	if err != nil {
		return 0, false
	}
	d := ta.Sub(tb).Hours() / 24
	if d < 0 {
		d = -d
	}
	return int(d + 0.5), true
}
