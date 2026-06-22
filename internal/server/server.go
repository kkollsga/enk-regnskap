package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/kkollsga/enk-regnskap/internal/core"
	"github.com/kkollsga/enk-regnskap/internal/mcp"
	"github.com/kkollsga/enk-regnskap/internal/tax"
	"github.com/kkollsga/enk-regnskap/web"
)

// Server binder core.App til HTTP-laget.
type Server struct {
	app      *core.App
	renderer *Renderer
	mux      http.Handler
}

// New lager en Server med parsede maler og ferdig router.
func New(app *core.App) (*Server, error) {
	r, err := NewRenderer()
	if err != nil {
		return nil, err
	}
	s := &Server{app: app, renderer: r}
	s.mux = s.routes()
	return s, nil
}

// ServeHTTP gjor Server til en http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(s.onboardingGate)

	// Statiske filer fra embeddet web/static.
	staticFS, _ := fs.Sub(web.Static, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	r.Get("/welcome", s.handleWelcome)
	r.Post("/onboard", s.handleOnboard)

	r.Get("/health", s.handleHealth)
	r.Get("/events", s.handleEvents)
	r.Get("/api/exchange-rate", s.handleExchangeRate)

	// MCP-endepunkt (in-process => agentens endringer oppdaterer UI-et live).
	r.Post("/mcp", mcp.New(s.app).HTTPHandler())

	r.Get("/", s.handleDashboard)
	r.Get("/set-year", s.handleSetYear)
	r.Get("/set-lang", s.handleSetLang)

	r.Get("/income", s.handleIncomeList)
	r.Get("/income/new", s.handleIncomeNew)
	r.Post("/income", s.handleIncomeCreate)

	r.Get("/expenses", s.handleExpenseList)
	r.Get("/expenses/new", s.handleExpenseNew)
	r.Post("/expenses", s.handleExpenseCreate)

	r.Get("/receipts", s.handleReceiptsList)
	r.Post("/receipts", s.handleReceiptUpload)
	r.Get("/receipts/file/{id}", s.handleReceiptFile)
	r.Post("/receipts/link", s.handleReceiptLink)

	r.Get("/foreign-tax", s.handleForeignTax)
	r.Post("/foreign-tax", s.handleForeignTaxUpdate)

	r.Get("/tax-info", s.handleTaxInfo)

	r.Get("/reports", s.handleReports)
	r.Get("/reports/annual.pdf", s.handleAnnualPDF)
	r.Get("/reports/tax-summary.pdf", s.handleTaxSummaryPDF)
	r.Get("/reports/transactions.xlsx", s.handleTransactionsXLSX)
	r.Get("/reports/naeringsspesifikasjon.xlsx", s.handleNaeringsspesifikasjonXLSX)
	r.Get("/reports/transactions.csv", s.handleTransactionsCSV)
	r.Get("/export/backup.zip", s.handleBackup)

	r.Get("/changelog", s.handleChangelog)
	r.Post("/changelog/rollback", s.handleRollback)

	return r
}

// view bygger en grunnleggende View med sprak, aar og oversettelser.
func (s *Server) view(r *http.Request, active, title string) View {
	ctx := r.Context()
	lang := s.app.Language(ctx)
	return View{
		Lang:   lang,
		T:      s.renderer.translations(lang),
		Active: active,
		Title:  title,
		Year:   s.app.ActiveYear(ctx),
		Years:  selectableYears(s.app.ActiveYear(ctx)),
	}
}

// selectableYears er registrerte skatteaar pluss aktivt aar (stigende, unikt).
func selectableYears(active int) []int {
	set := map[int]bool{active: true}
	for _, y := range tax.AvailableYears() {
		set[y] = true
	}
	years := make([]int, 0, len(set))
	for y := range set {
		years = append(years, y)
	}
	sort.Ints(years)
	return years
}

// handleSetLang bytter sprak (uten omstart) og gaar tilbake til forrige side.
func (s *Server) handleSetLang(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")
	if lang == "nb" || lang == "pt" || lang == "en" {
		_ = s.app.SetConfig(r.Context(), core.ConfigLanguage, lang)
	}
	dest := r.Header.Get("Referer")
	if dest == "" {
		dest = "/"
	}
	http.Redirect(w, r, dest, http.StatusSeeOther)
}

// handleSetYear setter aktivt inntektsaar og gaar tilbake til forrige side.
func (s *Server) handleSetYear(w http.ResponseWriter, r *http.Request) {
	year := parseInt(r.URL.Query().Get("year"))
	if year >= 2000 && year <= 2100 {
		_ = s.app.SetConfig(r.Context(), core.ConfigActiveYear, strconv.Itoa(year))
	}
	dest := r.Header.Get("Referer")
	if dest == "" {
		dest = "/"
	}
	http.Redirect(w, r, dest, http.StatusSeeOther)
}

// tr returnerer en oversettelse for forespoerselens sprak.
func (s *Server) tr(r *http.Request, key string) string {
	m := s.renderer.translations(s.app.Language(r.Context()))
	if v, ok := m[key]; ok {
		return v
	}
	return key
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	v := s.view(r, "dashboard", "")
	d, err := s.app.Dashboard(r.Context(), v.Year)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	v.Data = d
	s.renderer.Render(w, "dashboard", v)
}

func (s *Server) handleExchangeRate(w http.ResponseWriter, r *http.Request) {
	currency := r.URL.Query().Get("currency")
	date := r.URL.Query().Get("date")
	if currency == "" || date == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mangler currency eller date"})
		return
	}
	rate, err := s.app.Currency.Rate(r.Context(), currency, date)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rate)
}

// handleEvents er SSE-endepunktet for live oppdatering.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming stottes ikke", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch, unsubscribe := s.app.Events.Subscribe()
	defer unsubscribe()

	// Innledende kommentar slik at klienten vet at strommen er aapen.
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", core.EncodeEvent(ev))
			flusher.Flush()
		case <-ping.C:
			fmt.Fprintf(w, "data: {\"type\":\"ping\"}\n\n")
			flusher.Flush()
		}
	}
}

// stub returnerer 501 for ruter som ikke er implementert enda.
func (s *Server) stub(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, name+": ikke implementert enda", http.StatusNotImplemented)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
