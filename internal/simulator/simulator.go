// Package simulator genera eventos sinteticos pero coherentes de varias partidas
// (juegos "fps-arena" y "race-x"), incluyendo eventos invalidos con su error esperado.
//
// La generacion es deterministica: la misma Seed produce los mismos eventos.
package simulator

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"
)

// Identificadores de los juegos simulados.
const (
	FPSGameID   = "fps-arena"
	RaceGameID  = "race-x"
	fpsVersion  = "1.0.0"
	raceVersion = "2.1.0"
	tsLayout    = "2006-01-02T15:04:05.000Z"
)

// Codigos de error esperados (los mismos que devuelve el modulo Go).
const (
	errMalformed = "MALFORMED_JSON"
	errMissing   = "MISSING_FIELD"
	errTimestamp = "INVALID_TIMESTAMP"
	errType      = "UNKNOWN_TYPE"
	errAction    = "UNKNOWN_ACTION"
	errValue     = "INVALID_FIELD_VALUE"
	errDuplicate = "DUPLICATE_EVENT"
)

// Config controla que se genera.
type Config struct {
	Seed        int64
	Prefix      string    // prefijo de matchId/eventId (default "sim-")
	Start       time.Time // inicio de la primera partida (default: hace 1 hora)
	Matches     int       // partidas de FPS
	RaceMatches int       // partidas de carreras
	Players     int       // jugadores por partida FPS (minimo 2, default 3)
	Invalid     int       // eventos invalidos a inyectar (se reparten entre las partidas FPS)
}

// Item es un envio al servidor: un evento estructurado o un cuerpo crudo.
type Item struct {
	Event  map[string]any // evento estructurado (nil si Raw != "")
	Raw    string         // cuerpo crudo, solo para MALFORMED_JSON
	Expect string         // "" = valido; si no, el codigo de error esperado
}

// Streak es una racha de eliminaciones que Scala debe detectar en la partida.
type Streak struct {
	PlayerID  string
	KillCount int
	Start     time.Time
	End       time.Time
}

// Match es una partida completa, con sus items en el orden en que deben enviarse
// (MATCH_FINISHED / RACE_FINISHED siempre al final).
type Match struct {
	ID       string
	GameID   string
	Scenario string
	Items    []Item
	Streaks  []Streak
}

var (
	fpsScenarios = []string{"streak", "boundary", "normal", "long-streak"}
	mapNames     = []string{"dust-valley", "neon-city", "iron-arena"}
	trackNames   = []string{"monaco", "suzuka", "interlagos"}
	weapons      = []string{"rifle", "pistol", "shotgun", "sniper"}
	itemNames    = []string{"medkit", "armor", "ammo"}
	nicknames    = []string{"Shadow", "Viper", "Blaze", "Frost", "Ghost", "Nova", "Raven", "Titan"}
)

// Generate crea las partidas segun la configuracion.
func Generate(cfg Config) []Match {
	if cfg.Players < 2 {
		cfg.Players = 3
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "sim-"
	}
	if cfg.Start.IsZero() {
		cfg.Start = time.Now().Add(-time.Hour)
	}
	cfg.Start = cfg.Start.UTC().Truncate(time.Second)

	r := rand.New(rand.NewSource(cfg.Seed))
	var out []Match
	for i := 0; i < cfg.Matches; i++ {
		out = append(out, genFPS(r, cfg, i))
	}
	for i := 0; i < cfg.RaceMatches; i++ {
		out = append(out, genRace(r, cfg, i))
	}
	injectInvalid(r, cfg, out)
	return out
}

// ---------------------------------------------------------------- constructor de eventos

type timedEvent struct {
	at time.Duration
	ev map[string]any
}

type builder struct {
	matchID string
	gameID  string
	version string
	start   time.Time
	events  []timedEvent
}

func (b *builder) add(at time.Duration, player, typ, action string, data map[string]any) {
	ev := map[string]any{
		"timestamp":   b.start.Add(at).Format(tsLayout),
		"gameId":      b.gameID,
		"gameVersion": b.version,
		"matchId":     b.matchID,
		"type":        typ,
		"data":        data,
	}
	if player == "" {
		ev["playerId"] = nil
	} else {
		ev["playerId"] = player
	}
	if action != "" {
		ev["action"] = action
	}
	b.events = append(b.events, timedEvent{at: at, ev: ev})
}

// items ordena por tiempo y asigna eventIds secuenciales.
func (b *builder) items() []Item {
	sort.SliceStable(b.events, func(i, j int) bool { return b.events[i].at < b.events[j].at })
	out := make([]Item, len(b.events))
	for i, te := range b.events {
		te.ev["eventId"] = fmt.Sprintf("%s-%03d", b.matchID, i+1)
		out[i] = Item{Event: te.ev}
	}
	return out
}

func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }

func pick(r *rand.Rand, list []string) string { return list[r.Intn(len(list))] }

func otherPlayer(r *rand.Rand, players []string, not string) string {
	for {
		p := players[r.Intn(len(players))]
		if p != not {
			return p
		}
	}
}

// ---------------------------------------------------------------- partidas FPS

type kill struct {
	at     time.Duration
	killer string
	victim string
	weapon string
}

func newKill(r *rand.Rand, at time.Duration, killer string, players []string) kill {
	return kill{at: at, killer: killer, victim: otherPlayer(r, players, killer), weapon: pick(r, weapons)}
}

// blockKills: k eliminaciones del mismo jugador con una duracion total < 30 s (racha).
func blockKills(r *rand.Rand, killer string, players []string, k int, duration time.Duration) []kill {
	startMs := 12000 + r.Intn(int(duration/time.Millisecond)-35000-12000+1)
	at := ms(startMs)
	maxGap := 25000 / (k - 1)
	ks := []kill{newKill(r, at, killer, players)}
	for i := 1; i < k; i++ {
		at += ms(3000 + r.Intn(maxGap-3000+1))
		ks = append(ks, newKill(r, at, killer, players))
	}
	return ks
}

// boundaryKills: 3 eliminaciones separadas exactamente 15 s. La 3ra ocurre a 30 s de la 1ra,
// y la regla pide MENOS de 30 s, asi que NO es racha (caso limite).
func boundaryKills(r *rand.Rand, killer string, players []string, duration time.Duration) []kill {
	startMs := 12000 + r.Intn(int(duration/time.Millisecond)-40000-12000+1)
	var ks []kill
	for i := 0; i < 3; i++ {
		ks = append(ks, newKill(r, ms(startMs+15000*i), killer, players))
	}
	return ks
}

// spacedKills: eliminaciones separadas >= 16 s, asi que nunca forman racha (2 gaps >= 32 s).
func spacedKills(r *rand.Rand, killer string, players []string, duration time.Duration) []kill {
	var ks []kill
	t := ms(10000 + r.Intn(10000))
	for t < duration-5*time.Second {
		ks = append(ks, newKill(r, t, killer, players))
		t += ms(16000 + r.Intn(20000))
	}
	return ks
}

func genFPS(r *rand.Rand, cfg Config, idx int) Match {
	matchID := fmt.Sprintf("%sfps-%02d", cfg.Prefix, idx+1)
	scenario := fpsScenarios[idx%len(fpsScenarios)]
	start := cfg.Start.Add(time.Duration(idx) * 3 * time.Minute)
	duration := time.Duration(70+r.Intn(50)) * time.Second // 70..119 s

	b := &builder{matchID: matchID, gameID: FPSGameID, version: fpsVersion, start: start}
	players := make([]string, cfg.Players)
	for i := range players {
		players[i] = fmt.Sprintf("p%d", i+1)
	}

	b.add(0, "", "MATCH_STARTED", "", map[string]any{
		"map": pick(r, mapNames), "mode": "deathmatch", "maxPlayers": cfg.Players + 2,
	})
	for i, p := range players {
		team := "red"
		if i%2 == 1 {
			team = "blue"
		}
		b.add(ms(1000+500*i), p, "PLAYER_JOINED", "", map[string]any{
			"nickname": nicknames[i%len(nicknames)], "team": team,
		})
	}

	// Eliminaciones segun el escenario. El jugador p1 es el "especial".
	special := players[0]
	var kills []kill
	var streaks []Streak
	switch scenario {
	case "streak":
		ks := blockKills(r, special, players, 3, duration)
		kills = append(kills, ks...)
		streaks = append(streaks, mkStreak(start, ks))
	case "long-streak":
		ks := blockKills(r, special, players, 5, duration)
		kills = append(kills, ks...)
		streaks = append(streaks, mkStreak(start, ks))
	case "boundary":
		kills = append(kills, boundaryKills(r, special, players, duration)...)
	}
	for _, p := range players {
		if scenario != "normal" && p == special {
			continue // el jugador especial solo tiene sus eliminaciones del escenario
		}
		kills = append(kills, spacedKills(r, p, players, duration)...)
	}
	for _, k := range kills {
		b.add(k.at-400*time.Millisecond, k.killer, "PLAYER_ACTION", "SHOOT", map[string]any{
			"weapon": k.weapon, "hit": true, "damage": 100, "targetId": k.victim,
		})
		b.add(k.at, k.killer, "PLAYER_ELIMINATED", "", map[string]any{
			"victimId": k.victim, "weapon": k.weapon,
			"distance": math.Round((1+r.Float64()*79)*10) / 10,
			"headshot": r.Intn(100) < 30,
		})
	}

	// Disparos extra (aciertos y fallos) y una accion de variedad por jugador.
	spanMs := int(duration/time.Millisecond) - 8000
	for _, p := range players {
		for i, n := 0, 2+r.Intn(3); i < n; i++ {
			at := ms(5000 + r.Intn(spanMs))
			w := pick(r, weapons)
			if r.Intn(100) < 40 {
				b.add(at, p, "PLAYER_ACTION", "SHOOT", map[string]any{
					"weapon": w, "hit": true, "damage": 10 + 10*r.Intn(5),
					"targetId": otherPlayer(r, players, p),
				})
			} else {
				b.add(at, p, "PLAYER_ACTION", "SHOOT", map[string]any{"weapon": w, "hit": false, "damage": 0})
			}
		}
		at := ms(5000 + r.Intn(spanMs))
		switch r.Intn(3) {
		case 0:
			b.add(at, p, "PLAYER_ACTION", "RELOAD", map[string]any{"weapon": pick(r, weapons)})
		case 1:
			b.add(at, p, "PLAYER_ACTION", "MOVE", map[string]any{
				"x": math.Round(r.Float64()*1000) / 10, "y": math.Round(r.Float64()*1000) / 10,
			})
		default:
			b.add(at, p, "PLAYER_ACTION", "PICKUP_ITEM", map[string]any{"item": pick(r, itemNames)})
		}
	}

	b.add(duration, "", "MATCH_FINISHED", "", map[string]any{"reason": "TIME_LIMIT"})

	return Match{ID: matchID, GameID: FPSGameID, Scenario: scenario, Items: b.items(), Streaks: streaks}
}

func mkStreak(start time.Time, ks []kill) Streak {
	return Streak{
		PlayerID:  ks[0].killer,
		KillCount: len(ks),
		Start:     start.Add(ks[0].at),
		End:       start.Add(ks[len(ks)-1].at),
	}
}

// ---------------------------------------------------------------- partidas de carreras

func genRace(r *rand.Rand, cfg Config, idx int) Match {
	matchID := fmt.Sprintf("%srace-%02d", cfg.Prefix, idx+1)
	start := cfg.Start.Add(time.Duration(idx)*3*time.Minute + 90*time.Second)
	const laps = 3
	drivers := []string{"d1", "d2", "d3"}

	b := &builder{matchID: matchID, gameID: RaceGameID, version: raceVersion, start: start}
	b.add(0, "", "RACE_STARTED", "", map[string]any{"track": pick(r, trackNames), "laps": laps})
	for i, d := range drivers {
		b.add(ms(1000+500*i), d, "PLAYER_JOINED", "", map[string]any{"nickname": nicknames[(i+3)%len(nicknames)]})
	}

	lapTime := make([][]int, len(drivers))
	cum := make([][]int, len(drivers))
	for d := range drivers {
		lapTime[d] = make([]int, laps)
		cum[d] = make([]int, laps)
		total := 0
		for l := 0; l < laps; l++ {
			lt := 80000 + r.Intn(15000)
			total += lt
			lapTime[d][l] = lt
			cum[d][l] = total
		}
	}

	maxTotal := 0
	for d := range drivers {
		for l := 0; l < laps; l++ {
			pos := 1
			for e := range drivers {
				if e != d && (cum[e][l] < cum[d][l] || (cum[e][l] == cum[d][l] && e < d)) {
					pos++
				}
			}
			b.add(ms(5000+cum[d][l]), drivers[d], "LAP_COMPLETED", "", map[string]any{
				"lap": l + 1, "lapTimeMs": lapTime[d][l], "position": pos,
			})
		}
		if cum[d][laps-1] > maxTotal {
			maxTotal = cum[d][laps-1]
		}
	}
	b.add(ms(5000+maxTotal+7000), "", "RACE_FINISHED", "", map[string]any{"reason": "COMPLETED"})

	return Match{ID: matchID, GameID: RaceGameID, Scenario: "race", Items: b.items()}
}

// ---------------------------------------------------------------- eventos invalidos

func cloneEvent(ev map[string]any) map[string]any {
	c := make(map[string]any, len(ev))
	for k, v := range ev {
		c[k] = v
	}
	if d, ok := ev["data"].(map[string]any); ok {
		dc := make(map[string]any, len(d))
		for k, v := range d {
			dc[k] = v
		}
		c["data"] = dc
	}
	return c
}

// findBase devuelve una copia del primer evento valido del tipo (y accion) indicados.
func findBase(items []Item, typ, action string) (map[string]any, bool) {
	for _, it := range items {
		if it.Expect != "" || it.Event == nil || it.Event["type"] != typ {
			continue
		}
		if action != "" && it.Event["action"] != action {
			continue
		}
		return cloneEvent(it.Event), true
	}
	return nil, false
}

const invalidKinds = 9

// makeInvalid fabrica el n-esimo evento invalido de una partida FPS.
func makeInvalid(m *Match, kind, n int) (Item, bool) {
	badID := fmt.Sprintf("%s-bad-%02d", m.ID, n+1)
	switch kind {
	case 0: // falta matchId
		ev, ok := findBase(m.Items, "PLAYER_JOINED", "")
		if !ok {
			return Item{}, false
		}
		ev["eventId"] = badID
		delete(ev, "matchId")
		return Item{Event: ev, Expect: errMissing}, true
	case 1: // timestamp invalido
		ev, ok := findBase(m.Items, "PLAYER_JOINED", "")
		if !ok {
			return Item{}, false
		}
		ev["eventId"] = badID
		ev["timestamp"] = "ayer a las 10"
		return Item{Event: ev, Expect: errTimestamp}, true
	case 2: // tipo desconocido
		ev, ok := findBase(m.Items, "PLAYER_JOINED", "")
		if !ok {
			return Item{}, false
		}
		ev["eventId"] = badID
		ev["type"] = "PLAYER_TELEPORTED"
		return Item{Event: ev, Expect: errType}, true
	case 3: // accion desconocida
		ev, ok := findBase(m.Items, "PLAYER_ACTION", "SHOOT")
		if !ok {
			return Item{}, false
		}
		ev["eventId"] = badID
		ev["action"] = "FLY"
		return Item{Event: ev, Expect: errAction}, true
	case 4: // un jugador se elimina a si mismo
		ev, ok := findBase(m.Items, "PLAYER_ELIMINATED", "")
		if !ok {
			return Item{}, false
		}
		ev["eventId"] = badID
		ev["data"].(map[string]any)["victimId"] = ev["playerId"]
		return Item{Event: ev, Expect: errValue}, true
	case 5: // dano negativo
		ev, ok := findBase(m.Items, "PLAYER_ACTION", "SHOOT")
		if !ok {
			return Item{}, false
		}
		ev["eventId"] = badID
		d := ev["data"].(map[string]any)
		d["hit"] = true
		d["damage"] = -20
		return Item{Event: ev, Expect: errValue}, true
	case 6: // PLAYER_JOINED sin playerId
		ev, ok := findBase(m.Items, "PLAYER_JOINED", "")
		if !ok {
			return Item{}, false
		}
		ev["eventId"] = badID
		ev["playerId"] = nil
		return Item{Event: ev, Expect: errMissing}, true
	case 7: // evento duplicado (mismo eventId que MATCH_STARTED)
		ev, ok := findBase(m.Items, "MATCH_STARTED", "")
		if !ok {
			return Item{}, false
		}
		return Item{Event: ev, Expect: errDuplicate}, true
	default: // JSON mal formado
		return Item{Raw: `{"eventId": "broken-json", "type": `, Expect: errMalformed}, true
	}
}

// injectInvalid reparte cfg.Invalid eventos invalidos entre las partidas FPS,
// siempre despues de MATCH_STARTED y antes de MATCH_FINISHED.
func injectInvalid(r *rand.Rand, cfg Config, matches []Match) {
	var fps []int
	for i := range matches {
		if matches[i].GameID == FPSGameID {
			fps = append(fps, i)
		}
	}
	if len(fps) == 0 {
		return
	}
	for n := 0; n < cfg.Invalid; n++ {
		m := &matches[fps[n%len(fps)]]
		item, ok := makeInvalid(m, n%invalidKinds, n)
		if !ok {
			continue
		}
		pos := 1 + r.Intn(len(m.Items)-1) // [1, len-1]: nunca antes del inicio ni despues del fin
		m.Items = append(m.Items, Item{})
		copy(m.Items[pos+1:], m.Items[pos:])
		m.Items[pos] = item
	}
}
