// Package analysis es el cliente HTTP que envia partidas al modulo Scala.
package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gamestats/internal/model"
)

// Request es el cuerpo de POST /analyze (ver goToScalaContract en events_spec.json).
// "complete" es un campo adicional: false si la partida se cerro por inactividad
// o nunca recibio MATCH_STARTED.
type Request struct {
	MatchID     string        `json:"matchId"`
	GameID      string        `json:"gameId"`
	GameVersion string        `json:"gameVersion"`
	SentAt      string        `json:"sentAt"`
	Complete    bool          `json:"complete"`
	Events      []model.Event `json:"events"`
}

// Client envia partidas a Scala con reintentos.
type Client struct {
	baseURL string
	http    *http.Client
	retries int
	backoff time.Duration
}

// NewClient crea un cliente para el servicio Scala en baseURL (ej. http://scala:9090).
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{},
		retries: 3,
		backoff: 500 * time.Millisecond,
	}
}

// Analyze envia la partida y devuelve el JSON de resultados de Scala.
// Reintenta ante errores de red y respuestas 5xx; no reintenta ante 4xx.
func (c *Client) Analyze(ctx context.Context, req Request) (json.RawMessage, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encoding request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(c.backoff * time.Duration(attempt)):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		result, retry, err := c.post(ctx, body)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !retry {
			break
		}
	}
	return nil, lastErr
}

func (c *Client) post(ctx context.Context, body []byte) (json.RawMessage, bool, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/analyze", bytes.NewReader(body))
	if err != nil {
		return nil, false, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, true, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, true, err
	}
	if resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("scala returned %d: %s", resp.StatusCode, snippet(data))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false, fmt.Errorf("scala returned %d: %s", resp.StatusCode, snippet(data))
	}
	if !json.Valid(data) {
		return nil, false, errors.New("scala returned a body that is not valid JSON")
	}
	return json.RawMessage(data), false, nil
}

func snippet(b []byte) string {
	const max = 200
	s := strings.TrimSpace(string(b))
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
