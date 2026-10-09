package validator

import (
	"encoding/json"
	"os"
	"testing"

	"gamestats/internal/model"
)

type specFile struct {
	Valid []struct {
		Description string          `json:"description"`
		Event       json.RawMessage `json:"event"`
	} `json:"validEventExamples"`
	Invalid []struct {
		Description   string          `json:"description"`
		ExpectedError string          `json:"expectedError"`
		Event         json.RawMessage `json:"event"`
		RawBody       string          `json:"rawBody"`
	} `json:"invalidEventExamples"`
}

func loadValidator(t *testing.T) *Validator {
	t.Helper()
	cfg, err := LoadConfig("../../config/event_types.json")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return New(cfg)
}

func loadSpec(t *testing.T) specFile {
	t.Helper()
	data, err := os.ReadFile("../../testdata/events_spec.json")
	if err != nil {
		t.Fatalf("reading events_spec.json: %v", err)
	}
	var spec specFile
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("parsing events_spec.json: %v", err)
	}
	return spec
}

// Todos los ejemplos validos del Dia 1 deben pasar la validacion.
func TestValidExamplesFromSpec(t *testing.T) {
	v := loadValidator(t)
	spec := loadSpec(t)
	if len(spec.Valid) == 0 {
		t.Fatal("no valid examples found in spec")
	}
	for _, ex := range spec.Valid {
		t.Run(ex.Description, func(t *testing.T) {
			var ev model.Event
			if err := json.Unmarshal(ex.Event, &ev); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if verr := v.Validate(&ev); verr != nil {
				t.Fatalf("expected valid, got %v", verr)
			}
			if ev.Parsed.IsZero() {
				t.Fatal("Parsed timestamp was not filled")
			}
		})
	}
}

// Cada ejemplo invalido del Dia 1 debe fallar con el codigo esperado.
// DUPLICATE_EVENT depende del estado (se prueba en el pipeline); MALFORMED_JSON se prueba aparte.
func TestInvalidExamplesFromSpec(t *testing.T) {
	v := loadValidator(t)
	spec := loadSpec(t)
	if len(spec.Invalid) == 0 {
		t.Fatal("no invalid examples found in spec")
	}
	for _, ex := range spec.Invalid {
		if ex.ExpectedError == CodeDuplicateEvent || ex.ExpectedError == CodeMalformedJSON {
			continue
		}
		t.Run(ex.Description, func(t *testing.T) {
			var ev model.Event
			if err := json.Unmarshal(ex.Event, &ev); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			verr := v.Validate(&ev)
			if verr == nil {
				t.Fatalf("expected %s, but event was accepted", ex.ExpectedError)
			}
			if verr.Code != ex.ExpectedError {
				t.Fatalf("expected %s, got %s (%s)", ex.ExpectedError, verr.Code, verr.Message)
			}
		})
	}
}

func TestMalformedRawBodyIsNotValidJSON(t *testing.T) {
	spec := loadSpec(t)
	found := false
	for _, ex := range spec.Invalid {
		if ex.ExpectedError == CodeMalformedJSON {
			found = true
			var ev model.Event
			if err := json.Unmarshal([]byte(ex.RawBody), &ev); err == nil {
				t.Fatal("rawBody should not be valid JSON")
			}
		}
	}
	if !found {
		t.Fatal("spec has no MALFORMED_JSON example")
	}
}
