package server

import (
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/kkollsga/enk-regnskap/internal/core"
	"github.com/kkollsga/enk-regnskap/internal/db"
)

type receiptsData struct {
	Receipts []db.Receipt
	Filter   string // "all" | "unlinked"
}

func (s *Server) handleReceiptsList(w http.ResponseWriter, r *http.Request) {
	v := s.view(r, "receipts", s.tr(r, "nav_receipts"))
	filter := r.URL.Query().Get("filter")
	var rows []db.Receipt
	var err error
	if filter == "unlinked" {
		rows, err = s.app.ListUnlinkedReceipts(r.Context())
	} else {
		filter = "all"
		rows, err = s.app.ListReceipts(r.Context())
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	v.Data = receiptsData{Receipts: rows, Filter: filter}
	s.renderer.Render(w, "receipts", v)
}

// handleReceiptUpload tar imot en kvittering fra kvitteringssiden.
func (s *Server) handleReceiptUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "ugyldig opplasting", http.StatusBadRequest)
		return
	}
	rid, err := s.maybeUploadReceipt(r, s.app.ActiveYear(r.Context()))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if rid == nil {
		http.Error(w, "ingen fil valgt", http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/receipts?saved=1", http.StatusSeeOther)
}

// maybeUploadReceipt laster opp en eventuell fil i feltet "receipt".
// Returnerer (nil, nil) hvis ingen fil ble sendt.
func (s *Server) maybeUploadReceipt(r *http.Request, year int) (*int64, error) {
	if r.MultipartForm == nil {
		return nil, nil
	}
	file, header, err := r.FormFile("receipt")
	if err != nil {
		return nil, nil // ingen fil - greit (valgfritt)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 32<<20))
	if err != nil {
		return nil, err
	}
	mime := header.Header.Get("Content-Type")
	rec, err := s.app.SaveReceipt(r.Context(), core.ActorWeb, core.ReceiptInput{
		OriginalName: header.Filename,
		MimeType:     mime,
		Data:         data,
		TaxYear:      year,
	})
	if err != nil {
		if ve, ok := core.AsValidation(err); ok {
			return nil, ve
		}
		return nil, err
	}
	return &rec.ID, nil
}

// handleReceiptFile serverer en kvitteringsfil inline (forhaandsvisning).
func (s *Server) handleReceiptFile(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "ugyldig id", http.StatusBadRequest)
		return
	}
	rec, err := s.app.GetReceipt(r.Context(), id)
	if err != nil {
		http.Error(w, "ikke funnet", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", rec.MimeType)
	w.Header().Set("Content-Disposition", "inline; filename=\""+rec.OriginalName+"\"")
	http.ServeFile(w, r, s.app.ReceiptPath(rec))
}

// handleReceiptLink knytter en kvittering til en transaksjon.
func (s *Server) handleReceiptLink(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "ugyldig skjema", http.StatusBadRequest)
		return
	}
	receiptID, _ := strconv.ParseInt(r.FormValue("receipt_id"), 10, 64)
	txID, _ := strconv.ParseInt(r.FormValue("tx_id"), 10, 64)
	kind := r.FormValue("kind")
	if err := s.app.LinkReceipt(r.Context(), core.ActorWeb, kind, txID, receiptID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/receipts?saved=1", http.StatusSeeOther)
}
