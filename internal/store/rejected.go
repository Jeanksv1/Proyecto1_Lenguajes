package store

import (
	"sync"
	"time"
)

// Rejection registra un evento rechazado y el motivo.
type Rejection struct {
	At      time.Time `json:"at"`
	EventID string    `json:"eventId,omitempty"`
	MatchID string    `json:"matchId,omitempty"`
	Type    string    `json:"type,omitempty"`
	Code    string    `json:"code"`
	Message string    `json:"message"`
}

// RejectedLog guarda los ultimos N eventos rechazados (en memoria).
type RejectedLog struct {
	mu    sync.Mutex
	items []Rejection
	max   int
	total int
}

// NewRejectedLog crea un log que conserva como maximo "max" entradas.
func NewRejectedLog(max int) *RejectedLog {
	if max <= 0 {
		max = 1000
	}
	return &RejectedLog{max: max}
}

// Add agrega una entrada, descartando la mas antigua si se llena.
func (l *RejectedLog) Add(r Rejection) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.total++
	if len(l.items) >= l.max {
		copy(l.items, l.items[1:])
		l.items = l.items[:len(l.items)-1]
	}
	l.items = append(l.items, r)
}

// List devuelve una copia de las entradas (de la mas antigua a la mas reciente).
func (l *RejectedLog) List() []Rejection {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Rejection, len(l.items))
	copy(out, l.items)
	return out
}

// Total devuelve cuantos eventos se han rechazado desde que inicio el servidor.
func (l *RejectedLog) Total() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.total
}
