package server

import (
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

// handleProjectCreate oppretter et nytt foretaksprosjekt og fullforer oppsett.
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
	// Fullfor oppsettet for det nye prosjektet.
	if err := s.app().CompleteOnboarding(r.Context(), core.OnboardInput{
		BusinessName: proj.Company, OrgNr: proj.OrgNr, Language: lang,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/?saved=1", http.StatusSeeOther)
}
