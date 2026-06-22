package core

import (
	"encoding/json"
	"sync"
)

// Event er en endringshendelse som kringkastes til abonnenter (nettleseren)
// slik at UI-et kan oppdatere seg live når data endres - også når
// endringen kommer fra MCP-agenten.
type Event struct {
	Type   string `json:"type"`   // f.eks. "income", "expense", "rollback"
	Action string `json:"action"` // "created", "updated", "deleted"
	Entity string `json:"entity"` // tabellnavn
	ID     int64  `json:"id"`     // rad-id (0 hvis ikke relevant)
	Year   int    `json:"year"`   // berort inntektsår (0 hvis ikke relevant)
}

// Hub er en enkel SSE-kringkaster. Abonnenter registrerer en kanal og får
// alle påfølgende hendelser.
type Hub struct {
	mu   sync.Mutex
	subs map[chan Event]struct{}
}

// NewHub lager en tom hub.
func NewHub() *Hub {
	return &Hub{subs: map[chan Event]struct{}{}}
}

// Subscribe registrerer en ny abonnent og returnerer kanalen samt en
// unsubscribe-funksjon.
func (h *Hub) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 16)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		if _, ok := h.subs[ch]; ok {
			delete(h.subs, ch)
			close(ch)
		}
		h.mu.Unlock()
	}
}

// Broadcast sender en hendelse til alle abonnenter. Trege abonnenter hopper
// over hendelsen (ikke-blokkerende) i stedet for å henge hele appen.
func (h *Hub) Broadcast(ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// Count returnerer antall aktive abonnenter (brukes i test).
func (h *Hub) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}

// EncodeEvent serialiserer en hendelse til JSON for SSE-data-feltet.
func EncodeEvent(ev Event) string {
	b, _ := json.Marshal(ev)
	return string(b)
}
