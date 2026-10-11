package gamestats

import cats.effect.{ExitCode, IO, IOApp}
import com.comcast.ip4s.*
import org.http4s.ember.server.EmberServerBuilder
import org.typelevel.log4cats.LoggerFactory
import org.typelevel.log4cats.slf4j.Slf4jFactory

import java.nio.file.Paths

/** Modulo de analisis de GameStats (Scala). Escucha POST /analyze del modulo Go.
  *
  * Variables de entorno: PORT (default 9090), OUTPUT_DIR (opcional: guarda cada resultado como JSON).
  */
object Main extends IOApp:

  given LoggerFactory[IO] = Slf4jFactory.create[IO]

  def run(args: List[String]): IO[ExitCode] =
    val port      = sys.env.get("PORT").flatMap(Port.fromString).getOrElse(port"9090")
    val outputDir = sys.env.get("OUTPUT_DIR").filter(_.trim.nonEmpty).map(Paths.get(_))

    IO.println(s"[gamestats-scala] listening on :$port, output dir: ${outputDir.getOrElse("(disabled)")}") *>
      EmberServerBuilder
        .default[IO]
        .withHost(host"0.0.0.0")
        .withPort(port)
        .withHttpApp(Routes.httpApp(outputDir))
        .build
        .useForever
        .as(ExitCode.Success)
