# GameStats — Módulo Go (ingestión y procesamiento)

Recibe eventos de partidas, los valida, los agrupa por `matchId` y, cuando una partida
termina, la envía ordenada por `timestamp` al módulo Scala (`POST /analyze`).
Go **no calcula estadísticas**: eso es responsabilidad de Scala.

Requiere **Go 1.22+**. Solo usa la librería estándar (sin dependencias externas).

## Ejecutar

```bash
go run ./cmd/server
# con Scala:
SCALA_URL=http://localhost:9090 go run ./cmd/server
```

| Variable | Default | Descripción |
|---|---|---|
| `PORT` | `8080` | Puerto HTTP |
| `EVENT_TYPES_PATH` | `config/event_types.json` | Definición de tipos de evento |
| `SCALA_URL` | *(vacío)* | URL base del módulo Scala. Vacío = no se envía (estado `analysis_skipped`) |
| `INGEST_WORKERS` | `4` | Goroutines de ingestión (una cola por worker) |
| `ANALYSIS_WORKERS` | `2` | Goroutines que envían a Scala |
| `QUEUE_SIZE` | `1024` | Tamaño de cada cola |
| `INACTIVITY_TIMEOUT` | `0` (off) | Ej. `2m`. Cierra partidas sin actividad y las envía con `complete:false` |

## Endpoints

| Método y ruta | Qué hace |
|---|---|
| `POST /events` | Recibe un evento (objeto) o varios (arreglo). `202` aceptado, `400` inválido, `409` duplicado, `503` cola llena |
| `GET /matches` | Lista de partidas y su estado |
| `GET /matches/{id}` | Resumen de una partida (jugadores, nº de eventos, estado) |
| `GET /matches/{id}/results` | JSON de resultados que devolvió Scala |
| `GET /matches/{id}/payload` | Lo que Go enviaría a Scala (útil para depurar sin Scala) |
| `GET /rejected` | Últimos eventos rechazados con su código de error |
| `GET /health` | Estado del servicio |

Prueba rápida:

```bash
curl -i -X POST localhost:8080/events -d '{
  "eventId":"e1","timestamp":"2026-09-01T10:00:00Z","gameId":"fps-arena",
  "gameVersion":"1.0.0","matchId":"m1","playerId":null,"type":"MATCH_STARTED",
  "data":{"map":"dust","mode":"deathmatch","maxPlayers":8}}'

curl -i -X POST localhost:8080/events -d '{"eventId":"e2"}'   # 400 MISSING_FIELD
curl localhost:8080/matches
curl localhost:8080/rejected
```

## Arquitectura

```
POST /events
   │  decodificar JSON → validar → deduplicar eventId        (síncrono, responde 400/409)
   ▼
hash(matchId) % N ──► cola del worker i ──► worker de ingestión i
                                                │  store.AddEvent (RWMutex, estado por partida)
                                                │  si el evento es MATCH_FINISHED
                                                ▼
                                       canal de análisis ──► worker de análisis ──► POST Scala /analyze
```

- **Concurrencia:** goroutines + channels. Una partida siempre cae en el mismo worker
  (hash de `matchId`), así sus eventos se procesan en orden; partidas distintas van en paralelo.
- **Estado compartido:** `sync.RWMutex` para las partidas y `sync.Map` para los `eventId` vistos.
- **Contrapresión:** si una cola está llena, `POST /events` responde `503` (el cliente reintenta).
- **Cierre ordenado (graceful shutdown):** deja de aceptar, vacía las colas y espera los envíos a Scala.
- **Genérico:** los tipos de evento y sus campos obligatorios están en `config/event_types.json`,
  no en el código. Para agregar un evento o juego nuevo basta editar ese archivo
  (`LAP_COMPLETED` de carreras está como ejemplo).

## Decisiones tomadas (las que el Día 1 dejó abiertas)

1. **Evento de una partida sin `MATCH_STARTED` previo:** se acepta y la partida queda `pending`.
2. **Evento después de `MATCH_FINISHED`:** se descarta y se registra como `MATCH_ALREADY_FINISHED` en `/rejected`.
3. **Cuándo se envía a Scala:** al procesar el evento marcado `endsMatch` (`MATCH_FINISHED`), o por inactividad si `INACTIVITY_TIMEOUT` está activo.
4. **Campo extra en el contrato:** el request a Scala incluye `"complete"` (bool). Es `false` si la partida se cerró por inactividad o no tuvo `MATCH_STARTED`. Avísale a tu compañero.
5. **Reintentos a Scala:** 3 reintentos con espera creciente ante errores de red o `5xx`; no reintenta ante `4xx`.

## Limitaciones conocidas

- El estado vive en memoria: al reiniciar se pierde.
- Si dos peticiones concurrentes de la **misma** partida llegan en desorden y `MATCH_FINISHED` se procesa antes que un evento anterior, ese evento se descarta. El simulador (Día 4) debe enviar `MATCH_FINISHED` al final de cada partida.
- Una partida que solo tenga eventos como `LAP_COMPLETED` (sin tipo de inicio/fin) nunca se cierra salvo por inactividad. Para otro juego, define sus eventos de inicio/fin con `startsMatch` / `endsMatch`.

## Pruebas

```bash
go vet ./...
go test -race ./...
```

- `internal/validator`: usa los ejemplos válidos e inválidos de `testdata/events_spec.json` (el entregable del Día 1) como casos de prueba.
- `internal/pipeline`: partida completa enviada a un Scala simulado y ordenada por timestamp, duplicados, eventos tras el fin, y 30 partidas en paralelo (usar `-race`).

## Estructura

```
cmd/server/main.go          arranque, variables de entorno, graceful shutdown
config/event_types.json     tipos de evento y reglas de validación
internal/model              struct Event
internal/validator          validación configurable
internal/store              estado por partida (RWMutex) + log de rechazados
internal/pipeline           workers, colas, envío a Scala
internal/analysis           cliente HTTP hacia Scala
internal/api                handlers HTTP
testdata/events_spec.json   entregable del Día 1
```

## Simulador de eventos (Día 4)

Genera eventos coherentes de varias partidas y los envía **en paralelo** (una goroutine por
partida) al módulo Go. Al terminar compara la respuesta de cada evento con la esperada.

```bash
# 1) Con el servidor Go corriendo en otra terminal:
go run ./cmd/simulator

# Solo generar un archivo, sin enviar:
go run ./cmd/simulator -send=false -out eventos_sim.json
```

Escenarios que genera (todos con `seed` fija, así que son reproducibles):

| Partida | Escenario | Qué debe detectar Scala |
|---|---|---|
| `fps-01` | `streak` | 1 racha de 3 eliminaciones (< 30 s) |
| `fps-02` | `boundary` | **Ninguna**: 3 eliminaciones a exactamente 15 s entre sí (la 3ª a 30 s de la 1ª; la regla pide *menos* de 30 s) |
| `fps-03` | `normal` | Ninguna: eliminaciones separadas ≥ 16 s |
| `fps-04` | `long-streak` | 1 racha de 5 eliminaciones |
| `race-01` | `race` | Partida de otro juego (`race-x`) con vueltas y posiciones |

Además inyecta 9 eventos inválidos (uno de cada tipo: falta `matchId`, timestamp inválido,
tipo y acción desconocidos, auto-eliminación, daño negativo, sin `playerId`, duplicado y JSON
mal formado). El simulador imprime las rachas esperadas para que se comparen con lo que calcule Scala.

| Flag | Default | Descripción |
|---|---|---|
| `-url` | `http://localhost:8080/events` | Dónde enviar |
| `-matches` | `4` | Partidas FPS (los escenarios rotan) |
| `-race` | `1` | Partidas de carreras (`0` = desactivar) |
| `-players` | `3` | Jugadores por partida FPS |
| `-invalid` | `9` | Eventos inválidos a inyectar |
| `-seed` | `42` | Misma semilla = mismos eventos |
| `-prefix` | `sim-MMDD-HHMM-` | Prefijo de `matchId`/`eventId` (evita duplicados al reejecutar) |
| `-delay` | `20ms` | Pausa entre eventos de una misma partida |
| `-out` | *(vacío)* | Guarda los eventos en un archivo JSON |
| `-send` | `true` | `false` = solo generar |

El archivo de `-out` también se puede enviar de una vez con
`curl -X POST localhost:8080/events -d @eventos_sim.json` (el endpoint acepta arreglos).

La prueba `internal/simulator` pasa todo lo generado por el módulo Go real: los válidos se aceptan,
cada inválido falla con el código esperado, y las rachas declaradas coinciden con la regla del Día 1.
