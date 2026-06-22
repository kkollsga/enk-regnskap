package server

import (
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/kkollsga/enk-regnskap/internal/core"
)

// uploadReceipts lagrer alle opplastede vedlegg ("attachment") og knytter dem
// til en inntekt/utgift. Tittel/beskrivelse leses fra parallelle felter.
// Returnerer antall lagrede vedlegg, eller en feil ved ugyldig fil.
func (s *Server) uploadReceipts(r *http.Request, parentKind string, parentID int64, year int) (int, error) {
	if r.MultipartForm == nil {
		return 0, nil
	}
	files := r.MultipartForm.File["attachment"]
	titles := r.MultipartForm.Value["attachment_title"]
	descs := r.MultipartForm.Value["attachment_desc"]
	saved := 0
	for i, header := range files {
		f, err := header.Open()
		if err != nil {
			return saved, err
		}
		data, err := io.ReadAll(io.LimitReader(f, 32<<20))
		f.Close()
		if err != nil {
			return saved, err
		}
		title, desc := "", ""
		if i < len(titles) {
			title = titles[i]
		}
		if i < len(descs) {
			desc = descs[i]
		}
		if _, err := s.app().SaveReceipt(r.Context(), core.ActorWeb, core.ReceiptInput{
			OriginalName: header.Filename,
			MimeType:     header.Header.Get("Content-Type"),
			Data:         data,
			Title:        title,
			Description:  desc,
			ParentKind:   parentKind,
			ParentID:     parentID,
			TaxYear:      year,
		}); err != nil {
			if ve, ok := core.AsValidation(err); ok {
				return saved, ve
			}
			return saved, err
		}
		saved++
	}
	return saved, nil
}

// handleReceiptFile serverer en vedleggsfil inline (forhåndsvisning).
func (s *Server) handleReceiptFile(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "ugyldig id", http.StatusBadRequest)
		return
	}
	rec, err := s.app().GetReceipt(r.Context(), id)
	if err != nil {
		http.Error(w, "ikke funnet", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", rec.MimeType)
	w.Header().Set("Content-Disposition", "inline; filename=\""+rec.OriginalName+"\"")
	http.ServeFile(w, r, s.app().ReceiptPath(rec))
}

// handleReceiptMeta oppdaterer tittel/beskrivelse på et vedlegg.
func (s *Server) handleReceiptMeta(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "ugyldig skjema", http.StatusBadRequest)
		return
	}
	if err := s.app().UpdateReceiptMeta(r.Context(), core.ActorWeb, id,
		r.FormValue("title"), r.FormValue("description")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	redirectBack(w, r)
}

// handleReceiptDelete sletter et vedlegg.
func (s *Server) handleReceiptDelete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := s.app().DeleteReceipt(r.Context(), core.ActorWeb, id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	redirectBack(w, r)
}

// attachmentTypeError sjekker filtypene før posten lagres (unngår delvis lagring).
func attachmentTypeError(r *http.Request) string {
	if r.MultipartForm == nil {
		return ""
	}
	for _, h := range r.MultipartForm.File["attachment"] {
		if !core.ReceiptTypeAllowed(h.Header.Get("Content-Type")) {
			return "Ugyldig filtype for vedlegg: " + h.Filename + " (bruk bilde eller PDF)."
		}
	}
	return ""
}

func redirectBack(w http.ResponseWriter, r *http.Request) {
	dest := r.Header.Get("Referer")
	if dest == "" {
		dest = "/"
	}
	http.Redirect(w, r, dest, http.StatusSeeOther)
}
