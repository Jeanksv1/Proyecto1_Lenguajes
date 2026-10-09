// Package model define la estructura comun (envelope) de los eventos de GameStats.
package model

import "time"

// Event es el formato de entrada definido en events_spec.json.
// Los campos especificos de cada juego viajan en Data.
type Event struct {
	EventID     string         `json:"eventId"`
	Timestamp   string         `json:"timestamp"`
	GameID      string         `json:"gameId"`
	GameVersion string         `json:"gameVersion"`
	MatchID     string         `json:"matchId"`
	PlayerID    *string        `json:"playerId"`
	Type        string         `json:"type"`
	Action      *string        `json:"action,omitempty"`
	Data        map[string]any `json:"data"`

	// Parsed lo llena el validador a partir de Timestamp. No viaja por la red.
	Parsed time.Time `json:"-"`
}
