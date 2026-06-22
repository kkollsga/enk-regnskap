// Package mocks inneholder mock-versjoner av eksterne tjenester slik at
// tester kan kjore uten internettilgang.
package mocks

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kkollsga/enk-regnskap/internal/currency"
)

const dateLayout = "2006-01-02"

// NorgesBankMock er en deterministisk ExchangeRateProvider for test.
// Rates er kurser per bankdag: currency -> dato (ISO) -> NOK per enhet.
// Datoer som ikke finnes regnes som helg/helligdag og faller tilbake til
// nærmeste foregående bankdag som finnes.
type NorgesBankMock struct {
	Rates map[string]map[string]float64
	Calls int // antall FetchRate-kall (for cache-verifisering)
}

// NewNorgesBankMock lager en tom mock.
func NewNorgesBankMock() *NorgesBankMock {
	return &NorgesBankMock{Rates: map[string]map[string]float64{}}
}

// AddRate registrerer en kurs for en gitt valuta og bankdag.
func (m *NorgesBankMock) AddRate(currency, date string, rate float64) {
	currency = strings.ToUpper(currency)
	if m.Rates[currency] == nil {
		m.Rates[currency] = map[string]float64{}
	}
	m.Rates[currency][date] = rate
}

// FetchRate implementerer currency.ExchangeRateProvider.
func (m *NorgesBankMock) FetchRate(ctx context.Context, cur, date string) (currency.Rate, error) {
	m.Calls++
	cur = strings.ToUpper(strings.TrimSpace(cur))
	if cur == "NOK" {
		return currency.Rate{Currency: "NOK", Date: date, RateNOK: 1, Source: "fixed"}, nil
	}
	rates, ok := m.Rates[cur]
	if !ok {
		return currency.Rate{}, fmt.Errorf("mock: ingen kurser for %s", cur)
	}
	t, err := time.Parse(dateLayout, date)
	if err != nil {
		return currency.Rate{}, fmt.Errorf("mock: ugyldig dato %q", date)
	}
	// Gå bakover inntil 10 dager for å finne nærmeste foregående bankdag.
	for i := 0; i < 11; i++ {
		d := t.AddDate(0, 0, -i).Format(dateLayout)
		if r, ok := rates[d]; ok {
			return currency.Rate{Currency: cur, Date: d, RateNOK: r, Source: "norges-bank-mock"}, nil
		}
	}
	return currency.Rate{}, fmt.Errorf("mock: ingen kurs for %s frem til %s", cur, date)
}

// SeedBRL fyller inn realistiske BRL/NOK-kurser for alle ukedager i et
// intervall (helger utelates med vilje for å teste fallback). Brukes av
// fixtures. base er kurs ved start; den varierer svakt.
func (m *NorgesBankMock) SeedBRL(from, to string, base float64) error {
	start, err := time.Parse(dateLayout, from)
	if err != nil {
		return err
	}
	end, err := time.Parse(dateLayout, to)
	if err != nil {
		return err
	}
	i := 0
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			continue
		}
		// Liten deterministisk variasjon rundt base.
		rate := base + float64(i%7)*0.01
		m.AddRate("BRL", d.Format(dateLayout), rate)
		i++
	}
	return nil
}
