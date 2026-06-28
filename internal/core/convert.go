package core

import (
	"context"
	"database/sql"

	"github.com/kkollsga/enk-regnskap/internal/tax"
)

// ConvertedAmount er resultatet av en valutakonvertering til NOK. Brukes både av
// inntekt og utgift, så konverteringslogikken finnes ett sted.
type ConvertedAmount struct {
	AmountNOK    float64         // beløp i NOK
	Rate         float64         // kurs (1.0 for NOK)
	RateDate     string          // datoen kursen gjelder for
	ExchangeRate sql.NullFloat64 // satt (gyldig) kun når valutaen ikke er NOK
	RateDateNull sql.NullString  // satt (gyldig) kun når valutaen ikke er NOK
}

// convertToNOK henter Norges Bank-kursen for valuta/dato og konverterer amount
// til NOK. For NOK (eller tom valuta) returneres beløpet uendret med kurs 1.0.
// field er feltnavnet en eventuell valideringsfeil knyttes til.
func (a *App) convertToNOK(ctx context.Context, currency, date string, amount float64, field string) (ConvertedAmount, error) {
	c := ConvertedAmount{AmountNOK: amount, Rate: 1.0, RateDate: date}
	if currency == "" || currency == "NOK" {
		return c, nil
	}
	r, err := a.Currency.Rate(ctx, currency, date)
	if err != nil {
		ve := newValidation()
		ve.add(field, "kunne ikke hente valutakurs: "+err.Error())
		return c, ve
	}
	c.Rate = r.RateNOK
	c.RateDate = r.Date
	c.AmountNOK = tax.Round2(amount * r.RateNOK)
	c.ExchangeRate = sql.NullFloat64{Float64: r.RateNOK, Valid: true}
	c.RateDateNull = sql.NullString{String: r.Date, Valid: true}
	return c, nil
}
