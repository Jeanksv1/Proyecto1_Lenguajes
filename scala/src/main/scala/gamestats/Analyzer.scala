package gamestats

import gamestats.Event.*

import java.time.{Duration, Instant}
import scala.math.BigDecimal.RoundingMode

/** Analisis puro y funcional de una partida: sin IO, sin estado mutable, sin `var`. */
object Analyzer:

  private val StartTypes = Set("MATCH_STARTED", "RACE_STARTED")
  private val EndTypes   = Set("MATCH_FINISHED", "RACE_FINISHED")

  /** Parametros de KILL_STREAK (events_spec.json -> complexPattern.parameters). */
  val StreakMinKills: Int  = 3
  val StreakWindowMs: Long = 30000L

  def analyze(req: AnalyzeRequest): AnalysisResult =
    val events  = sortByTime(req.events)
    val players = playerStats(events)
    AnalysisResult(
      matchId = req.matchId,
      gameId = req.gameId,
      complete = req.complete,
      matchStats = matchStats(events, players),
      playerStats = players,
      patterns = Patterns(killStreaks(events)),
      raceStats = raceStats(events)
    )

  // ------------------------------------------------------------ ordenamiento

  /** Orden estable por timestamp (Go ya los envia ordenados; aqui no se confia en eso). */
  def sortByTime(events: List[Event]): List[Event] =
    events.sortBy(e => (e.timestamp.getEpochSecond, e.timestamp.getNano))

  // ------------------------------------------------------------ metricas por jugador

  def playerStats(events: List[Event]): List[PlayerStats] =
    val eliminations = events.filter(_.isElimination)
    val shoots       = events.filter(_.isShoot)

    def countBy(ids: List[String]): Map[String, Int] =
      ids.groupBy(identity).map { case (id, xs) => id -> xs.size }

    val killsBy  = countBy(eliminations.flatMap(_.playerId))
    val deathsBy = countBy(eliminations.flatMap(_.victimId))
    val shotsBy  = countBy(shoots.flatMap(_.playerId))
    val hitsBy   = countBy(shoots.filter(_.isHit).flatMap(_.playerId))
    val damageBy: Map[String, Double] =
      shoots
        .flatMap(e => e.playerId.map(_ -> e.damage))
        .groupBy(_._1)
        .map { case (id, xs) => id -> xs.map(_._2).foldLeft(0.0)(_ + _) }

    val playerIds =
      (events.flatMap(_.playerId) ++ eliminations.flatMap(_.victimId)).distinct.sorted

    playerIds.map { p =>
      val kills  = killsBy.getOrElse(p, 0)
      val deaths = deathsBy.getOrElse(p, 0)
      val shots  = shotsBy.getOrElse(p, 0)
      val hits   = hitsBy.getOrElse(p, 0)
      PlayerStats(
        playerId = p,
        kills = kills,
        deaths = deaths,
        kdRatio = if deaths == 0 then kills.toDouble else round(kills.toDouble / deaths, 2),
        shots = shots,
        hits = hits,
        accuracy = if shots == 0 then 0.0 else round(hits.toDouble / shots, 4),
        totalDamage = round(damageBy.getOrElse(p, 0.0), 2)
      )
    }

  // ------------------------------------------------------------ metricas de carreras

  def raceStats(events: List[Event]): List[RaceStats] =
    events
      .filter(_.eventType == "LAP_COMPLETED")
      .flatMap { event =>
        for
          player <- event.playerId
          lapTime <- event.data.get("lapTimeMs").flatMap(_.asNumber).map(_.toDouble.toLong)
          position <- event.data.get("position").flatMap(_.asNumber).map(_.toDouble.toInt)
        yield player -> (lapTime, position)
      }
      .groupBy(_._1)
      .toList
      .sortBy(_._1)
      .map { case (player, laps) =>
        val values = laps.map(_._2)
        val best = values.map(_._1).min
        val average = values.map(_._1).foldLeft(0L)(_ + _).toDouble / values.size
        RaceStats(
          playerId = player,
          lapsCompleted = values.size,
          bestLapMs = best,
          avgLapMs = round(average, 3),
          finalPosition = values.last._2
        )
      }

  // ------------------------------------------------------------ metricas por partida

  def matchStats(events: List[Event], players: List[PlayerStats]): MatchStats =
    val start = events.find(e => StartTypes(e.eventType)).orElse(events.headOption).map(_.timestamp)
    val end   = events.findLast(e => EndTypes(e.eventType)).orElse(events.lastOption).map(_.timestamp)
    val durationSeconds = (start, end) match
      case (Some(s), Some(e)) => round(Duration.between(s, e).toMillis / 1000.0, 3)
      case _                  => 0.0
    MatchStats(durationSeconds, events.count(_.isElimination), mvp(players))

  /** MVP: mas kills; desempates: mayor totalDamage, menos deaths, playerId alfabetico. Sin kills => None. */
  private def mvp(players: List[PlayerStats]): Option[String] =
    players
      .filter(_.kills > 0)
      .reduceOption((a, b) => if better(a, b) then a else b)
      .map(_.playerId)

  private def better(a: PlayerStats, b: PlayerStats): Boolean =
    if a.kills != b.kills then a.kills > b.kills
    else if a.totalDamage != b.totalDamage then a.totalDamage > b.totalDamage
    else if a.deaths != b.deaths then a.deaths < b.deaths
    else a.playerId <= b.playerId

  // ------------------------------------------------------------ patron: KILL_STREAK

  def killStreaks(events: List[Event]): List[KillStreak] =
    events
      .filter(_.isElimination)
      .flatMap(e => e.playerId.map(p => p -> e.timestamp))
      .groupBy(_._1)
      .toList
      .flatMap { case (player, kills) =>
        detectStreaks(player, kills.map(_._2).toVector, StreakMinKills, StreakWindowMs)
      }
      .sortBy(s => (s.startTime, s.playerId))

  /** Ventana deslizante sobre las eliminaciones de UN jugador (ya ordenadas por tiempo).
    *
    * Una ventana de `minKills` eliminaciones consecutivas cumple si t[i+minKills-1] - t[i] < windowMs.
    * Las ventanas que se solapan (comparten alguna eliminacion) se fusionan en una sola racha.
    */
  def detectStreaks(
      player: String,
      times: Vector[Instant],
      minKills: Int,
      windowMs: Long
  ): List[KillStreak] =
    val span = minKills - 1
    val qualifying: List[Int] =
      times.indices
        .filter(i => i + span < times.size && Duration.between(times(i), times(i + span)).toMillis < windowMs)
        .toList

    val ranges: List[(Int, Int)] =
      qualifying
        .foldLeft(List.empty[(Int, Int)]) {
          case ((from, to) :: rest, i) if i <= to => (from, math.max(to, i + span)) :: rest
          case (acc, i)                           => (i, i + span) :: acc
        }
        .reverse

    ranges.map { case (from, to) =>
      KillStreak(
        playerId = player,
        killCount = to - from + 1,
        startTime = Timestamps.format(times(from)),
        endTime = Timestamps.format(times(to)),
        durationMs = Duration.between(times(from), times(to)).toMillis
      )
    }

  private def round(x: Double, decimals: Int): Double =
    BigDecimal(x).setScale(decimals, RoundingMode.HALF_UP).toDouble
