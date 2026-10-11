ThisBuild / scalaVersion := "3.3.4"
ThisBuild / version      := "0.1.0"

val http4sVersion = "0.23.30"
val circeVersion  = "0.14.10"

lazy val root = (project in file("."))
  .settings(
    name := "gamestats-scala",
    scalacOptions ++= Seq("-deprecation", "-feature", "-unchecked"),
    libraryDependencies ++= Seq(
      "org.http4s"    %% "http4s-ember-server" % http4sVersion,
      "org.http4s"    %% "http4s-dsl"          % http4sVersion,
      "org.http4s"    %% "http4s-circe"        % http4sVersion,
      "io.circe"      %% "circe-generic"       % circeVersion,
      "io.circe"      %% "circe-parser"        % circeVersion,
      "org.typelevel" %% "log4cats-slf4j"      % "2.7.0",
      "ch.qos.logback" % "logback-classic"     % "1.5.12" % Runtime,
      "org.scalameta" %% "munit"               % "1.0.2"  % Test
    ),
    Compile / mainClass := Some("gamestats.Main"),
    Compile / run / fork := true
  )
