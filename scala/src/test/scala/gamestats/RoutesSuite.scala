package gamestats

import cats.effect.IO
import cats.effect.unsafe.implicits.global
import io.circe.parser.parse
import org.http4s.*
import org.http4s.implicits.*
import org.typelevel.ci.CIStringSyntax

import scala.io.Source

class RoutesSuite extends munit.FunSuite:

  private val app = Routes.httpApp(None)
  private val requestBody = Source.fromResource("request-match-001.json").mkString

  private def run(request: Request[IO]): Response[IO] =
    app.run(request).unsafeRunSync()

  private def jsonRequest(body: String): Request[IO] =
    Request[IO](Method.POST, uri"/analyze")
      .withEntity(body)
      .putHeaders(Header.Raw(ci"Content-Type", "application/json"))

  private def responseBody(response: Response[IO]): String =
    response.as[String].unsafeRunSync()

  test("GET /health responds with ok") {
    val response = run(Request[IO](Method.GET, uri"/health"))

    assertEquals(response.status, Status.Ok)
    assertEquals(parse(responseBody(response)).toOption.flatMap(_.hcursor.get[String]("status").toOption), Some("ok"))
  }

  test("POST /analyze returns the expected analysis") {
    val response = run(jsonRequest(requestBody))

    assertEquals(response.status, Status.Ok)
    assertEquals(
      parse(responseBody(response)).toOption.flatMap(_.hcursor.downField("matchStats").get[Int]("totalKills").toOption),
      Some(3)
    )
  }

  test("POST /analyze rejects malformed JSON") {
    val response = run(jsonRequest("{"))

    assertEquals(response.status, Status.BadRequest)
    assertEquals(parse(responseBody(response)).toOption.flatMap(_.hcursor.get[String]("error").toOption), Some("INVALID_PAYLOAD"))
  }

  test("POST /analyze rejects an empty events list") {
    val response = run(jsonRequest("""{"matchId":"m","gameId":"fps-arena","gameVersion":"1.0.0","events":[]}"""))

    assertEquals(response.status, Status.BadRequest)
    assertEquals(parse(responseBody(response)).toOption.flatMap(_.hcursor.get[String]("error").toOption), Some("INVALID_PAYLOAD"))
  }

  test("POST /analyze rejects a valid payload without matchId") {
    val body = parse(requestBody).toOption.map(_.mapObject(_.remove("matchId")).noSpaces).get
    val response = run(jsonRequest(body))

    assertEquals(response.status, Status.BadRequest)
    assertEquals(parse(responseBody(response)).toOption.flatMap(_.hcursor.get[String]("error").toOption), Some("INVALID_PAYLOAD"))
  }
