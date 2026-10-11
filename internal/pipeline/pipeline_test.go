package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"gamestats/internal/analysis"
	"gamestats/internal/store"
	"gamestats/internal/validator"
)

var base = time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)

// mk construye un evento JSON. player debe ser JSON crudo: `null` o `"p1"`.
func mk(id, matchID string, offsetSec int, player, typ, data string) []byte {
	ts := base.Add(time.Duration(offsetSec) * time.Second).Format(time.RFC3339)
	return []byte(fmt.Sprintf(
		`{"eventId":%q,"timestamp":%q,"gameId":"fps-arena","gameVersion":"1.0.0","matchId":%q,"playerId":%s,"type":%q,"data":%s}`,
		id, ts, matchID, player, typ, data))
}

// matchEvents devuelve 7 eventos de una partida, en orden cronologico.
func matchEvents(m string) [][]byte {
	return [][]byte{
		mk(m+"-1", m, 0, `null`, "MATCH_STARTED", `{"map":"dust","mode":"dm","maxPlayers":4}`),
		mk(m+"-2", m, 1, `"p1"`, "PLAYER_JOINED", `{"nickname":"A"}`),
		mk(m+"-3", m, 2, `"p2"`, "PLAYER_JOINED", `{"nickname":"B"}`),
		mk(m+"-4", m, 10, `"p1"`, "PLAYER_ELIMINATED", `{"victimId":"p2","weapon":"rifle"}`),
		mk(m+"-5", m, 20, `"p1"`, "PLAYER_ELIMINATED", `{"victimId":"p2","weapon":"rifle"}`),
		mk(m+"-6", m, 30, `"p1"`, "PLAYER_ELIMINATED", `{"victimId":"p2","weapon":"pistol"}`),
		mk(m+"-7", m, 60, `null`, "MATCH_FINISHED", `{"reason":"TIME_LIMIT"}`),
	}
}

func newTestPipeline(t *testing.T, scalaURL string) (*Pipeline, *store.Store, *store.RejectedLog) {
	t.Helper()
	cfg, err := validator.LoadConfig("../../config/event_types.json")
	if err != nil {
		t.Fatal(err)
	}
	st := store.New()
	rej := store.NewRejectedLog(100)
	var client *analysis.Client
	if scalaURL != "" {
		client = analysis.NewClient(scalaURL)
	}
	p := New(validator.New(cfg), st, rej, client, Options{IngestWorkers: 4, AnalysisWorkers: 2}, log.New(io.Discard, "", 0))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = p.Shutdown(ctx)
	})
	return p, st, rej
}

func waitStatus(t *testing.T, st *store.Store, id string, want store.Status) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if s, ok := st.Summary(id); ok && s.Status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	s, _ := st.Summary(id)
	t.Fatalf("match %s: expected status %s, got %s", id, want, s.Status)
}

// fakeScala simula el modulo Scala y guarda lo que recibe.
type fakeScala struct {
	mu  sync.Mutex
	got map[string]analysis.Request
	srv *httptest.Server
}

func newFakeScala(t *testing.T) *fakeScala {
	f := &fakeScala{got: map[string]analysis.Request{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req analysis.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.got[req.MatchID] = req
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func TestFullMatchIsSentToScalaSortedByTimestamp(t *testing.T) {
	scala := newFakeScala(t)
	p, st, _ := newTestPipeline(t, scala.srv.URL)

	// Orden desordenado (MATCH_FINISHED al final, como en una partida real).
	evs := matchEvents("m1")
	order := []int{5, 3, 4, 2, 1, 0, 6}
	for _, i := range order {
		if err := p.Ingest(evs[i]); err != nil {
			t.Fatalf("event %d rejected: %v", i, err)
		}
	}
	waitStatus(t, st, "m1", store.StatusAnalyzed)

	scala.mu.Lock()
	defer scala.mu.Unlock()
	req, ok := scala.got["m1"]
	if !ok {
		t.Fatal("scala did not receive match m1")
	}
	if len(req.Events) != 7 {
		t.Fatalf("expected 7 events, got %d", len(req.Events))
	}
	if req.Events[0].Type != "MATCH_STARTED" || req.Events[6].Type != "MATCH_FINISHED" {
		t.Fatalf("events not sorted by timestamp: first=%s last=%s", req.Events[0].Type, req.Events[6].Type)
	}
	if !req.Complete {
		t.Fatal("expected complete=true")
	}
	if res, _, _ := st.Result("m1"); string(res) != `{"ok":true}` {
		t.Fatalf("unexpected stored result: %s", res)
	}
}

func TestDuplicateEventIsRejected(t *testing.T) {
	p, _, rej := newTestPipeline(t, "")
	evs := matchEvents("m2")
	if err := p.Ingest(evs[0]); err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	err := p.Ingest(evs[0])
	if err == nil || err.Code != validator.CodeDuplicateEvent {
		t.Fatalf("expected DUPLICATE_EVENT, got %v", err)
	}
	if err.HTTPStatus() != http.StatusConflict {
		t.Fatalf("expected 409, got %d", err.HTTPStatus())
	}
	if rej.Total() != 1 {
		t.Fatalf("expected 1 rejection logged, got %d", rej.Total())
	}
}

func TestMalformedAndInvalidEvents(t *testing.T) {
	p, _, _ := newTestPipeline(t, "")
	if err := p.Ingest([]byte(`{ "eventId": "x", `)); err == nil || err.Code != validator.CodeMalformedJSON {
		t.Fatalf("expected MALFORMED_JSON, got %v", err)
	}
	bad := mk("bad-1", "m3", 0, `"p1"`, "PLAYER_TELEPORTED", `{}`)
	if err := p.Ingest(bad); err == nil || err.Code != validator.CodeUnknownType {
		t.Fatalf("expected UNKNOWN_TYPE, got %v", err)
	}
}

func TestEventAfterFinishIsRejected(t *testing.T) {
	p, st, rej := newTestPipeline(t, "")
	for _, e := range matchEvents("m4") {
		if err := p.Ingest(e); err != nil {
			t.Fatalf("ingest: %v", err)
		}
	}
	waitStatus(t, st, "m4", store.StatusAnalysisSkipped)

	late := mk("m4-late", "m4", 70, `"p1"`, "PLAYER_JOINED", `{"nickname":"Late"}`)
	if err := p.Ingest(late); err != nil {
		t.Fatalf("late event should be accepted into the queue: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, r := range rej.List() {
			if r.Code == validator.CodeMatchAlreadyFinished {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected MATCH_ALREADY_FINISHED in rejected log")
}

// Varias partidas en paralelo: ejecutar con -race.
func TestManyMatchesInParallel(t *testing.T) {
	scala := newFakeScala(t)
	p, st, _ := newTestPipeline(t, scala.srv.URL)

	const n = 30
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for _, e := range matchEvents(fmt.Sprintf("par-%02d", i)) {
				if err := p.Ingest(e); err != nil {
					t.Errorf("match %d rejected: %v", i, err)
					return
				}
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		waitStatus(t, st, fmt.Sprintf("par-%02d", i), store.StatusAnalyzed)
	}
	scala.mu.Lock()
	defer scala.mu.Unlock()
	if len(scala.got) != n {
		t.Fatalf("scala received %d matches, expected %d", len(scala.got), n)
	}
	for id, req := range scala.got {
		if len(req.Events) != 7 {
			t.Errorf("match %s: expected 7 events, got %d", id, len(req.Events))
		}
	}
}
