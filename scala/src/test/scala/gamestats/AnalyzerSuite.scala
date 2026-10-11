package gamestats

import io.circe.Json
import io.circe.parser.decode
import io.circe.syntax.*

import java.time.Instant
import scala.io.Source

class AnalyzerSuite extends munit.FunSuite:

  // ---------------------------------------------------------------- helpers

  private val t0 = Instant.parse("2026-09-01T10:00:00Z")
  private def secs(n: Long): Long = n * 1000L

  private def mk(id: String, ms: Long, player: Option[String], tpe: String, action: Option[String], data: Map[String, Json]) =
    Event(id, t0.plusMillis(ms), "fps-arena", "1.0.0", "m", player, tpe, action, data)

  private def kill(ms: Long, killer: String, victim: String) =
    mk(s"k-$killer-$ms", ms, Some(killer), "PLAYER_ELIMINATED", None,
      Map("victimId" -> Json.fromString(victim), "weapon" -> Json.fromString("rifle")))

  private def shot(ms: Long, shooter: String, hit: Boolean, damage: Int) =
    mk(s"s-$shooter-$ms", ms, Some(shooter), "PLAYER_ACTION", Some("SHOOT"),
      Map("weapon" -> Json.fromString("rifle"), "hit" -> Json.fromBoolean(hit), "damage" -> Json.fromInt(damage)))

  private def request(events: List[Event]) =
    AnalyzeRequest("m", "fps-arena", "1.0.0", "2026-09-01T10:10:00Z", complete = true, events = events)

  private def streaksOf(events: List[Event]): List[KillStreak] =
    Analyzer.analyze(request(events)).patterns.killStreaks

  private def specRequest: AnalyzeRequest =
    val raw = Source.fromResource("request-match-001.json").mkString
    decode[AnalyzeRequest](raw).fold(e => fail(s"decode failed: $e"), identity)

  // ---------------------------------------------------------------- ejemplo del contrato (events_spec.json)

  test("decodifica el request de Go (playerId null, action ausente, timestamps con milisegundos)") {
    val req = specRequest
    assertEquals(req.events.size, 9)
    assertEquals(req.events.head.playerId, None)
    assertEquals(req.events.head.action, None)
    assertEquals(req.events(3).action, Some("SHOOT"))
    assertEquals(req.complete, true)
  }

  test("el ejemplo de events_spec.json produce la respuesta esperada") {
    val res = Analyzer.analyze(specRequest)

    assertEquals(res.matchStats.durationSeconds, 60.0)
    assertEquals(res.matchStats.totalKills, 3)
    assertEquals(res.matchStats.mvpPlayerId, Some("p1"))

    val p1 = res.playerStats.find(_.playerId == "p1").get
    assertEquals((p1.kills, p1.deaths, p1.kdRatio), (3, 0, 3.0))
    assertEquals((p1.shots, p1.hits, p1.accuracy, p1.totalDamage), (2, 1, 0.5, 100.0))

    val p2 = res.playerStats.find(_.playerId == "p2").get
    assertEquals((p2.kills, p2.deaths, p2.kdRatio), (0, 3, 0.0))
    assertEquals((p2.shots, p2.hits, p2.accuracy, p2.totalDamage), (0, 0, 0.0, 0.0))
    assertEquals(res.raceStats, Nil)

    assertEquals(res.patterns.killStreaks.size, 1)
    val s = res.patterns.killStreaks.head
    assertEquals(s.playerId, "p1")
    assertEquals(s.killCount, 3)
    assertEquals(s.startTime, "2026-09-01T10:00:12.000Z")
    assertEquals(s.endTime, "2026-09-01T10:00:33.000Z")
    assertEquals(s.durationMs, 21000L)
  }

  test("el JSON de salida respeta el contrato (formato de timestamps y claves)") {
    val json = Analyzer.analyze(specRequest).asJson
    val streak = json.hcursor.downField("patterns").downField("killStreaks").downArray
    assertEquals(streak.get[String]("startTime"), Right("2026-09-01T10:00:12.000Z"))
    assertEquals(json.hcursor.downField("matchStats").get[Option[String]]("mvpPlayerId"), Right(Some("p1")))
    assert(json.hcursor.downField("playerStats").downArray.get[Double]("kdRatio").isRight)
  }

  // ---------------------------------------------------------------- KILL_STREAK

  test("3 eliminaciones a exactamente 15 s entre si NO son racha (caso limite: 30 s no es < 30 s)") {
    val evs = List(kill(secs(10), "p1", "p2"), kill(secs(25), "p1", "p2"), kill(secs(40), "p1", "p2"))
    assertEquals(streaksOf(evs), Nil)
  }

  test("3 eliminaciones en menos de 30 s son una racha") {
    val evs = List(kill(secs(10), "p1", "p2"), kill(secs(20), "p1", "p2"), kill(secs(39) + 999, "p1", "p2"))
    assertEquals(streaksOf(evs).map(_.killCount), List(3))
  }

  test("5 eliminaciones con ventanas solapadas se fusionan en una sola racha de 5") {
    val evs = List(0, 6, 12, 18, 24).map(s => kill(secs(s), "p1", "p2"))
    val streaks = streaksOf(evs)
    assertEquals(streaks.map(_.killCount), List(5))
    assertEquals(streaks.head.durationMs, 24000L)
  }

  test("dos rachas separadas del mismo jugador se reportan por separado") {
    val evs = List(0, 5, 10, 100, 105, 110).map(s => kill(secs(s), "p1", "p2"))
    assertEquals(streaksOf(evs).map(_.killCount), List(3, 3))
  }

  test("las rachas se calculan por jugador (eliminaciones de otro jugador no cuentan)") {
    val evs = List(kill(secs(0), "p1", "p3"), kill(secs(5), "p2", "p3"), kill(secs(10), "p1", "p3"),
      kill(secs(15), "p2", "p3"), kill(secs(20), "p1", "p3"))
    assertEquals(streaksOf(evs).map(s => (s.playerId, s.killCount)), List(("p1", 3)))
  }

  test("el orden de llegada no importa: los eventos se ordenan por timestamp") {
    val evs = List(0, 6, 12, 18, 24).map(s => kill(secs(s), "p1", "p2")).reverse
    assertEquals(streaksOf(evs).map(_.killCount), List(5))
  }

  // ---------------------------------------------------------------- MVP y metricas

  test("MVP: con empate en kills gana quien tiene mas dano") {
    val evs = List(kill(secs(1), "p1", "p3"), kill(secs(2), "p2", "p3"), shot(secs(3), "p2", hit = true, damage = 50))
    assertEquals(Analyzer.analyze(request(evs)).matchStats.mvpPlayerId, Some("p2"))
  }

  test("MVP: con empate total gana el playerId alfabeticamente menor") {
    val evs = List(kill(secs(1), "p2", "p3"), kill(secs(2), "p1", "p3"))
    assertEquals(Analyzer.analyze(request(evs)).matchStats.mvpPlayerId, Some("p1"))
  }

  test("MVP es None si nadie tiene eliminaciones") {
    val evs = List(shot(secs(1), "p1", hit = false, damage = 0))
    assertEquals(Analyzer.analyze(request(evs)).matchStats.mvpPlayerId, None)
  }

  test("kdRatio sin muertes es igual a las kills; precision sin disparos es 0.0") {
    val res = Analyzer.analyze(request(List(kill(secs(1), "p1", "p2"), kill(secs(2), "p1", "p2"))))
    val p1  = res.playerStats.find(_.playerId == "p1").get
    assertEquals(p1.kdRatio, 2.0)
    assertEquals(p1.accuracy, 0.0)
  }

  // ---------------------------------------------------------------- otro juego (race-x)

  test("una partida de carreras no falla: duracion calculada y metricas FPS en cero") {
    def race(id: String, ms: Long, player: Option[String], tpe: String) =
      Event(
        id,
        t0.plusMillis(ms),
        "race-x",
        "2.1.0",
        "r1",
        player,
        tpe,
        None,
        if tpe == "LAP_COMPLETED" then Map(
          "lapTimeMs" -> Json.fromInt(90000),
          "position" -> Json.fromInt(1)
        )
        else Map.empty
      )
    val evs = List(
      race("r-1", 0, None, "RACE_STARTED"),
      race("r-2", secs(90), Some("d1"), "LAP_COMPLETED"),
      race("r-3", secs(120), None, "RACE_FINISHED")
    )
    val res = Analyzer.analyze(request(evs))
    assertEquals(res.matchStats.durationSeconds, 120.0)
    assertEquals(res.matchStats.totalKills, 0)
    assertEquals(res.matchStats.mvpPlayerId, None)
    assertEquals(res.playerStats.map(_.playerId), List("d1"))
    assertEquals(res.patterns.killStreaks, Nil)
    assertEquals(res.raceStats, List(RaceStats("d1", 1, 90000, 90000.0, 1)))
  }

  test("una carrera calcula metricas por piloto") {
    def lap(id: String, ms: Long, player: String, lapTimeMs: Int, position: Int) =
      Event(
        id,
        t0.plusMillis(ms),
        "race-x",
        "2.1.0",
        "r1",
        Some(player),
        "LAP_COMPLETED",
        None,
        Map(
          "lapTimeMs" -> Json.fromInt(lapTimeMs),
          "position" -> Json.fromInt(position)
        )
      )

    val res = Analyzer.analyze(request(List(
      lap("r-1", secs(10), "d1", 90000, 2),
      lap("r-2", secs(11), "d2", 88000, 1),
      lap("r-3", secs(20), "d1", 85000, 1),
      lap("r-4", secs(21), "d2", 91000, 2),
      lap("r-5", secs(30), "d1", 87000, 2)
    )))

    assertEquals(
      res.raceStats,
      List(
        RaceStats("d1", 3, 85000, 87333.333, 2),
        RaceStats("d2", 2, 88000, 89500.0, 2)
      )
    )
  }
