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
