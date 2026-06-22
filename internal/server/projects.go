package server

import (
	"fmt"
	"net/http"

	"github.com/kkollsga/enk-regnskap/internal/core"
)

type projectsData struct {
	BaseDir  string
	Projects []core.Project
	Current  string
}

// handleProjects viser prosjektvelgeren (foretak under ~/ENK-Regnskap/).
func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	if s.ws == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	projects, err := s.ws.Projects()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	v := s.view(r, "projects", "Prosjekter")
	v.Data = projectsData{
		BaseDir:  s.ws.BaseDir,
		Projects: projects,
		Current:  s.ws.CurrentName(),
	}
	s.renderer.Render(w, "projects", v)
}

// handleProjectOpen bytter til et eksisterende prosjekt.
func (s *Server) handleProjectOpen(w http.ResponseWriter, r *http.Request) {
	if s.ws == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	folder := r.FormValue("folder")
	if _, err := s.ws.Open(folder); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleProjectDemo oppretter et demo-foretak fylt med fiktive testdata, slik
// at man kan prøve appen uten å registrere noe selv. Foretaket blir liggende i
// listen og kan velges som et hvilket som helst annet foretak.
func (s *Server) handleProjectDemo(w http.ResponseWriter, r *http.Request) {
	if s.ws == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	company := s.uniqueDemoName()
	if _, err := s.ws.CreateProject(company, "000000000"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.app().CompleteOnboarding(r.Context(), core.OnboardInput{
		BusinessName: company, OrgNr: "000000000", Language: "nb",
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Bruk et år med fullstendige skatteregler for et komplett eksempel.
	_ = s.app().SetConfig(r.Context(), core.ConfigActiveYear, "2025")
	if _, err := s.app().GenerateDummyData(r.Context(), core.ActorSystem); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Tilbake til prosjektvelgeren – demo-foretaket er nå aktivt og valgbart.
	http.Redirect(w, r, "/projects", http.StatusSeeOther)
}

// uniqueDemoName finner et ledig navn for et demo-foretak.
func (s *Server) uniqueDemoName() string {
	const base = "Demo-foretak"
	taken := map[string]bool{}
	if projects, err := s.ws.Projects(); err == nil {
		for _, p := range projects {
			taken[p.Folder] = true
		}
	}
	name := base
	for i := 2; taken[core.ProjectFolderName(name, "000000000")]; i++ {
		name = fmt.Sprintf("%s %d", base, i)
	}
	return name
}

// handleProjectCreate oppretter et nytt foretaksprosjekt og fullfører oppsett.
func (s *Server) handleProjectCreate(w http.ResponseWriter, r *http.Request) {
	if s.ws == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	company := r.FormValue("company")
	orgnr := r.FormValue("org_nr")
	lang := r.FormValue("language")
	if core.ProjectFolderName(company, orgnr) == "" {
		http.Error(w, "Oppgi firmanavn og/eller organisasjonsnummer", http.StatusBadRequest)
		return
	}
	proj, err := s.ws.CreateProject(company, orgnr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Fullfør oppsettet for det nye prosjektet.
	if err := s.app().CompleteOnboarding(r.Context(), core.OnboardInput{
		BusinessName: proj.Company, OrgNr: proj.OrgNr, Language: lang,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/?saved=1", http.StatusSeeOther)
}
