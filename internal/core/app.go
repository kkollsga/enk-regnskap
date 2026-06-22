// Package core er forretningslogikklaget i ENK Regnskap. Bade HTTP-handlerne
// og MCP-serveren er tynne adaptere over dette laget, slik at en AI-agent og
// en menneskelig bruker far nøyaktig samme oppforsel - inkludert revisjonsspor
// (change_log) og live oppdatering (SSE Hub).
package core

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kkollsga/enk-regnskap/internal/currency"
	"github.com/kkollsga/enk-regnskap/internal/db"
)

// App samler delte avhengigheter for hele applikasjonen.
type App struct {
	DB       *sql.DB
	Q        *db.Queries
	Currency *currency.Service
	Events   *Hub
	DataDir  string
}

// New åpner databasen i dataDir/data.db, oppretter nodvendige mapper og
// returnerer en klar App. Hvis dataDir er tom brukes en flyktig in-memory
// database (for test).
func New(dataDir string, provider currency.ExchangeRateProvider) (*App, error) {
	dbPath := ":memory:"
	if dataDir != "" {
		if err := os.MkdirAll(filepath.Join(dataDir, "receipts"), 0o755); err != nil {
			return nil, fmt.Errorf("opprett receipts-mappe: %w", err)
		}
		dbPath = filepath.Join(dataDir, "data.db")
	}
	conn, err := db.Open(dbPath)
	if err != nil {
		return nil, err
	}
	q := db.New(conn)
	if provider == nil {
		provider = currency.NewNorgesBank()
	}
	return &App{
		DB:       conn,
		Q:        q,
		Currency: currency.NewService(provider, q),
		Events:   NewHub(),
		DataDir:  dataDir,
	}, nil
}

// Close lukker databasen.
func (a *App) Close() error {
	if a.DB != nil {
		return a.DB.Close()
	}
	return nil
}

// --- Konfigurasjon ---

// Config-nøkler som brukes av appen.
const (
	ConfigLanguage     = "language"
	ConfigBusinessName = "business_name"
	ConfigOrgNr        = "org_nr"
	ConfigActiveYear   = "active_tax_year"
	ConfigOnboarded    = "onboarded"
)

// GetConfig henter en konfigurasjonsverdi, eller fallback hvis den mangler.
func (a *App) GetConfig(ctx context.Context, key, fallback string) string {
	v, err := a.Q.GetConfig(ctx, key)
	if err != nil {
		return fallback
	}
	return v
}

// SetConfig lagrer en konfigurasjonsverdi.
func (a *App) SetConfig(ctx context.Context, key, value string) error {
	return a.Q.SetConfig(ctx, db.SetConfigParams{Key: key, Value: value})
}

// ActiveYear returnerer aktivt inntektsår fra config, eller inneverende år.
func (a *App) ActiveYear(ctx context.Context) int {
	v := a.GetConfig(ctx, ConfigActiveYear, "")
	if v != "" {
		var y int
		if _, err := fmt.Sscanf(v, "%d", &y); err == nil && y > 2000 {
			return y
		}
	}
	return time.Now().Year()
}

// Language returnerer valgt språk (default norsk bokmal).
func (a *App) Language(ctx context.Context) string {
	return a.GetConfig(ctx, ConfigLanguage, "nb")
}

// IsOnboarded forteller om førstegangsoppsettet er fullfort.
func (a *App) IsOnboarded(ctx context.Context) bool {
	return a.GetConfig(ctx, ConfigOnboarded, "") == "1"
}

// OnboardInput er dataene fra velkomstskjermen.
type OnboardInput struct {
	BusinessName string
	OrgNr        string
	Language     string
}

// CompleteOnboarding lagrer virksomhetsinfo og markerer oppsettet som fullfort.
func (a *App) CompleteOnboarding(ctx context.Context, in OnboardInput) error {
	if in.Language == "" {
		in.Language = "nb"
	}
	if err := a.SetConfig(ctx, ConfigBusinessName, in.BusinessName); err != nil {
		return err
	}
	if err := a.SetConfig(ctx, ConfigOrgNr, in.OrgNr); err != nil {
		return err
	}
	if err := a.SetConfig(ctx, ConfigLanguage, in.Language); err != nil {
		return err
	}
	if a.GetConfig(ctx, ConfigActiveYear, "") == "" {
		if err := a.SetConfig(ctx, ConfigActiveYear, fmt.Sprintf("%d", a.ActiveYear(ctx))); err != nil {
			return err
		}
	}
	return a.SetConfig(ctx, ConfigOnboarded, "1")
}
