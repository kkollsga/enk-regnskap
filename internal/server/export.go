package server

import (
	"io"
	"net/http"

	"github.com/kkollsga/enk-regnskap/internal/core"
)

// handleBackup streamer en fullstendig sikkerhetskopi (data.db + kvitteringer)
// som en ZIP-fil. Dette er den generiske "eksporter"-handlingen.
func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	setDownload(w, core.BackupFilename(), "application/zip")
	if err := s.app().WriteBackup(r.Context(), w); err != nil {
		// Header er allerede sendt; logg via 500 best effort.
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleImportPage viser den generiske import-siden.
func (s *Server) handleImportPage(w http.ResponseWriter, r *http.Request) {
	v := s.view(r, "import", "Importer")
	v.MirrorDir = s.app().MirrorDir()
	s.renderer.Render(w, "import", v)
}

// handleImport tar imot en generisk import: en fil (.zip sikkerhetskopi eller
// .db/.sqlite database) eller en sti til en lesbar speilkopi-mappe.
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseMultipartForm(64 << 20)

	if file, header, err := r.FormFile("file"); err == nil {
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, 512<<20))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.app().Import(r.Context(), header.Filename, data); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(w, r, "/?saved=1", http.StatusSeeOther)
		return
	}

	if dir := r.FormValue("dir"); dir != "" {
		if err := s.app().ImportMirror(r.Context(), dir); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(w, r, "/?saved=1", http.StatusSeeOther)
		return
	}

	http.Error(w, "Velg en fil eller oppgi en mappe", http.StatusBadRequest)
}

// handleGenerateDummy fyller appen med fiktivt testdatasett.
func (s *Server) handleGenerateDummy(w http.ResponseWriter, r *http.Request) {
	if _, err := s.app().GenerateDummyData(r.Context(), core.ActorWeb); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/?saved=1", http.StatusSeeOther)
}
