# GameStats

Plataforma multilenguaje para **recibir, procesar y analizar eventos de partidas de videojuegos**.
Proyecto del curso de Lenguajes de Programación.

- **Go (ingestión y procesamiento):** recibe eventos por HTTP, los valida, los deduplica, los agrupa por
  `matchId` con concurrencia (goroutines y channels) y, cuando una partida termina, la envía ordenada por
  `timestamp` al módulo Scala. Go **no calcula estadísticas**.
- **Scala (análisis):** transforma los eventos con colecciones funcionales (`filter`, `map`, `flatMap`,
  `groupBy`, `foldLeft`, `reduce`), calcula las métricas, detecta patrones y devuelve el resultado en JSON.

```
Simulador / fuente externa
        │  POST /events  (JSON: un evento o un arreglo)
        ▼
┌─────────────────────────┐   POST /analyze    ┌──────────────────────────┐
│  Módulo Go  (:8080)     │ ─────────────────► │  Módulo Scala  (:9090)   │
│  validar · deduplicar   │   partida completa │  métricas · patrones     │
│  agrupar · concurrencia │ ◄───────────────── │  resultado en JSON       │
└─────────────────────────┘   resultado JSON   └──────────────────────────┘
        │                                                  │
   GET /matches, /results                      output/<matchId>.json (opcional)
```

## Autores

- Jean Carlos Segura Vargas
- Leiner Josue Aguilar González

Proyecto 1 del curso de Lenguajes de Programación. Fecha de entrega: 18 de octubre de 2026.

## Inicio rápido con Docker (recomendado)

Requisitos: **Docker** y **Docker Compose**. No hace falta instalar Go, Scala ni Java.

```bash
docker compose up --build                              # levanta Scala (9090) y Go (8080)
docker compose --profile sim run --rm simulator        # envía eventos de prueba (en otra terminal)
curl localhost:8080/matches                            # estado de las partidas
docker compose down                                    # apagar
```

La primera vez, la compilación de Scala puede tardar varios minutos.
Cuando termina el simulador, las 5 partidas deben aparecer con `"status":"analyzed"`.
Los resultados de Scala también se guardan como `output/<matchId>.json`.

Para ver el resultado de una partida: `curl localhost:8080/matches/<matchId>/results`.

## Ejecución sin Docker

Requisitos: **Go 1.22+** (solo librería estándar) y, para Scala, **JDK 17 o 21** y **sbt**.

```bash
# Terminal 1: módulo Scala (puerto 9090)
cd scala
sbt run

# Terminal 2: módulo Go (puerto 8080)
SCALA_URL=http://localhost:9090 go run ./cmd/server
#   PowerShell:  $env:SCALA_URL="http://localhost:9090"; go run ./cmd/server

# Terminal 3: simulador de eventos
go run ./cmd/simulator
```

Sin `SCALA_URL`, Go funciona igual pero no envía a Scala (las partidas quedan en `analysis_skipped`).
Para probar solo Go existe `fake_scala.py`, un Scala falso que responde `{"ok": true}`.

## Variables de entorno

**Módulo Go**

| Variable | Default | Descripción |
|---|---|---|
| `PORT` | `8080` | Puerto HTTP |
| `EVENT_TYPES_PATH` | `config/event_types.json` | Definición de tipos de evento |
| `SCALA_URL` | *(vacío)* | URL base del módulo Scala. Vacío = no se envía (`analysis_skipped`) |
| `INGEST_WORKERS` | `4` | Goroutines de ingestión (una cola por worker) |
| `ANALYSIS_WORKERS` | `2` | Goroutines que envían a Scala |
| `QUEUE_SIZE` | `1024` | Tamaño de cada cola |
| `INACTIVITY_TIMEOUT` | `0` (off) | Ej. `2m`. Cierra partidas sin actividad y las envía con `complete:false` |

**Módulo Scala**

| Variable | Default | Descripción |
|---|---|---|
| `PORT` | `9090` | Puerto HTTP |
| `OUTPUT_DIR` | *(vacío)* | Si se define, guarda cada resultado como `<matchId>.json` en esa carpeta |

En `docker-compose.yml`, Scala usa `OUTPUT_DIR=/data/results`, montada en `./output`.

## Endpoints

**Go (`:8080`)**

| Método y ruta | Qué hace |
|---|---|
| `POST /events` | Recibe un evento (objeto) o varios (arreglo). `202` aceptado, `400` inválido, `409` duplicado, `503` cola llena |
| `GET /matches` | Lista de partidas y su estado |
| `GET /matches/{id}` | Resumen de una partida (jugadores, nº de eventos, estado) |
| `GET /matches/{id}/results` | JSON de resultados que devolvió Scala |
| `GET /matches/{id}/payload` | Lo que Go enviaría a Scala (útil para depurar) |
| `GET /rejected` | Últimos eventos rechazados con su código de error |
| `GET /health` | Estado del servicio |

**Scala (`:9090`)**

| Método y ruta | Qué hace |
|---|---|
| `POST /analyze` | Analiza una partida. `200` con el resultado, `400` si el payload es inválido, `500` en error interno |
| `GET /health` | Estado del servicio |

Prueba rápida:

```bash
curl -i -X POST localhost:8080/events -d '{
  "eventId":"e1","timestamp":"2026-09-01T10:00:00Z","gameId":"fps-arena",
  "gameVersion":"1.0.0","matchId":"m1","playerId":null,"type":"MATCH_STARTED",
  "data":{"map":"dust","mode":"deathmatch","maxPlayers":8}}'

curl -i -X POST localhost:8080/events -d '{"eventId":"e2"}'      # 400 MISSING_FIELD
curl -X POST localhost:8080/events -d @testdata/partida.json      # una partida completa
curl localhost:8080/matches
curl localhost:8080/rejected
```

## Formato de eventos

Todos los eventos comparten un **envelope** común, y el campo `data` es flexible según el tipo.
La especificación completa, con ejemplos válidos e inválidos, está en
[`testdata/events_spec.json`](testdata/events_spec.json).

| Campo | Descripción |
|---|---|
| `eventId` | Identificador único del evento (un repetido se rechaza como `DUPLICATE_EVENT`) |
| `timestamp` | RFC 3339 en UTC, con milisegundos (ej. `2026-09-01T10:00:12.000Z`) |
| `gameId`, `gameVersion` | Juego que produjo el evento |
| `matchId` | Partida a la que pertenece |
| `playerId` | Jugador asociado; `null` en eventos de inicio/fin de partida |
| `type` | Tipo de evento definido en `config/event_types.json` |
| `action` | Solo en `PLAYER_ACTION` (`SHOOT`, `RELOAD`, `MOVE`, `PICKUP_ITEM`) |
| `data` | Campos específicos del tipo o la acción |

**Juego inicial: `fps-arena` (FPS).** Eventos: `MATCH_STARTED`, `PLAYER_JOINED`, `PLAYER_ACTION`,
`PLAYER_ELIMINATED` (el `playerId` es quien elimina; la víctima va en `data.victimId`), `MATCH_FINISHED`.

**Segundo juego, para demostrar extensibilidad: `race-x` (carreras).** Eventos: `RACE_STARTED`,
`LAP_COMPLETED`, `RACE_FINISHED`.

**Códigos de error de validación:** `MALFORMED_JSON`, `MISSING_FIELD`, `INVALID_TIMESTAMP`, `UNKNOWN_TYPE`,
`UNKNOWN_ACTION`, `INVALID_FIELD_VALUE`, `DUPLICATE_EVENT` y `MATCH_ALREADY_FINISHED`.

Regla condicional: en `PLAYER_ACTION`/`SHOOT`, `data.targetId` es obligatorio cuando `hit=true`
(propiedad `requiredIf` en `config/event_types.json`), y `damage` debe ser `0` si `hit=false`.

### Agregar un evento o un juego nuevo

Los tipos de evento y sus campos obligatorios viven en `config/event_types.json`, no en el código.
Para sumar un evento o un juego basta editar ese archivo. Los eventos que inician o terminan una partida
se marcan con `startsMatch` / `endsMatch`.

## Contrato Go → Scala

`POST /analyze` con una partida completa y ordenada por `timestamp`:

```json
{
  "matchId": "match-001", "gameId": "fps-arena", "gameVersion": "1.0.0",
  "sentAt": "2026-09-01T10:01:01.000Z",
  "complete": true,
  "events": [ { "eventId": "e1", "timestamp": "...", "type": "MATCH_STARTED", "...": "..." } ]
}
```

`complete` es un campo adicional: es `false` si la partida se cerró por inactividad o nunca recibió su
evento de inicio. Respuesta de Scala:

```json
{
  "matchId": "match-001", "gameId": "fps-arena", "complete": true,
  "matchStats":  { "durationSeconds": 60.0, "totalKills": 3, "mvpPlayerId": "p1" },
  "playerStats": [ { "playerId": "p1", "kills": 3, "deaths": 0, "kdRatio": 3.0, "shots": 2,
                     "hits": 1, "accuracy": 0.5, "totalDamage": 100.0 } ],
  "patterns":    { "killStreaks": [ { "playerId": "p1", "killCount": 3,
                     "startTime": "2026-09-01T10:00:12.000Z", "endTime": "2026-09-01T10:00:33.000Z",
                     "durationMs": 21000 } ] },
  "raceStats":   []
}
```

## Qué calcula Scala

**Por jugador**

| Métrica | Definición |
|---|---|
| `kills` / `deaths` | `PLAYER_ELIMINATED` como killer / como víctima (`data.victimId`) |
| `kdRatio` | `kills / deaths`; si `deaths = 0` vale `kills` (2 decimales) |
| `shots` / `hits` | `PLAYER_ACTION` con `SHOOT` / los que tienen `hit = true` |
| `accuracy` | `hits / shots`; `0.0` si no hay disparos (4 decimales) |
| `totalDamage` | Suma de `data.damage` de sus `SHOOT` |

**Por partida**

| Métrica | Definición |
|---|---|
| `durationSeconds` | Fin menos inicio de la partida (`MATCH_*` o `RACE_*`) |
| `totalKills` | Total de `PLAYER_ELIMINATED` |
| `mvpPlayerId` | Más kills; desempates: mayor `totalDamage`, menos `deaths`, `playerId` alfabético. `null` si nadie tiene kills |

**Patrón de secuencia: `KILL_STREAK` (racha de eliminaciones).** Un jugador hace 3 o más eliminaciones tales
que la tercera ocurre en **menos de 30 s** después de la primera (`t[i+2] − t[i] < 30 000 ms`).
Las ventanas que se solapan se fusionan en una sola racha. Con exactamente 30 s **no** es racha.

**Carreras (`race-x`): `raceStats`.** Por piloto, a partir de `LAP_COMPLETED`: `lapsCompleted`,
`bestLapMs`, `avgLapMs` y `finalPosition` (la posición de su última vuelta). En partidas de otro tipo
queda vacío (`[]`).

## Arquitectura del módulo Go

- **Concurrencia:** goroutines y channels. Una partida siempre cae en el mismo worker (hash de `matchId`),
  así sus eventos se procesan en orden; partidas distintas se procesan en paralelo.
- **Estado compartido:** `sync.RWMutex` para las partidas y `sync.Map` para los `eventId` vistos.
- **Contrapresión:** si una cola está llena, `POST /events` responde `503` y el cliente reintenta.
- **Reintentos hacia Scala:** 3 reintentos con espera creciente ante errores de red o `5xx`; no reintenta ante `4xx`.
- **Cierre ordenado (graceful shutdown):** deja de aceptar eventos, vacía las colas y espera los envíos a Scala.
- **Genérico:** la validación se configura en `config/event_types.json`.

```
POST /events
   │  decodificar JSON → validar → deduplicar eventId        (síncrono, responde 400/409)
   ▼
hash(matchId) % N ──► cola del worker i ──► worker de ingestión i
                                                │  store.AddEvent (RWMutex, estado por partida)
                                                │  si el evento cierra la partida
                                                ▼
                                       canal de análisis ──► worker de análisis ──► POST Scala /analyze
```

## Decisiones de diseño

1. **Evento de una partida sin inicio previo:** se acepta y la partida queda `pending`.
2. **Evento después del fin de la partida:** se descarta y se registra como `MATCH_ALREADY_FINISHED` en `/rejected`.
3. **Cuándo se envía a Scala:** al procesar el evento marcado `endsMatch`, o por inactividad si `INACTIVITY_TIMEOUT` está activo.
4. **Campo extra en el contrato:** `complete` (bool) en el request a Scala.
5. **Scala no confía en el orden:** vuelve a ordenar los eventos por `timestamp` antes de calcular.

## Limitaciones conocidas

- El estado vive en memoria: al reiniciar Go se pierde.
- Si dos peticiones concurrentes de la **misma** partida llegan en desorden y el evento de fin se procesa antes
  que uno anterior, ese evento se descarta. El simulador envía el evento de fin al final de cada partida.
- Una partida cuyos eventos no incluyan tipos de inicio/fin nunca se cierra, salvo por inactividad.
  Para otro juego hay que definir esos eventos con `startsMatch` / `endsMatch`.
- Si Scala no responde, los workers de análisis esperan; con las colas llenas, Go responde `503`.
- La API no tiene autenticación: está pensada para ejecutarse de forma local.

## Simulador de eventos

Genera eventos coherentes de varias partidas y los envía **en paralelo** (una goroutine por partida) a Go.
Al terminar compara la respuesta de cada evento con la esperada. Con `docker compose` se ejecuta con
`docker compose --profile sim run --rm simulator`; sin Docker, con `go run ./cmd/simulator`.

| Partida | Escenario | Qué debe detectar Scala |
|---|---|---|
| `fps-01` | `streak` | 1 racha de 3 eliminaciones (< 30 s) |
| `fps-02` | `boundary` | **Ninguna**: 3 eliminaciones a exactamente 15 s entre sí (la 3.ª a 30 s de la 1.ª) |
| `fps-03` | `normal` | Ninguna: eliminaciones separadas ≥ 16 s |
| `fps-04` | `long-streak` | 1 racha de 5 eliminaciones |
| `race-01` | `race` | Partida de `race-x`, con `raceStats` por piloto |

Además inyecta 9 eventos inválidos (falta `matchId`, timestamp inválido, tipo y acción desconocidos,
auto-eliminación, daño negativo, sin `playerId`, duplicado y JSON mal formado).

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

## Pruebas

```bash
go vet ./...
go test -race ./...        # Go

cd scala
sbt test                   # Scala (munit)
```

- **Go:** `internal/validator` usa los ejemplos válidos e inválidos de `testdata/events_spec.json`;
  `internal/pipeline` prueba una partida completa contra un Scala simulado, duplicados, eventos tras el fin
  y 30 partidas en paralelo; `internal/simulator` pasa todo lo generado por el módulo Go real.
- **Scala:** `AnalyzerSuite` (métricas, MVP, rachas, caso límite de 30 s, carreras) y `RoutesSuite`
  (rutas HTTP: `/health`, `/analyze` válido e inválido).

## CI/CD y Docker

- **Docker:** `Dockerfile` (Go, multietapa, con imágenes `server` y `simulator`), `scala/Dockerfile`
  (Scala, multietapa: compila con JDK 21 + sbt y ejecuta con un JRE ligero) y `docker-compose.yml`
  que une ambos módulos. Go espera a que Scala esté saludable antes de iniciar.
- **CI (GitHub Actions):** `.github/workflows/ci.yml` ejecuta en cada push y pull request:
  1. `go`: `gofmt`, `go vet` y `go test -race`.
  2. `scala`: `sbt test` con JDK 21.
  3. `docker`: construye ambas imágenes, levanta el sistema, envía los eventos del simulador y
     verifica que las 5 partidas queden `analyzed`.

## Despliegue

El sistema se ejecuta **de forma local con Docker Compose** (ver *Inicio rápido*). No necesita un servidor
remoto ni ninguna licencia. [`DEPLOY.md`](DEPLOY.md) describe cómo ejecutarlo en un servidor con Docker,
por si se quisiera llevar a otro entorno.

## Estructura del repositorio

```
cmd/server/main.go            arranque del módulo Go, variables de entorno, graceful shutdown
cmd/simulator/main.go         simulador de eventos
config/event_types.json       tipos de evento y reglas de validación
internal/model                struct Event
internal/validator            validación configurable
internal/store                estado por partida (RWMutex) + log de rechazados
internal/pipeline             workers, colas, envío a Scala
internal/analysis             cliente HTTP hacia Scala
internal/api                  handlers HTTP
internal/simulator            generador de partidas y eventos inválidos
testdata/                     events_spec.json (spec de eventos), evento.json, partida.json
scala/                        módulo Scala (sbt): modelos, Analyzer, rutas http4s, pruebas
Dockerfile                    imágenes de Go (server y simulator)
scala/Dockerfile              imagen de Scala
docker-compose.yml            orquestación local de Go + Scala
.github/workflows/ci.yml      pipeline de CI
DEPLOY.md                     guía para ejecutarlo en un servidor con Docker
fake_scala.py                 Scala falso para probar Go por separado
output/                       resultados JSON de Scala (se genera al ejecutar con Docker)
```