# syntax=docker/dockerfile:1
# Modulo Scala: compila con sbt + JDK 21 y ejecuta solo con un JRE 21 ligero.

# ---------- Etapa 1: compilar ----------
FROM eclipse-temurin:21-jdk AS build
ARG SBT_VERSION=1.10.5
RUN apt-get update \
 && apt-get install -y --no-install-recommends curl ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && curl -fsSL "https://github.com/sbt/sbt/releases/download/v${SBT_VERSION}/sbt-${SBT_VERSION}.tgz" | tar -xz -C /opt
ENV PATH="/opt/sbt/bin:${PATH}"
WORKDIR /build

# Primero solo la definicion del build: esta capa (descarga de dependencias) se cachea.
COPY project/ project/
COPY build.sbt ./
RUN sbt -batch update

COPY src/ src/
# "stage" deja todos los .jar (app + dependencias) en target/universal/stage/lib
RUN sbt -batch stage

# ---------- Etapa 2: ejecutar ----------
FROM eclipse-temurin:21-jre-alpine
WORKDIR /app
COPY --from=build /build/target/universal/stage/lib/ /app/lib/
ENV PORT=9090
EXPOSE 9090
HEALTHCHECK --interval=10s --timeout=3s --start-period=15s --retries=5 \
  CMD wget -qO- http://localhost:9090/health || exit 1
ENTRYPOINT ["java", "-XX:MaxRAMPercentage=75", "-cp", "/app/lib/*", "gamestats.Main"]
