// Package validator valida eventos contra los tipos definidos en config/event_types.json.
package validator

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Tipos de dato soportados para los campos de "data".
const (
	KindString  = "string"
	KindNumber  = "number"
	KindInteger = "integer"
	KindBoolean = "boolean"
)

// Reglas para playerId segun el tipo de evento.
const (
	PlayerRequired  = "required"
	PlayerForbidden = "forbidden"
	PlayerOptional  = "optional"
)

// FieldSpec describe un campo dentro de "data".
type FieldSpec struct {
	Name               string   `json:"name"`
	Kind               string   `json:"kind"`
	Required           bool     `json:"required,omitempty"`
	Min                *float64 `json:"min,omitempty"`
	Allowed            []string `json:"allowed,omitempty"`
	NotEqualToPlayerID bool     `json:"notEqualToPlayerId,omitempty"`
	// ZeroUnless: el valor debe ser 0 cuando el campo booleano indicado sea false.
	ZeroUnless string `json:"zeroUnless,omitempty"`
	// RequiredIf: el campo es obligatorio cuando el campo booleano indicado sea true.
	RequiredIf string `json:"requiredIf,omitempty"`
}

// TypeSpec describe un tipo de evento.
type TypeSpec struct {
	Description string                 `json:"description,omitempty"`
	Player      string                 `json:"player"`
	StartsMatch bool                   `json:"startsMatch,omitempty"`
	EndsMatch   bool                   `json:"endsMatch,omitempty"`
	Fields      []FieldSpec            `json:"fields,omitempty"`
	Actions     map[string][]FieldSpec `json:"actions,omitempty"`
}

// Config es el contenido completo de event_types.json.
type Config struct {
	Types map[string]TypeSpec `json:"types"`
}

// LoadConfig lee y verifica el archivo de tipos de evento.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading event types config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if err := cfg.check(); err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return &cfg, nil
}

func (c *Config) check() error {
	if len(c.Types) == 0 {
		return errors.New("no event types defined")
	}
	for name, t := range c.Types {
		switch t.Player {
		case "":
			t.Player = PlayerOptional
			c.Types[name] = t
		case PlayerRequired, PlayerForbidden, PlayerOptional:
		default:
			return fmt.Errorf("type %s: unknown player rule %q", name, t.Player)
		}
		all := append([]FieldSpec{}, t.Fields...)
		for _, af := range t.Actions {
			all = append(all, af...)
		}
		for _, f := range all {
			if f.Name == "" {
				return fmt.Errorf("type %s: field without name", name)
			}
			switch f.Kind {
			case KindString, KindNumber, KindInteger, KindBoolean:
			default:
				return fmt.Errorf("type %s, field %s: unknown kind %q", name, f.Name, f.Kind)
			}
		}
	}
	return nil
}
