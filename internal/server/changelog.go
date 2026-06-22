package server

import (
	"net/http"
	"strconv"

	"github.com/kkollsga/enk-regnskap/internal/core"
	"github.com/kkollsga/enk-regnskap/internal/db"
)

type changelogData struct {
	Entries []db.ChangeLog
}

func (s *Server) handleChangelog(w http.ResponseWriter, r *http.Request) {
	v := s.view(r, "changelog", s.tr(r, "nav_changelog"))
	entries, err := s.app().Q.ListChangeLog(r.Context(), 200)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	v.Data = changelogData{Entries: entries}
	s.renderer.Render(w, "changelog", v)
}

func (s *Server) handleRollback(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "ugyldig skjema", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "ugyldig id", http.StatusBadRequest)
		return
	}
	if err := s.app().Rollback(r.Context(), core.ActorWeb, id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/changelog?saved=1", http.StatusSeeOther)
}
