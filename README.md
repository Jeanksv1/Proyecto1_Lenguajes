# GameConsole — Proyecto 2 de Lenguajes de Programación

**Consola web multijuego orientada a objetos, con inteligencia de juego en Prolog e integración con GameStats.**

**Autores:** Jean Carlos Segura Vargas y Leiner Josue Aguilar González  
**Curso:** Lenguajes de Programación  
**Fecha de entrega:** 20 de noviembre de 2026  
**Repositorio:** https://github.com/Jeanksv1/Proyecto1_Lenguajes  
**Aplicación desplegada:** https://gameconsole-jeank.duckdns.org

## 1. Descripción

GameConsole integra tres videojuegos en una interfaz web: **Memory Match**, **Snake Match** y **Gato (Tic-Tac-Toe)**. Los juegos se ejecutan en instancias independientes, pueden mantenerse partidas simultáneas y generan eventos que se envían a **GameStats**. El panel de la consola consulta las estadísticas procesadas por GameStats y las actualiza durante el uso de la aplicación.

El juego Gato incorpora un adversario automatizado cuya decisión se calcula mediante un servicio **SWI-Prolog** con el algoritmo **Minimax**.

## 2. Juegos y uso de la consola

| Juego | Descripción | Interacción |
|---|---|---|
| Memory Match | Juego de memoria en el que se descubren y emparejan cartas. | Iniciar partida y seleccionar cartas para encontrar parejas. |
| Snake Match | Juego de serpiente que registra movimientos y consumo de alimentos. | Iniciar partida y controlar la dirección de la serpiente. |
| Gato — Tic-Tac-Toe | Juego de tres en raya contra un adversario controlado por Prolog. | Iniciar partida, seleccionar casillas y recibir la respuesta de la IA. |

Desde la biblioteca de juegos, el usuario pulsa **Iniciar Partida**. La consola crea una instancia identificable, presenta el juego y permite gestionar su ejecución. Es posible iniciar varias partidas en la misma página y utilizar otras pestañas del navegador. El monitor de eventos y el panel de estadísticas permiten observar la actividad de las partidas.

## 3. Arquitectura y responsabilidades

```text
Navegador web
    |
    | HTTPS
    v
Caddy (proxy inverso y TLS)
    |
    v
GameConsole (TypeScript / Vite / Nginx; contenedor :80)
    |                        |
    | /api/                  | /prolog/
    v                        v
GameStats Go (:8080)       SWI-Prolog (:5000)
    |
    | análisis de partidas finalizadas
    v
GameStats Scala (:9090)
    |
    v
Resultados estadísticos JSON
    |
    +----> Consultados y presentados por GameConsole
```

- **GameConsole:** presentación, ciclo de vida de los juegos, lógica orientada a objetos, emisión de eventos y visualización de resultados.
- **Go / GameStats:** recepción HTTP, validación y deduplicación de eventos, organización por partida, procesamiento concurrente y comunicación con Scala.
- **Scala / GameStats:** análisis de eventos, cálculo de métricas y generación de resultados estadísticos.
- **SWI-Prolog:** cálculo de las jugadas de la computadora en Gato mediante Minimax.
- **Nginx:** entrega del frontend y encaminamiento de solicitudes `/api/` y `/prolog/` a los servicios internos.
- **Caddy:** entrada HTTPS pública y proxy inverso hacia la consola.

Esta separación evita duplicar el cálculo estadístico en la interfaz: GameConsole presenta los resultados que proporciona GameStats.

## 4. Diseño orientado a objetos y concurrencia

Los módulos de juego mantienen sus propias reglas, estado e interacción. La consola administra instancias independientes, de manera que una partida no comparte su estado de juego con otra. La organización en objetos separa responsabilidades como la inicialización, el procesamiento de acciones y la finalización de una partida.

Cada instancia dispone de un identificador de partida (`matchId`), que permite atribuir sus eventos al juego y a la ejecución correspondiente. Los eventos se envían a GameStats mientras se desarrollan las partidas. En Go, las partidas se distribuyen entre trabajadores concurrentes usando el identificador de partida, con colas y mecanismos de sincronización para preservar el procesamiento por instancia.

## 5. Inteligencia de juego con Prolog

**Gato — Tic-Tac-Toe** es el juego de usuario contra computadora. La consola envía al servicio Prolog el estado del tablero necesario para elegir la siguiente jugada. El componente de inteligencia evalúa las alternativas mediante **Minimax** y devuelve una decisión que se aplica al tablero. Así, la elección de la computadora se fundamenta en el análisis del estado del juego y no en una selección meramente aleatoria.

La lógica de este componente está en `prolog-ai/` y se ejecuta en su propio contenedor Docker.

## 6. Contrato de eventos e integración con GameStats

Los videojuegos producen eventos compatibles con el contrato de GameStats. El formato común identifica el evento, el momento en que ocurrió, el juego, la versión, la partida, el jugador y el tipo de acción.

| Campo | Propósito |
|---|---|
| `eventId` | Identificador único del evento. |
| `timestamp` | Marca temporal del evento. |
| `gameId` | Identificador del videojuego. |
| `gameVersion` | Versión del videojuego. |
| `matchId` | Identificador de la instancia o partida. |
| `playerId` | Jugador relacionado con el evento, cuando corresponda. |
| `type` | Tipo de evento. |
| `action` | Acción específica, cuando corresponda. |
| `data` | Información propia del evento y del juego. |

Entre los eventos del ciclo de vida se encuentran `MATCH_STARTED` y `MATCH_FINISHED`; las acciones de los jugadores se representan mediante los tipos y acciones admitidos por el contrato. Los eventos concretos y sus validaciones se definen en `config/event_types.json`.

El frontend se comunica con GameStats a través de la ruta `/api/` que sirve Nginx. Go recibe los eventos, verifica su estructura, controla duplicados y organiza los registros por `matchId`. Al finalizar una partida, solicita el análisis a Scala. La interfaz consulta las respuestas de GameStats para mostrar el historial y las estadísticas.

## 7. Panel de estadísticas

El panel integrado muestra información general y por videojuego, incluyendo partidas registradas, partidas analizadas, eventos registrados, distribución de partidas por juego, historial de partidas y resultados del análisis. También dispone de un monitor de eventos en tiempo real. La interfaz actualiza periódicamente la información consultada a GameStats mientras la consola está en uso.

## 8. Tecnologías

| Componente | Tecnología |
|---|---|
| Consola y videojuegos | TypeScript, Vite, HTML/CSS, programación orientada a objetos |
| Servidor web del frontend | Nginx |
| API e ingestión de GameStats | Go |
| Análisis estadístico | Scala |
| Inteligencia del juego Gato | SWI-Prolog, Minimax |
| Contenedores | Docker y Docker Compose |
| Automatización | GitHub Actions |
| Servidor de producción | Máquina virtual Ubuntu 24.04 LTS en Microsoft Azure |
| Acceso público | DuckDNS, Caddy y HTTPS |

## 9. Instalación y ejecución local

**Requisitos:** Git, Docker Engine o Docker Desktop y Docker Compose v2. No es necesario instalar Go, Scala, Java, Node.js ni Prolog en el equipo anfitrión cuando se utiliza Docker.

```bash
git clone https://github.com/Jeanksv1/Proyecto1_Lenguajes.git
cd Proyecto1_Lenguajes
docker compose up -d --build --wait
docker compose ps
```

Abrir **http://localhost:3000** en el navegador.

Los servicios locales del archivo `docker-compose.yml` se publican en los puertos 3000 (consola), 8080 (Go), 9090 (Scala) y 5000 (Prolog). La primera construcción puede tardar varios minutos, especialmente la imagen de Scala.

Comprobaciones de GameStats:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/matches
```

Ejecutar el simulador de integración de GameStats:

```bash
docker compose --profile sim run --rm simulator
```

Consultar el estado y detener el entorno:

```bash
docker compose logs --tail=100
docker compose down
```

## 10. Despliegue en Azure

La versión de producción se ejecuta en la máquina virtual `vm-gameconsole`, con Ubuntu 24.04 LTS, 2 vCPU y 4 GiB de RAM. Docker Compose administra los cuatro servicios: `game-console`, `gato-ai`, `go` y `scala`.

Se utiliza el archivo `docker-compose.prod.yml` como complemento de la configuración base. Esta variante no publica directamente Go, Scala ni Prolog en Internet; la consola queda disponible en el puerto 3000 del servidor para que Caddy actúe como proxy inverso.

```bash
cd ~/Proyecto1_Lenguajes
sudo docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build --wait
sudo docker compose -f docker-compose.yml -f docker-compose.prod.yml ps
```

El acceso público se realiza mediante **https://gameconsole-jeank.duckdns.org**, con certificados HTTPS gestionados por Caddy. La configuración de despliegue y operación está detallada en [DEPLOY.md](DEPLOY.md).

## 11. Integración y despliegue continuos (CI/CD)

El repositorio incorpora dos flujos de GitHub Actions:

**Integración continua — `.github/workflows/ci.yml`:** se ejecuta en cambios a `main` y en solicitudes de incorporación. Comprueba formato, análisis estático y pruebas de Go; compila el frontend TypeScript/Vite; ejecuta pruebas de Scala; construye los contenedores y verifica la integración enviando partidas del simulador a GameStats.

**Despliegue continuo — `.github/workflows/deploy.yml`:** se activa tras una ejecución satisfactoria del CI correspondiente a un `push` en `main`. Verifica la identidad SSH del servidor, se conecta a Azure, obtiene el commit aprobado, reconstruye y actualiza los contenedores con Docker Compose y muestra su estado.

Las credenciales de acceso al servidor se almacenan como secretos de GitHub Actions (`AZURE_HOST`, `AZURE_USER`, `AZURE_SSH_KEY`), sin incorporarlas al código fuente.

## 12. Pruebas y demostración funcional

La validación automática incluye:

```bash
go vet ./...
go test -race ./...
```

```bash
cd scala
sbt test
```

```bash
cd game-console
npm ci
npm run build
```

La demostración funcional del sistema comprende el inicio y desarrollo de los tres juegos, varias instancias simultáneas, la respuesta de Prolog en Gato, la emisión de eventos y la actualización del panel de GameStats. El flujo de CI/CD permite comprobar la construcción e integración antes del despliegue.

## 13. Estructura del repositorio

```text
.github/workflows/ci.yml       Pruebas y construcción continua
.github/workflows/deploy.yml   Despliegue automatizado en Azure
cmd/                           Servicios y simulador Go
config/event_types.json        Contrato y validaciones de eventos
internal/                      API, ingestión, procesamiento y almacenamiento Go
scala/                         Servicio de análisis estadístico
prolog-ai/                     Servicio de inteligencia en Prolog
game-console/                  Interfaz web y videojuegos
Dockerfile                     Construcción de Go
docker-compose.yml             Orquestación local
docker-compose.prod.yml        Configuración de producción
README.md                      Descripción, uso y arquitectura
DEPLOY.md                      Guía de despliegue y operación
```

## 14. Decisiones técnicas principales

1. **Tres juegos en una sola consola:** interfaz común y módulos independientes para mantener el estado de cada partida.
2. **POO en los videojuegos:** encapsulación del estado y las reglas de juego en componentes con responsabilidades diferenciadas.
3. **Prolog como servicio separado:** la estrategia Minimax se calcula fuera del frontend y su resultado se utiliza durante la partida.
4. **GameStats como responsable de las métricas:** Go procesa los eventos y Scala calcula estadísticas; el frontend únicamente las consulta y presenta.
5. **Identificación por partida:** los identificadores permiten separar eventos concurrentes.
6. **Docker Compose:** ejecución reproducible de los cuatro servicios.
7. **Azure con Caddy:** despliegue accesible mediante dominio y HTTPS, sin exponer directamente los servicios internos.
8. **GitHub Actions:** comprobación automática del código y actualización del servidor después de una integración exitosa.
