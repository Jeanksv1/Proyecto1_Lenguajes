package simulator

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"reflect"
	"sort"
	"testing"
	"time"

	"gamestats/internal/pipeline"
	"gamestats/internal/store"
	"gamestats/internal/validator"
)

func testConfig() Config {
	return Config{
		Seed:        7,
		Prefix:      "t-",
		Start:       time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
		Matches:     4,
		RaceMatches: 1,
		Players:     3,
		Invalid:     9,
	}
}

func bodyOf(t *testing.T, it Item) []byte {
	t.Helper()
	if it.Raw != "" {
		return []byte(it.Raw)
	}
	b, err := json.Marshal(it.Event)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestGeneratesEnoughEvents(t *testing.T) {
	total := 0
	for _, m := range Generate(testConfig()) {
		total += len(m.Items)
	}
	if total < 50 {
		t.Fatalf("expected at least 50 events, got %d", total)
	}
	t.Logf("total events generated: %d", total)
}

// Genera todo y lo pasa por el modulo Go real (validador + dedup + store):
// los validos deben aceptarse y cada invalido debe fallar con el codigo esperado.
func TestGeneratedEventsAgainstGoModule(t *testing.T) {
	cfg, err := validator.LoadConfig("../../config/event_types.json")
	if err != nil {
		t.Fatal(err)
	}
	st := store.New()
	rej := store.NewRejectedLog(500)
	p := pipeline.New(validator.New(cfg), st, rej, nil, pipeline.Options{}, log.New(io.Discard, "", 0))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = p.Shutdown(ctx)
	})

	matches := Generate(testConfig())
	validCount := map[string]int{}
	for _, m := range matches {
		for i, it := range m.Items {
			ierr := p.Ingest(bodyOf(t, it))
			if it.Expect == "" {
				if ierr != nil {
					t.Fatalf("%s item %d should be valid, got %v", m.ID, i, ierr)
				}
				validCount[m.ID]++
				continue
			}
			if ierr == nil || ierr.Code != it.Expect {
				t.Fatalf("%s item %d: expected %s, got %v", m.ID, i, it.Expect, ierr)
			}
		}
	}

	for _, m := range matches {
		deadline := time.Now().Add(3 * time.Second)
		for {
			s, ok := st.Summary(m.ID)
			if ok && s.Status == store.StatusAnalysisSkipped {
				if s.EventCount != validCount[m.ID] {
					t.Fatalf("%s: store has %d events, expected %d", m.ID, s.EventCount, validCount[m.ID])
				}
				if !s.Complete {
					t.Fatalf("%s: expected complete=true", m.ID)
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s did not finish (status %q)", m.ID, s.Status)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// streakSizes aplica la regla de racha de forma independiente al generador:
// 3+ eliminaciones donde t[i+2]-t[i] < 30 s; las ventanas solapadas se fusionan.
func streakSizes(times []time.Time) []int {
	var res []int
	i := 0
	for i+2 < len(times) {
		if times[i+2].Sub(times[i]) < 30*time.Second {
			j := i + 2
			for j+1 < len(times) && times[j+1].Sub(times[j-1]) < 30*time.Second {
				j++
			}
			res = append(res, j-i+1)
			i = j + 1
		} else {
			i++
		}
	}
	return res
}

// Las rachas "esperadas" que declara el generador deben coincidir con la regla del Dia 1.
func TestStreakLabelsMatchRule(t *testing.T) {
	sawStreak, sawBoundary := false, false
	for _, m := range Generate(testConfig()) {
		times := map[string][]time.Time{}
		for _, it := range m.Items {
			if it.Expect != "" || it.Event == nil || it.Event["type"] != "PLAYER_ELIMINATED" {
				continue
			}
			pid, _ := it.Event["playerId"].(string)
			ts, err := time.Parse(time.RFC3339Nano, it.Event["timestamp"].(string))
			if err != nil {
				t.Fatal(err)
			}
			times[pid] = append(times[pid], ts)
		}
		got := map[string][]int{}
		for pid, ts := range times {
			sort.Slice(ts, func(i, j int) bool { return ts[i].Before(ts[j]) })
			if s := streakSizes(ts); len(s) > 0 {
				got[pid] = s
			}
		}
		want := map[string][]int{}
		for _, s := range m.Streaks {
			want[s.PlayerID] = append(want[s.PlayerID], s.KillCount)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s (%s): streaks by rule %v, declared %v", m.ID, m.Scenario, got, want)
		}
		if len(m.Streaks) > 0 {
			sawStreak = true
		}
		if m.Scenario == "boundary" {
			sawBoundary = true
		}
	}
	if !sawStreak || !sawBoundary {
		t.Fatal("default scenarios should include a streak match and a boundary match")
	}
}

func TestDeterministic(t *testing.T) {
	a, _ := json.Marshal(Generate(testConfig()))
	b, _ := json.Marshal(Generate(testConfig()))
	if string(a) != string(b) {
		t.Fatal("same seed produced different output")
	}
}
