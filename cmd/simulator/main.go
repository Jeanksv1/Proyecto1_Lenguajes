// Comando simulator: genera eventos de varias partidas en paralelo y los envia al modulo Go.
//
// Cada partida se envia desde su propia goroutine (en orden, MATCH_FINISHED al final).
// Al terminar compara la respuesta de cada evento con la esperada y muestra un resumen.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"gamestats/internal/simulator"
)

func main() {
	url := flag.String("url", "http://localhost:8080/events", "URL de POST /events del modulo Go")
	numFPS := flag.Int("matches", 4, "partidas de FPS (escenarios: streak, boundary, normal, long-streak)")
	numRace := flag.Int("race", 1, "partidas de carreras (segundo juego); 0 para desactivar")
	players := flag.Int("players", 3, "jugadores por partida FPS (minimo 2)")
	invalid := flag.Int("invalid", 9, "eventos invalidos a inyectar")
	seed := flag.Int64("seed", 42, "semilla: la misma semilla genera los mismos eventos")
	prefix := flag.String("prefix", "", "prefijo de matchId/eventId (default: sim-MMDD-HHMM-)")
	delay := flag.Duration("delay", 20*time.Millisecond, "pausa entre eventos de una misma partida")
	out := flag.String("out", "", "si se indica, guarda los eventos en este archivo JSON")
	send := flag.Bool("send", true, "enviar los eventos al servidor (false = solo generar)")
	flag.Parse()

	if *prefix == "" {
		*prefix = "sim-" + time.Now().Format("0102-1504") + "-"
	}

	matches := simulator.Generate(simulator.Config{
		Seed:        *seed,
		Prefix:      *prefix,
		Matches:     *numFPS,
		RaceMatches: *numRace,
		Players:     *players,
		Invalid:     *invalid,
	})
	printSummary(matches)

	if *out != "" {
		if err := writeEvents(*out, matches); err != nil {
			fmt.Fprintln(os.Stderr, "error guardando archivo:", err)
			os.Exit(1)
		}
		fmt.Printf("Eventos guardados en %s\n", *out)
	}
	if !*send {
		return
	}

	fmt.Printf("\nEnviando a %s ...\n", *url)
	res := sendAll(*url, matches, *delay)
	fmt.Printf("Enviados: %d | respuesta esperada: %d | inesperadas: %d\n", res.sent, res.matched, len(res.problems))
	if len(res.problems) > 0 {
		for _, p := range res.problems {
			fmt.Println("  !", p)
		}
		fmt.Println("Revisa que el servidor Go este corriendo y que no se haya usado antes el mismo prefijo.")
		os.Exit(1)
	}
	fmt.Println("OK. Revisa el resultado con:  curl localhost:8080/matches  y  curl localhost:8080/rejected")
}

func printSummary(ms []simulator.Match) {
	total, bad := 0, 0
	fmt.Println("Partidas generadas:")
	for _, m := range ms {
		valid, inv := 0, 0
		for _, it := range m.Items {
			if it.Expect == "" {
				valid++
			} else {
				inv++
			}
		}
		total += valid + inv
		bad += inv
		fmt.Printf("  %-26s %-10s %-12s validos=%-3d invalidos=%d\n", m.ID, m.GameID, m.Scenario, valid, inv)
		for _, s := range m.Streaks {
			fmt.Printf("      racha esperada: %s con %d eliminaciones (%s -> %s)\n",
				s.PlayerID, s.KillCount, s.Start.Format("15:04:05.000"), s.End.Format("15:04:05.000"))
		}
	}
	fmt.Printf("Total: %d eventos (%d validos, %d invalidos)\n", total, total-bad, bad)
}

func bodyOf(it simulator.Item) []byte {
	if it.Raw != "" {
		return []byte(it.Raw)
	}
	b, _ := json.Marshal(it.Event)
	return b
}

// writeEvents guarda los eventos estructurados (los cuerpos crudos mal formados se omiten).
// El archivo se puede enviar de una vez con: curl -X POST localhost:8080/events -d @archivo.json
func writeEvents(path string, ms []simulator.Match) error {
	var events []map[string]any
	for _, m := range ms {
		for _, it := range m.Items {
			if it.Event != nil {
				events = append(events, it.Event)
			}
		}
	}
	data, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

type outcome struct {
	sent     int
	matched  int
	problems []string
}

func sendAll(url string, ms []simulator.Match, delay time.Duration) outcome {
	client := &http.Client{Timeout: 10 * time.Second}
	var mu sync.Mutex
	var res outcome
	var wg sync.WaitGroup

	for _, m := range ms {
		wg.Add(1)
		go func(m simulator.Match) {
			defer wg.Done()
			for i, it := range m.Items {
				status, code, err := post(client, url, bodyOf(it))
				mu.Lock()
				res.sent++
				if err != nil {
					res.problems = append(res.problems, fmt.Sprintf("%s #%d: no se pudo enviar: %v", m.ID, i+1, err))
					mu.Unlock()
					return
				}
				if isExpected(it.Expect, status, code) {
					res.matched++
				} else {
					res.problems = append(res.problems, fmt.Sprintf(
						"%s #%d: se esperaba %q y el servidor respondio HTTP %d %q", m.ID, i+1, expectLabel(it.Expect), status, code))
				}
				mu.Unlock()
				time.Sleep(delay)
			}
		}(m)
	}
	wg.Wait()
	return res
}

func expectLabel(expect string) string {
	if expect == "" {
		return "aceptado (202)"
	}
	return expect
}

func isExpected(expect string, status int, code string) bool {
	if expect == "" {
		return status == http.StatusAccepted
	}
	return code == expect
}

func post(c *http.Client, url string, body []byte) (int, string, error) {
	resp, err := c.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var parsed struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(data, &parsed)
	return resp.StatusCode, parsed.Error, nil
}
