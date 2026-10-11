package gamestats

import io.circe.generic.semiauto.deriveEncoder
import io.circe.{Decoder, Encoder, Json}

import java.time.format.DateTimeFormatter
import java.time.{Instant, OffsetDateTime, ZoneOffset}
import scala.util.Try

// ---------------------------------------------------------------- entrada (lo que envia Go)

/** Evento con el envelope definido en events_spec.json. Los campos propios de cada juego van en `data`. */
final case class Event(
    eventId: String,
    timestamp: Instant,
    gameId: String,
    gameVersion: String,
    matchId: String,
    playerId: Option[String],
    eventType: String,
    action: Option[String],
    data: Map[String, Json]
)

object Event:
  private val instantDecoder: Decoder[Instant] =
    Decoder.decodeString.emapTry(s => Try(OffsetDateTime.parse(s).toInstant))

  given Decoder[Event] = Decoder.instance { c =>
    for
      id      <- c.get[String]("eventId")
      ts      <- c.get[Instant]("timestamp")(using instantDecoder)
      gameId  <- c.get[String]("gameId")
      version <- c.get[String]("gameVersion")
      matchId <- c.get[String]("matchId")
      player  <- c.get[Option[String]]("playerId")
      tpe     <- c.get[String]("type")
      action  <- c.get[Option[String]]("action")
      data    <- c.getOrElse[Map[String, Json]]("data")(Map.empty)
    yield Event(
      id, ts, gameId, version, matchId,
      player.filter(_.trim.nonEmpty), tpe, action.filter(_.trim.nonEmpty), data
    )
  }

  extension (e: Event)
    def isElimination: Boolean = e.eventType == "PLAYER_ELIMINATED"
    def isShoot: Boolean       = e.eventType == "PLAYER_ACTION" && e.action.contains("SHOOT")
    def victimId: Option[String] = e.data.get("victimId").flatMap(_.asString)
    def isHit: Boolean         = e.data.get("hit").flatMap(_.asBoolean).getOrElse(false)
    def damage: Double         = e.data.get("damage").flatMap(_.asNumber).map(_.toDouble).getOrElse(0.0)

/** Cuerpo de POST /analyze (ver goToScalaContract). `complete` es el campo extra que agrega Go. */
final case class AnalyzeRequest(
    matchId: String,
    gameId: String,
    gameVersion: String,
    sentAt: String,
    complete: Boolean,
    events: List[Event]
)

object AnalyzeRequest:
  given Decoder[AnalyzeRequest] = Decoder.instance { c =>
    for
      matchId  <- c.get[String]("matchId")
      gameId   <- c.get[String]("gameId")
      version  <- c.get[String]("gameVersion")
      sentAt   <- c.getOrElse[String]("sentAt")("")
      complete <- c.getOrElse[Boolean]("complete")(true)
      events   <- c.get[List[Event]]("events")
    yield AnalyzeRequest(matchId, gameId, version, sentAt, complete, events)
  }

// ---------------------------------------------------------------- salida (lo que responde Scala)

final case class MatchStats(durationSeconds: Double, totalKills: Int, mvpPlayerId: Option[String])
object MatchStats:
  given Encoder[MatchStats] = deriveEncoder

final case class PlayerStats(
    playerId: String,
    kills: Int,
    deaths: Int,
    kdRatio: Double,
    shots: Int,
    hits: Int,
    accuracy: Double,
    totalDamage: Double
)
object PlayerStats:
  given Encoder[PlayerStats] = deriveEncoder

final case class KillStreak(
    playerId: String,
    killCount: Int,
    startTime: String,
    endTime: String,
    durationMs: Long
)
object KillStreak:
  given Encoder[KillStreak] = deriveEncoder

final case class Patterns(killStreaks: List[KillStreak])
object Patterns:
  given Encoder[Patterns] = deriveEncoder

final case class AnalysisResult(
    matchId: String,
    gameId: String,
    complete: Boolean,
    matchStats: MatchStats,
    playerStats: List[PlayerStats],
    patterns: Patterns
)
object AnalysisResult:
  given Encoder[AnalysisResult] = deriveEncoder

object Timestamps:
  private val fmt = DateTimeFormatter.ofPattern("yyyy-MM-dd'T'HH:mm:ss.SSS'Z'").withZone(ZoneOffset.UTC)
  /** Formato del contrato: RFC 3339 en UTC con milisegundos (ej. 2026-09-01T10:00:12.000Z). */
  def format(i: Instant): String = fmt.format(i)
