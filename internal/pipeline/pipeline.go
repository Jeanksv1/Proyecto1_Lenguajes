// Package pipeline orquesta el flujo de eventos en Go:
//
//	Ingest (validar + deduplicar) -> shard por matchId -> worker de ingestion
//	    -> store (estado por partida) -> canal de analisis -> worker de analisis -> Scala
//
// Concurrencia:
//   - N workers de ingestion, cada uno con su propio canal. Una partida siempre cae
//     en el mismo worker (hash de matchId), asi sus eventos se procesan en orden y
//     partidas distintas se procesan en paralelo.
//   - M workers de analisis que envian las partidas terminadas a Scala.
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"net/http"
	"sync"
	"time"

	"gamestats/internal/analysis"
	"gamestats/internal/model"
	"gamestats/internal/store"
	"gamestats/internal/validator"
)

// Codigos adicionales de ingestion.
const (
	CodeQueueFull    = "QUEUE_FULL"
	CodeShuttingDown = "SHUTTING_DOWN"
)

// IngestError describe por que un evento fue rechazado al recibirlo.
type IngestError struct {
	Code    string
	Message string
	EventID string
}

func (e *IngestError) Error() string { return e.Code + ": " + e.Message }

// HTTPStatus devuelve el codigo HTTP apropiado para el error.
func (e *IngestError) HTTPStatus() int {
	switch e.Code {
	case validator.CodeDuplicateEvent:
		return http.StatusConflict
	case CodeQueueFull, CodeShuttingDown:
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadRequest
	}
}

// Options configura el pipeline. Los valores <= 0 usan el valor por defecto.
type Options struct {
	IngestWorkers     int           // default 4
	AnalysisWorkers   int           // default 2
	QueueSize         int           // tamano de cada cola, default 1024
	InactivityTimeout time.Duration // 0 = desactivado
	AnalysisTimeout   time.Duration // default 30s
}

// Pipeline es el nucleo de procesamiento de Go.
type Pipeline struct {
	v      *validator.Validator
	st     *store.Store
	rej    *store.RejectedLog
	client *analysis.Client // nil si no hay SCALA_URL
	opts   Options
	log    *log.Logger

	shards     []chan model.Event
	analysisCh chan string

	closeMu     sync.RWMutex
	closed      bool
	stopJanitor chan struct{}

	ingestWG   sync.WaitGroup
	analysisWG sync.WaitGroup
	janitorWG  sync.WaitGroup
}

// New crea el pipeline e inicia sus goroutines.
func New(v *validator.Validator, st *store.Store, rej *store.RejectedLog, client *analysis.Client, opts Options, logger *log.Logger) *Pipeline {
	if opts.IngestWorkers <= 0 {
		opts.IngestWorkers = 4
	}
	if opts.AnalysisWorkers <= 0 {
		opts.AnalysisWorkers = 2
	}
	if opts.QueueSize <= 0 {
		opts.QueueSize = 1024
	}
	if opts.AnalysisTimeout <= 0 {
		opts.AnalysisTimeout = 30 * time.Second
	}

	p := &Pipeline{
		v:           v,
		st:          st,
		rej:         rej,
		client:      client,
		opts:        opts,
		log:         logger,
		shards:      make([]chan model.Event, opts.IngestWorkers),
		analysisCh:  make(chan string, opts.QueueSize),
		stopJanitor: make(chan struct{}),
	}

	for i := range p.shards {
		p.shards[i] = make(chan model.Event, opts.QueueSize)
		p.ingestWG.Add(1)
		go p.ingestWorker(p.shards[i])
	}
	for i := 0; i < opts.AnalysisWorkers; i++ {
		p.analysisWG.Add(1)
		go p.analysisWorker()
	}
	if opts.InactivityTimeout > 0 {
		p.janitorWG.Add(1)
		go p.janitor(opts.InactivityTimeout)
	}
	return p
}

// Ingest recibe un evento en JSON crudo: lo decodifica, valida, deduplica y lo encola.
// Devuelve nil si fue aceptado. El procesamiento posterior es asincrono.
func (p *Pipeline) Ingest(raw []byte) *IngestError {
	var ev model.Event
	if err := json.Unmarshal(raw, &ev); err != nil {
		return p.reject(&ev, validator.CodeMalformedJSON, fmt.Sprintf("invalid JSON: %v", err))
	}
	if verr := p.v.Validate(&ev); verr != nil {
		return p.reject(&ev, verr.Code, verr.Message)
	}
	if !p.st.RegisterEventID(ev.EventID) {
		return p.reject(&ev, validator.CodeDuplicateEvent, fmt.Sprintf("eventId %q was already received", ev.EventID))
	}
	if code := p.enqueue(ev); code != "" {
		p.st.ForgetEventID(ev.EventID) // permite reintentar el mismo evento
		return p.reject(&ev, code, "event could not be queued, retry later")
	}
	return nil
}

// enqueue envia el evento a la cola de su worker sin bloquear. Devuelve "" si tuvo exito.
func (p *Pipeline) enqueue(ev model.Event) string {
	p.closeMu.RLock()
	defer p.closeMu.RUnlock()
	if p.closed {
		return CodeShuttingDown
	}
	select {
	case p.shards[shardFor(ev.MatchID, len(p.shards))] <- ev:
		return ""
	default:
		return CodeQueueFull
	}
}

func shardFor(matchID string, n int) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(matchID))
	return int(h.Sum32() % uint32(n))
}

func (p *Pipeline) reject(ev *model.Event, code, msg string) *IngestError {
	p.rej.Add(store.Rejection{
		At:      time.Now().UTC(),
		EventID: ev.EventID,
		MatchID: ev.MatchID,
		Type:    ev.Type,
		Code:    code,
		Message: msg,
	})
	p.log.Printf("rejected event id=%q match=%q code=%s: %s", ev.EventID, ev.MatchID, code, msg)
	return &IngestError{Code: code, Message: msg, EventID: ev.EventID}
}

func (p *Pipeline) ingestWorker(ch <-chan model.Event) {
	defer p.ingestWG.Done()
	for ev := range ch {
		p.process(ev)
	}
}

func (p *Pipeline) process(ev model.Event) {
	spec, _ := p.v.TypeSpec(ev.Type)
	finished, err := p.st.AddEvent(ev, spec.StartsMatch, spec.EndsMatch)
	if err != nil {
		p.reject(&ev, validator.CodeMatchAlreadyFinished,
			fmt.Sprintf("match %q already finished, event ignored", ev.MatchID))
		return
	}
	if finished {
		p.analysisCh <- ev.MatchID
	}
}

func (p *Pipeline) analysisWorker() {
	defer p.analysisWG.Done()
	for matchID := range p.analysisCh {
		p.analyze(matchID)
	}
}

// BuildRequest arma el cuerpo que se enviaria a Scala para una partida.
func (p *Pipeline) BuildRequest(matchID string) (analysis.Request, bool) {
	snap, ok := p.st.Snapshot(matchID)
	if !ok {
		return analysis.Request{}, false
	}
	return analysis.Request{
		MatchID:     snap.MatchID,
		GameID:      snap.GameID,
		GameVersion: snap.GameVersion,
		SentAt:      time.Now().UTC().Format(time.RFC3339),
		Complete:    snap.Complete,
		Events:      snap.Events,
	}, true
}

func (p *Pipeline) analyze(matchID string) {
	req, ok := p.BuildRequest(matchID)
	if !ok {
		return
	}
	if p.client == nil {
		p.st.SetAnalysis(matchID, store.StatusAnalysisSkipped, nil, "SCALA_URL is not configured")
		p.log.Printf("match %q finished (%d events); analysis skipped, SCALA_URL not set", matchID, len(req.Events))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.opts.AnalysisTimeout)
	defer cancel()

	result, err := p.client.Analyze(ctx, req)
	if err != nil {
		p.st.SetAnalysis(matchID, store.StatusAnalysisFailed, nil, err.Error())
		p.log.Printf("analysis failed for match %q: %v", matchID, err)
		return
	}
	p.st.SetAnalysis(matchID, store.StatusAnalyzed, result, "")
	p.log.Printf("match %q analyzed by Scala (%d events)", matchID, len(req.Events))
}

// janitor cierra como incompletas las partidas sin actividad.
func (p *Pipeline) janitor(timeout time.Duration) {
	defer p.janitorWG.Done()
	interval := timeout / 2
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-p.stopJanitor:
			return
		case <-ticker.C:
			for _, id := range p.st.ExpireIdle(timeout) {
				p.log.Printf("match %q closed by inactivity (incomplete)", id)
				p.analysisCh <- id
			}
		}
	}
}

// Shutdown deja de aceptar eventos, procesa lo que ya esta en cola y espera a que
// terminen los envios a Scala. Es seguro llamarlo mas de una vez.
func (p *Pipeline) Shutdown(ctx context.Context) error {
	p.closeMu.Lock()
	if p.closed {
		p.closeMu.Unlock()
		return nil
	}
	p.closed = true
	close(p.stopJanitor)
	p.closeMu.Unlock()

	done := make(chan struct{})
	go func() {
		p.janitorWG.Wait()
		for _, ch := range p.shards {
			close(ch)
		}
		p.ingestWG.Wait()
		close(p.analysisCh)
		p.analysisWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
