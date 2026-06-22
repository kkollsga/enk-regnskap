package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kkollsga/enk-regnskap/internal/currency"
)

// Workspace forvalter flere prosjekter (ett per foretak) under en basismappe,
// f.eks. ~/ENK-Regnskap/. Hvert prosjekt er en undermappe kalt
// "<firmanavn> - <org.nummer>/" med egen database, kvitteringer og mirror.
// Workspace holder den aktive App-instansen og kan bytte prosjekt i farten.
type Workspace struct {
	BaseDir  string
	provider currency.ExchangeRateProvider

	mu      sync.RWMutex
	current *App
	curName string
}

// Project beskriver ett foretaksprosjekt (en undermappe).
type Project struct {
	Folder  string // mappenavn, f.eks. "Acme AS - 999888777"
	Company string
	OrgNr   string
	Path    string // absolutt sti
}

// DefaultBaseDir er ~/ENK-Regnskap.
func DefaultBaseDir() string {
	if v := os.Getenv("ENK_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "ENK-Regnskap"
	}
	return filepath.Join(home, "ENK-Regnskap")
}

// NewWorkspace lager et workspace og åpner sist aktive prosjekt (eller det
// eneste som finnes). Hvis ingen prosjekter finnes er Current() nil til et
// prosjekt opprettes.
func NewWorkspace(baseDir string, provider currency.ExchangeRateProvider) (*Workspace, error) {
	if baseDir == "" {
		baseDir = DefaultBaseDir()
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("opprett basismappe: %w", err)
	}
	w := &Workspace{BaseDir: baseDir, provider: provider}

	projects, err := w.Projects()
	if err != nil {
		return nil, err
	}
	// Åpne sist aktive, ellers det eneste prosjektet.
	active := w.readActive()
	switch {
	case active != "" && w.projectExists(active):
		_, _ = w.Open(active)
	case len(projects) == 1:
		_, _ = w.Open(projects[0].Folder)
	}
	return w, nil
}

// Projects lister alle prosjekter (undermapper) i basismappen.
func (w *Workspace) Projects() ([]Project, error) {
	entries, err := os.ReadDir(w.BaseDir)
	if err != nil {
		return nil, err
	}
	var out []Project
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		company, orgnr := parseProjectFolder(e.Name())
		out = append(out, Project{
			Folder: e.Name(), Company: company, OrgNr: orgnr,
			Path: filepath.Join(w.BaseDir, e.Name()),
		})
	}
	return out, nil
}

// Open lukker gjeldende prosjekt og åpner et annet.
func (w *Workspace) Open(folder string) (*App, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.current != nil {
		w.current.Close()
		w.current = nil
	}
	app, err := New(filepath.Join(w.BaseDir, folder), w.provider)
	if err != nil {
		return nil, err
	}
	w.current = app
	w.curName = folder
	w.writeActive(folder)
	return app, nil
}

// CreateProject oppretter et nytt prosjekt fra firmanavn og org.nr og åpner det.
func (w *Workspace) CreateProject(company, orgnr string) (Project, error) {
	folder := ProjectFolderName(company, orgnr)
	if folder == "" {
		return Project{}, fmt.Errorf("ugyldig prosjektnavn")
	}
	path := filepath.Join(w.BaseDir, folder)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return Project{}, err
	}
	if _, err := w.Open(folder); err != nil {
		return Project{}, err
	}
	return Project{Folder: folder, Company: company, OrgNr: orgnr, Path: path}, nil
}

// Current returnerer den aktive App-instansen (kan være nil).
func (w *Workspace) Current() *App {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.current
}

// CurrentName er mappenavnet til det aktive prosjektet.
func (w *Workspace) CurrentName() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.curName
}

// Close lukker det aktive prosjektet.
func (w *Workspace) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.current != nil {
		err := w.current.Close()
		w.current = nil
		return err
	}
	return nil
}

func (w *Workspace) projectExists(folder string) bool {
	info, err := os.Stat(filepath.Join(w.BaseDir, folder))
	return err == nil && info.IsDir()
}

func (w *Workspace) readActive() string {
	b, err := os.ReadFile(filepath.Join(w.BaseDir, ".active"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func (w *Workspace) writeActive(folder string) {
	_ = os.WriteFile(filepath.Join(w.BaseDir, ".active"), []byte(folder), 0o644)
}

// ProjectFolderName lager et trygt mappenavn "<firma> - <orgnr>".
func ProjectFolderName(company, orgnr string) string {
	company = sanitizeSegment(company)
	orgnr = sanitizeSegment(orgnr)
	switch {
	case company == "" && orgnr == "":
		return ""
	case orgnr == "":
		return company
	case company == "":
		return orgnr
	default:
		return company + " - " + orgnr
	}
}

// SplitProjectFolder splitter "<firma> - <orgnr>" tilbake til delene.
func SplitProjectFolder(folder string) (company, orgnr string) {
	return parseProjectFolder(folder)
}

// parseProjectFolder splitter "<firma> - <orgnr>" tilbake til delene.
func parseProjectFolder(folder string) (company, orgnr string) {
	if i := strings.LastIndex(folder, " - "); i >= 0 {
		return strings.TrimSpace(folder[:i]), strings.TrimSpace(folder[i+3:])
	}
	return folder, ""
}

// sanitizeSegment fjerner tegn som ikke er trygge i mappenavn.
func sanitizeSegment(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			b.WriteByte(' ')
		default:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
