# syntax=docker/dockerfile:1
# Modulo Go: una sola etapa de compilacion y dos imagenes finales (server y simulator).

# ---------- Etapa 1: compilar ----------
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/simulator ./cmd/simulator

# ---------- Imagen final: servidor de ingestion ----------
FROM alpine:3.20 AS server
RUN adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/server /app/server
COPY config/ /app/config/
ENV PORT=8080 \
    EVENT_TYPES_PATH=/app/config/event_types.json
USER app
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=5 \
  CMD wget -qO- http://localhost:8080/health || exit 1
ENTRYPOINT ["/app/server"]

# ---------- Imagen final: simulador de eventos ----------
FROM alpine:3.20 AS simulator
COPY --from=build /out/simulator /simulator
ENTRYPOINT ["/simulator"]
