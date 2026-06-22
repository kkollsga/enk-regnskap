package server

import (
	"net/http"

	"github.com/kkollsga/enk-regnskap/internal/core"
)

// handleBackup streamer en fullstendig sikkerhetskopi (data.db + kvitteringer)
// som en ZIP-fil.
func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	setDownload(w, core.BackupFilename(), "application/zip")
	if err := s.app.WriteBackup(r.Context(), w); err != nil {
		// Header er allerede sendt; logg via 500 best effort.
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleMirrorImport setter tilstanden fra en lesbar mirror-mappe.
func (s *Server) handleMirrorImport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "ugyldig skjema", http.StatusBadRequest)
		return
	}
	dir := r.FormValue("dir")
	if dir == "" {
		dir = s.app.MirrorDir()
	}
	if err := s.app.ImportMirror(r.Context(), dir); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/reports?saved=1", http.StatusSeeOther)
}

// handleMirrorRebuild skriver mirror-mappen paa nytt fra databasen.
func (s *Server) handleMirrorRebuild(w http.ResponseWriter, r *http.Request) {
	if err := s.app.SyncMirror(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/reports?saved=1", http.StatusSeeOther)
}
