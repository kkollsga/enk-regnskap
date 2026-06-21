package currency_test

import (
	"context"
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/currency"
	"github.com/kkollsga/enk-regnskap/internal/db"
	"github.com/kkollsga/enk-regnskap/internal/testing/mocks"
)

func newService(t *testing.T) (*currency.Service, *mocks.NorgesBankMock) {
	t.Helper()
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	mock := mocks.NewNorgesBankMock()
	return currency.NewService(mock, db.New(conn)), mock
}

func TestNOKAlwaysOne(t *testing.T) {
	svc, mock := newService(t)
	r, err := svc.Rate(context.Background(), "NOK", "2025-01-15")
	if err != nil {
		t.Fatalf("Rate NOK: %v", err)
	}
	if r.RateNOK != 1 {
		t.Errorf("NOK-kurs = %v, forventet 1", r.RateNOK)
	}
	if mock.Calls != 0 {
		t.Errorf("NOK skal ikke kalle provider, Calls = %d", mock.Calls)
	}
}

func TestMockReturnsExpectedRate(t *testing.T) {
	svc, mock := newService(t)
	mock.AddRate("BRL", "2025-01-15", 1.90)
	r, err := svc.Rate(context.Background(), "BRL", "2025-01-15")
	if err != nil {
		t.Fatalf("Rate BRL: %v", err)
	}
	if r.RateNOK != 1.90 {
		t.Errorf("BRL-kurs = %v, forventet 1.90", r.RateNOK)
	}
	if r.Date != "2025-01-15" {
		t.Errorf("kursdato = %q, forventet 2025-01-15", r.Date)
	}
}

func TestCacheFetchesOnce(t *testing.T) {
	svc, mock := newService(t)
	mock.AddRate("BRL", "2025-01-15", 1.90)
	ctx := context.Background()

	if _, err := svc.Rate(ctx, "BRL", "2025-01-15"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Rate(ctx, "BRL", "2025-01-15"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Rate(ctx, "BRL", "2025-01-15"); err != nil {
		t.Fatal(err)
	}
	if mock.Calls != 1 {
		t.Errorf("provider kalt %d ganger, forventet 1 (resten fra cache)", mock.Calls)
	}
}

func TestWeekendFallsBackToPreviousBankDay(t *testing.T) {
	svc, mock := newService(t)
	// Bare fredag 2025-01-17 har kurs; 18.-19. er helg.
	mock.AddRate("BRL", "2025-01-17", 1.95)
	ctx := context.Background()

	// Lordag 2025-01-18 -> skal bruke fredagens kurs
	r, err := svc.Rate(ctx, "BRL", "2025-01-18")
	if err != nil {
		t.Fatalf("Rate lordag: %v", err)
	}
	if r.RateNOK != 1.95 {
		t.Errorf("lordagskurs = %v, forventet 1.95 (fredag)", r.RateNOK)
	}
	if r.Date != "2025-01-17" {
		t.Errorf("kursdato = %q, forventet 2025-01-17 (naermeste bankdag)", r.Date)
	}

	// Sondag igjen -> skal treffe cache, ikke kalle provider paa nytt
	if _, err := svc.Rate(ctx, "BRL", "2025-01-19"); err != nil {
		t.Fatalf("Rate sondag: %v", err)
	}
	if mock.Calls != 1 {
		t.Errorf("provider kalt %d ganger, forventet 1 (helg bruker cachet bankdag)", mock.Calls)
	}
}

func TestUnknownCurrencyErrors(t *testing.T) {
	svc, _ := newService(t)
	if _, err := svc.Rate(context.Background(), "XYZ", "2025-01-15"); err == nil {
		t.Error("ukjent valuta skulle gi feil")
	}
}
