// Package store mantiene en memoria el estado de las partidas.
//
// Proteccion de concurrencia:
//   - matches: sync.RWMutex (muchos lectores en la API, escrituras de los workers).
//   - seen (eventIds ya recibidos): sync.Map.
package store

import (
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"gamestats/internal/model"
)

// ErrMatchClosed indica que la partida ya termino y no acepta mas eventos.
var ErrMatchClosed = errors.New("match already finished")

// Status es el estado de una partida dentro de Go.
type Status string

const (
	StatusPending         Status = "pending"          // hay eventos pero aun no llega MATCH_STARTED
	StatusInProgress      Status = "in_progress"      // partida en curso
	StatusFinished        Status = "finished"         // terminada, esperando analisis
	StatusAnalyzed        Status = "analyzed"         // Scala respondio con el resultado
	StatusAnalysisFailed  Status = "analysis_failed"  // Scala no respondio / error
	StatusAnalysisSkipped Status = "analysis_skipped" // no hay SCALA_URL configurada
)

type match struct {
	id          string
	gameID      string
	gameVersion string
	status      Status
	complete    bool
	players     map[string]struct{}
	events      []model.Event
	startedAt   *time.Time
	finishedAt  *time.Time
	lastEventAt time.Time
	result      json.RawMessage
	analysisErr string
}

// Snapshot es una copia inmutable de una partida, con eventos ordenados por timestamp.
type Snapshot struct {
	MatchID     string
	GameID      string
	GameVersion string
	Status      Status
	Complete    bool
	Events      []model.Event
	Players     []string
}

// Summary es la vista resumida de una partida para la API.
type Summary struct {
	MatchID       string     `json:"matchId"`
	GameID        string     `json:"gameId"`
	GameVersion   string     `json:"gameVersion"`
	Status        Status     `json:"status"`
	Complete      bool       `json:"complete"`
	Players       []string   `json:"players"`
	EventCount    int        `json:"eventCount"`
	StartedAt     *time.Time `json:"startedAt,omitempty"`
	FinishedAt    *time.Time `json:"finishedAt,omitempty"`
	AnalysisError string     `json:"analysisError,omitempty"`
}

// Store guarda el estado de todas las partidas.
type Store struct {
	mu      sync.RWMutex
	matches map[string]*match
	seen    sync.Map
}

// New crea un store vacio.
func New() *Store {
	return &Store{matches: make(map[string]*match)}
}

// RegisterEventID marca un eventId como recibido. Devuelve false si ya existia.
func (s *Store) RegisterEventID(id string) bool {
	_, loaded := s.seen.LoadOrStore(id, struct{}{})
	return !loaded
}

// ForgetEventID deshace RegisterEventID (por ejemplo si el evento no pudo encolarse).
func (s *Store) ForgetEventID(id string) {
	s.seen.Delete(id)
}

// AddEvent agrega un evento a su partida (creandola si no existe).
// starts/ends indican si el tipo de evento inicia o termina la partida.
// Devuelve true si este evento cerro la partida.
func (s *Store) AddEvent(ev model.Event, starts, ends bool) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, ok := s.matches[ev.MatchID]
	if !ok {
		m = &match{
			id:          ev.MatchID,
			gameID:      ev.GameID,
			gameVersion: ev.GameVersion,
			status:      StatusPending,
			players:     make(map[string]struct{}),
		}
		s.matches[ev.MatchID] = m
	}
	if m.status != StatusPending && m.status != StatusInProgress {
		return false, ErrMatchClosed
	}

	m.events = append(m.events, ev)
	m.lastEventAt = time.Now()
	if ev.PlayerID != nil && *ev.PlayerID != "" {
		m.players[*ev.PlayerID] = struct{}{}
	}

	if starts && m.startedAt == nil {
		t := ev.Parsed
		m.startedAt = &t
		m.status = StatusInProgress
	}
	if ends {
		t := ev.Parsed
		m.finishedAt = &t
		m.complete = m.startedAt != nil
		m.status = StatusFinished
		return true, nil
	}
	return false, nil
}

// Snapshot devuelve una copia de la partida con los eventos ordenados por timestamp.
func (s *Store) Snapshot(id string) (Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.matches[id]
	if !ok {
		return Snapshot{}, false
	}
	events := make([]model.Event, len(m.events))
	copy(events, m.events)
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Parsed.Before(events[j].Parsed)
	})
	return Snapshot{
		MatchID:     m.id,
		GameID:      m.gameID,
		GameVersion: m.gameVersion,
		Status:      m.status,
		Complete:    m.complete,
		Events:      events,
		Players:     sortedPlayers(m),
	}, true
}

// Summary devuelve el resumen de una partida.
func (s *Store) Summary(id string) (Summary, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.matches[id]
	if !ok {
		return Summary{}, false
	}
	return summarize(m), true
}

// List devuelve el resumen de todas las partidas, ordenado por matchId.
func (s *Store) List() []Summary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Summary, 0, len(s.matches))
	for _, m := range s.matches {
		out = append(out, summarize(m))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MatchID < out[j].MatchID })
	return out
}

// SetAnalysis guarda el resultado (o el error) del analisis de Scala.
func (s *Store) SetAnalysis(id string, status Status, result json.RawMessage, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.matches[id]; ok {
		m.status = status
		m.result = result
		m.analysisErr = errMsg
	}
}

// Result devuelve el resultado de Scala (si existe) y el estado actual de la partida.
func (s *Store) Result(id string) (json.RawMessage, Status, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.matches[id]
	if !ok {
		return nil, "", false
	}
	return m.result, m.status, true
}

// ExpireIdle cierra como incompletas las partidas sin eventos durante "timeout".
// Devuelve los matchId cerrados para que se envien a analisis.
func (s *Store) ExpireIdle(timeout time.Duration) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	var ids []string
	for id, m := range s.matches {
		open := m.status == StatusPending || m.status == StatusInProgress
		if open && now.Sub(m.lastEventAt) > timeout {
			m.status = StatusFinished
			m.complete = false
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func summarize(m *match) Summary {
	return Summary{
		MatchID:       m.id,
		GameID:        m.gameID,
		GameVersion:   m.gameVersion,
		Status:        m.status,
		Complete:      m.complete,
		Players:       sortedPlayers(m),
		EventCount:    len(m.events),
		StartedAt:     m.startedAt,
		FinishedAt:    m.finishedAt,
		AnalysisError: m.analysisErr,
	}
}

func sortedPlayers(m *match) []string {
	players := make([]string, 0, len(m.players))
	for p := range m.players {
		players = append(players, p)
	}
	sort.Strings(players)
	return players
}
