// Package api expone el pipeline por HTTP.
package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"gamestats/internal/pipeline"
	"gamestats/internal/store"
)

// Server agrupa las dependencias de los handlers.
type Server struct {
	p       *pipeline.Pipeline
	st      *store.Store
	rej     *store.RejectedLog
	maxBody int64
}

// New crea el servidor HTTP. maxBody es el tamano maximo de cada peticion en bytes.
func New(p *pipeline.Pipeline, st *store.Store, rej *store.RejectedLog, maxBody int64) *Server {
	return &Server{p: p, st: st, rej: rej, maxBody: maxBody}
}

// Routes registra las rutas (requiere Go 1.22+ por los patrones con metodo y {id}).
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /events", s.postEvents)
	mux.HandleFunc("GET /matches", s.listMatches)
	mux.HandleFunc("GET /matches/{id}", s.getMatch)
	mux.HandleFunc("GET /matches/{id}/results", s.getResults)
	mux.HandleFunc("GET /matches/{id}/payload", s.getPayload)
	mux.HandleFunc("GET /rejected", s.listRejected)
	mux.HandleFunc("GET /health", s.health)
	return mux
}

type batchRejection struct {
	Index   int    `json:"index"`
	EventID string `json:"eventId,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// POST /events acepta un evento (objeto JSON) o varios (arreglo JSON).
func (s *Server) postEvents(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.maxBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "request body is too large")
			return
		}
		writeError(w, http.StatusBadRequest, "MALFORMED_JSON", err.Error())
		return
	}
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		writeError(w, http.StatusBadRequest, "MALFORMED_JSON", "empty body")
		return
	}

	if body[0] == '[' {
		s.postBatch(w, body)
		return
	}

	if ierr := s.p.Ingest(body); ierr != nil {
		writeJSON(w, ierr.HTTPStatus(), map[string]string{
			"error":   ierr.Code,
			"message": ierr.Message,
			"eventId": ierr.EventID,
		})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (s *Server) postBatch(w http.ResponseWriter, body []byte) {
	var items []json.RawMessage
	if err := json.Unmarshal(body, &items); err != nil {
		writeError(w, http.StatusBadRequest, "MALFORMED_JSON", err.Error())
		return
	}

	accepted := 0
	rejected := []batchRejection{}
	firstStatus := 0
	for i, item := range items {
		if ierr := s.p.Ingest(item); ierr != nil {
			rejected = append(rejected, batchRejection{Index: i, EventID: ierr.EventID, Code: ierr.Code, Message: ierr.Message})
			if firstStatus == 0 {
				firstStatus = ierr.HTTPStatus()
			}
			continue
		}
		accepted++
	}

	status := http.StatusAccepted
	if accepted == 0 && len(items) > 0 {
		status = firstStatus
	}
	writeJSON(w, status, map[string]any{
		"received": len(items),
		"accepted": accepted,
		"rejected": rejected,
	})
}

func (s *Server) listMatches(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"matches": s.st.List()})
}

func (s *Server) getMatch(w http.ResponseWriter, r *http.Request) {
	sum, ok := s.st.Summary(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "MATCH_NOT_FOUND", "unknown matchId")
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

// GET /matches/{id}/results devuelve el JSON que respondio Scala.
func (s *Server) getResults(w http.ResponseWriter, r *http.Request) {
	result, status, ok := s.st.Result(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "MATCH_NOT_FOUND", "unknown matchId")
		return
	}
	if result == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error":   "RESULT_NOT_AVAILABLE",
			"message": "no analysis result for this match yet",
			"status":  string(status),
		})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result)
}

// GET /matches/{id}/payload muestra exactamente lo que Go enviaria a Scala (util para depurar).
func (s *Server) getPayload(w http.ResponseWriter, r *http.Request) {
	req, ok := s.p.BuildRequest(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "MATCH_NOT_FOUND", "unknown matchId")
		return
	}
	writeJSON(w, http.StatusOK, req)
}

func (s *Server) listRejected(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"total":  s.rej.Total(),
		"recent": s.rej.List(),
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writing response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error": code, "message": msg})
}
