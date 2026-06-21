package server

import (
	"net/http"
	"strings"

	"github.com/kkollsga/enk-regnskap/internal/core"
)

// onboardingGate omdirigerer til velkomstskjermen til forstegangsoppsettet er
// fullfort. Statiske ressurser, helse, hendelser og selve oppsettet er unntatt.
func (s *Server) onboardingGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		exempt := p == "/welcome" || p == "/onboard" || p == "/health" ||
			p == "/events" || strings.HasPrefix(p, "/static/")
		if !exempt && !s.app.IsOnboarded(r.Context()) {
			http.Redirect(w, r, "/welcome", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleWelcome(w http.ResponseWriter, r *http.Request) {
	// Allerede ferdig? Gaa til dashbordet.
	if s.app.IsOnboarded(r.Context()) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	v := s.view(r, "welcome", "Velkommen")
	v.Data = map[string]string{"DataDir": s.app.DataDir}
	s.renderer.Render(w, "welcome", v)
}

func (s *Server) handleOnboard(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "ugyldig skjema", http.StatusBadRequest)
		return
	}
	in := core.OnboardInput{
		BusinessName: r.FormValue("business_name"),
		OrgNr:        r.FormValue("org_nr"),
		Language:     r.FormValue("language"),
	}
	if err := s.app.CompleteOnboarding(r.Context(), in); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/?saved=1", http.StatusSeeOther)
}
