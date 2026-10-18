# Despliegue (servidor remoto con Docker)

Requisitos del servidor: Ubuntu 22.04/24.04, **4 GB de RAM** (para compilar Scala con sbt),
puerto **8080** abierto. El puerto 9090 (Scala) NO debe abrirse al exterior.

## 1. Instalar Docker (una sola vez)
```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER      # cerrar sesion SSH y volver a entrar
docker compose version
```

## 2. Descargar y levantar el sistema
```bash
git clone <URL-DEL-REPO> gamestats
cd gamestats
docker compose up -d --build
docker compose ps                  # scala y go deben estar "healthy"
```

## 3. Probar desde cualquier computadora
```bash
curl http://<IP-DEL-SERVIDOR>:8080/health
go run ./cmd/simulator -url http://<IP-DEL-SERVIDOR>:8080/events
curl http://<IP-DEL-SERVIDOR>:8080/matches
```
Las 5 partidas deben quedar en `analyzed`.

## 4. Actualizar a una version nueva
```bash
cd gamestats && git pull && docker compose up -d --build
```

## 5. Apagar
```bash
docker compose down
```
