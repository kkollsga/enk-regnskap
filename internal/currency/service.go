package currency

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/kkollsga/enk-regnskap/internal/db"
)

// maxCacheStaleDays er hvor mange dager en cachet kurs maks kan ligge for
// onsket dato og fortsatt brukes direkte (dekker helg + helligdager). Er
// gapet storre, hentes ny kurs fra provideren for aa unngaa feil "naermeste".
const maxCacheStaleDays = 5

// Service henter kurser via en provider og cacher dem i SQLite.
type Service struct {
	provider ExchangeRateProvider
	q        *db.Queries
}

// NewService lager en valutatjeneste.
func NewService(provider ExchangeRateProvider, q *db.Queries) *Service {
	return &Service{provider: provider, q: q}
}

// Rate returnerer kursen for currency paa date (NOK per 1 enhet). For NOK
// returneres alltid 1. Kursen caches slik at gjentatte oppslag ikke trenger
// nett.
func (s *Service) Rate(ctx context.Context, currency, date string) (Rate, error) {
	currency = NormalizeCurrency(currency)
	if currency == "" || currency == "NOK" {
		return Rate{Currency: "NOK", Date: date, RateNOK: 1, Source: "fixed"}, nil
	}

	// 1. Cache: naermeste foregaaende kurs, hvis den er fersk nok.
	cached, err := s.q.GetNearestExchangeRate(ctx, db.GetNearestExchangeRateParams{
		Currency: currency,
		Date:     date,
	})
	if err == nil {
		if gap, ok := daysBetween(date, cached.Date); ok && gap <= maxCacheStaleDays {
			return Rate{
				Currency: cached.Currency,
				Date:     cached.Date,
				RateNOK:  cached.RateNok,
				Source:   "cache:" + cached.Source,
			}, nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Rate{}, fmt.Errorf("les kurscache: %w", err)
	}

	// 2. Hent fra provider.
	rate, ferr := s.provider.FetchRate(ctx, currency, date)
	if ferr != nil {
		// 3. Offline-fallback: bruk cachet kurs selv om den er eldre.
		if err == nil {
			return Rate{
				Currency: cached.Currency,
				Date:     cached.Date,
				RateNOK:  cached.RateNok,
				Source:   "cache-stale:" + cached.Source,
			}, nil
		}
		return Rate{}, fmt.Errorf("hent kurs for %s %s: %w", currency, date, ferr)
	}

	// 4. Cache resultatet under den faktiske bankdagen.
	if err := s.q.UpsertExchangeRate(ctx, db.UpsertExchangeRateParams{
		Currency: rate.Currency,
		Date:     rate.Date,
		RateNok:  rate.RateNOK,
		Source:   rate.Source,
	}); err != nil {
		return Rate{}, fmt.Errorf("cache kurs: %w", err)
	}
	return rate, nil
}
