package gamestats

import cats.effect.IO
import io.circe.Json
import io.circe.syntax.*
import org.http4s.*
import org.http4s.circe.CirceEntityDecoder.*
import org.http4s.circe.CirceEntityEncoder.*
import org.http4s.dsl.io.*
import org.http4s.implicits.*

import java.nio.file.{Files, Path}

object Routes:

  /** `outputDir`: si se indica, cada resultado tambien se escribe como <matchId>.json en esa carpeta. */
  def httpApp(outputDir: Option[Path]): HttpApp[IO] =
    HttpRoutes
      .of[IO] {
        case GET -> Root / "health" =>
          Ok(Json.obj("status" -> "ok".asJson))

        case req @ POST -> Root / "analyze" =>
          req.attemptAs[AnalyzeRequest].value.flatMap {
            case Left(failure) =>
              BadRequest(error("INVALID_PAYLOAD", failure.message))
            case Right(request) =>
              validate(request) match
                case Some(msg) => BadRequest(error("INVALID_PAYLOAD", msg))
                case None =>
                  IO(Analyzer.analyze(request)).attempt.flatMap {
                    case Left(e) => InternalServerError(error("ANALYSIS_ERROR", String.valueOf(e.getMessage)))
                    case Right(result) =>
                      outputDir.fold(IO.unit)(dir => persist(dir, result)) *> Ok(result)
                  }
          }
      }
      .orNotFound

  private def validate(r: AnalyzeRequest): Option[String] =
    if r.matchId.trim.isEmpty then Some("matchId is empty")
    else if r.events.isEmpty then Some("events is empty")
    else None

  private def error(code: String, message: String): Json =
    Json.obj("error" -> code.asJson, "message" -> message.asJson)

  /** Guardar en disco nunca debe hacer fallar la respuesta HTTP. */
  private def persist(dir: Path, result: AnalysisResult): IO[Unit] =
    IO.blocking {
      Files.createDirectories(dir)
      val safeName = result.matchId.replaceAll("[^A-Za-z0-9._-]", "_")
      Files.writeString(dir.resolve(s"$safeName.json"), result.asJson.spaces2)
    }.void
      .handleErrorWith(e => IO.println(s"[gamestats-scala] could not write result: ${e.getMessage}"))
