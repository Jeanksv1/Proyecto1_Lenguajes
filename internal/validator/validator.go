package validator

import (
	"fmt"
	"math"
	"strings"
	"time"

	"gamestats/internal/model"
)

// Codigos de error (ver events_spec.json -> validationRules.errorCodes).
const (
	CodeMalformedJSON        = "MALFORMED_JSON"
	CodeMissingField         = "MISSING_FIELD"
	CodeInvalidTimestamp     = "INVALID_TIMESTAMP"
	CodeUnknownType          = "UNKNOWN_TYPE"
	CodeUnknownAction        = "UNKNOWN_ACTION"
	CodeInvalidFieldValue    = "INVALID_FIELD_VALUE"
	CodeDuplicateEvent       = "DUPLICATE_EVENT"
	CodeMatchAlreadyFinished = "MATCH_ALREADY_FINISHED"
)

// Error es un error de validacion con su codigo.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Validator valida eventos usando una Config. Es seguro para uso concurrente
// porque la configuracion es de solo lectura despues de cargarse.
type Validator struct {
	cfg *Config
}

// New crea un validador.
func New(cfg *Config) *Validator { return &Validator{cfg: cfg} }

// TypeSpec devuelve la definicion de un tipo de evento.
func (v *Validator) TypeSpec(name string) (TypeSpec, bool) {
	t, ok := v.cfg.Types[name]
	return t, ok
}

// Validate comprueba el evento. Si es valido llena ev.Parsed y devuelve nil.
func (v *Validator) Validate(ev *model.Event) *Error {
	required := []struct{ name, val string }{
		{"eventId", ev.EventID},
		{"timestamp", ev.Timestamp},
		{"gameId", ev.GameID},
		{"gameVersion", ev.GameVersion},
		{"matchId", ev.MatchID},
		{"type", ev.Type},
	}
	for _, f := range required {
		if strings.TrimSpace(f.val) == "" {
			return missing(f.name)
		}
	}

	ts, err := time.Parse(time.RFC3339Nano, ev.Timestamp)
	if err != nil {
		return &Error{CodeInvalidTimestamp, fmt.Sprintf("timestamp %q is not a valid RFC 3339 value", ev.Timestamp)}
	}
	ev.Parsed = ts.UTC()

	spec, ok := v.cfg.Types[ev.Type]
	if !ok {
		return &Error{CodeUnknownType, fmt.Sprintf("event type %q is not defined", ev.Type)}
	}

	playerID := ""
	if ev.PlayerID != nil {
		playerID = strings.TrimSpace(*ev.PlayerID)
	}
	switch spec.Player {
	case PlayerRequired:
		if playerID == "" {
			return missing("playerId")
		}
	case PlayerForbidden:
		if playerID != "" {
			return invalid("playerId", "must be null for type "+ev.Type)
		}
	}

	if ev.Data == nil {
		ev.Data = map[string]any{}
	}

	fields := spec.Fields
	if len(spec.Actions) > 0 {
		action := ""
		if ev.Action != nil {
			action = strings.TrimSpace(*ev.Action)
		}
		if action == "" {
			return missing("action")
		}
		actionFields, ok := spec.Actions[action]
		if !ok {
			return &Error{CodeUnknownAction, fmt.Sprintf("action %q is not defined for type %s", action, ev.Type)}
		}
		fields = append(append([]FieldSpec{}, spec.Fields...), actionFields...)
	}

	return checkFields(fields, ev.Data, playerID)
}

func checkFields(specs []FieldSpec, data map[string]any, playerID string) *Error {
	for _, f := range specs {
		name := "data." + f.Name
		raw, present := data[f.Name]
		if !present || raw == nil {
			if f.Required {
				return missing(name)
			}
			continue
		}

		switch f.Kind {
		case KindString:
			s, ok := raw.(string)
			if !ok {
				return invalid(name, "must be a string")
			}
			if strings.TrimSpace(s) == "" {
				if f.Required {
					return missing(name)
				}
				continue
			}
			if len(f.Allowed) > 0 && !contains(f.Allowed, s) {
				return invalid(name, fmt.Sprintf("must be one of %v", f.Allowed))
			}
			if f.NotEqualToPlayerID && s == playerID {
				return invalid(name, "must be different from playerId")
			}

		case KindNumber, KindInteger:
			n, ok := raw.(float64)
			if !ok {
				return invalid(name, "must be a number")
			}
			if f.Kind == KindInteger && n != math.Trunc(n) {
				return invalid(name, "must be an integer")
			}
			if f.Min != nil && n < *f.Min {
				return invalid(name, fmt.Sprintf("must be >= %v", *f.Min))
			}
			if f.ZeroUnless != "" {
				flag, _ := data[f.ZeroUnless].(bool)
				if !flag && n != 0 {
					return invalid(name, fmt.Sprintf("must be 0 when %s is false", f.ZeroUnless))
				}
			}

		case KindBoolean:
			if _, ok := raw.(bool); !ok {
				return invalid(name, "must be a boolean")
			}
		}
	}
	return nil
}

func missing(field string) *Error {
	return &Error{CodeMissingField, fmt.Sprintf("required field %q is missing or empty", field)}
}

func invalid(field, why string) *Error {
	return &Error{CodeInvalidFieldValue, fmt.Sprintf("field %q %s", field, why)}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
