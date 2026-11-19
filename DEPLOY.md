# Despliegue y operación de GameConsole en Microsoft Azure

## 1. Entorno de producción

La plataforma GameConsole se aloja en una máquina virtual de Microsoft Azure bajo la suscripción Azure for Students.

| Parámetro | Configuración |
|---|---|
| Grupo de recursos | `rg-gameconsole` |
| Máquina virtual | `vm-gameconsole` |
| Región | North Central US |
| Sistema operativo | Ubuntu Server 24.04 LTS (x64) |
| Tamaño | Standard B2als v2, 2 vCPU y 4 GiB RAM |
| Usuario SSH | `azureuser` |
| IP pública utilizada en el despliegue | `52.162.136.229` |
| Dominio | `gameconsole-jeank.duckdns.org` |
| Acceso web | `https://gameconsole-jeank.duckdns.org` |
| Orquestación | Docker Compose v2 |
| Proxy inverso y TLS | Caddy |
| Automatización | GitHub Actions |

La máquina aloja cuatro contenedores: `game-console`, `gato-ai`, `go` y `scala`. El archivo `docker-compose.prod.yml` mantiene los servicios de API y análisis sin puertos publicados externamente.

## 2. Requisitos

- Máquina Ubuntu con acceso SSH y capacidad suficiente para compilar los servicios.
- Git, Docker Engine y Docker Compose v2.
- DNS del dominio apuntando a la IP pública de la máquina virtual.
- Reglas de red de Azure para TCP 22 (administración), TCP 80 (validación HTTP y redirección) y TCP 443 (HTTPS).
- Caddy instalado en el servidor y ejecutándose como servicio.

La aplicación se expone públicamente mediante Caddy; los servicios Go, Scala y Prolog permanecen dentro de la red de Docker. El puerto 3000 se utiliza como destino interno del proxy inverso.

## 3. Preparación de Ubuntu

Actualizar los paquetes e instalar Docker y Git:

```bash
sudo apt-get update
sudo apt-get install -y git docker.io docker-compose-v2
sudo systemctl enable --now docker
docker --version
docker compose version
```

Obtener el repositorio:

```bash
cd ~
git clone https://github.com/Jeanksv1/Proyecto1_Lenguajes.git
cd Proyecto1_Lenguajes
```

## 4. Construcción y ejecución de producción

Desde la raíz del repositorio:

```bash
sudo docker compose \
  -f docker-compose.yml \
  -f docker-compose.prod.yml \
  up -d --build --wait
```

Consultar los servicios:

```bash
sudo docker compose \
  -f docker-compose.yml \
  -f docker-compose.prod.yml \
  ps
```

Los contenedores de Go y Scala incluyen comprobaciones de salud. El frontend y el servicio Prolog se inician como parte del mismo conjunto.

Para consultar registros:

```bash
sudo docker compose \
  -f docker-compose.yml \
  -f docker-compose.prod.yml \
  logs --tail=100
```

## 5. DNS, HTTPS y proxy inverso

El subdominio `gameconsole-jeank.duckdns.org` apunta a la IP pública del servidor. Caddy atiende las solicitudes HTTPS y las dirige a GameConsole, que está publicada en el puerto 3000 del anfitrión.

Archivo `/etc/caddy/Caddyfile`:

```caddyfile
gameconsole-jeank.duckdns.org {
    reverse_proxy 127.0.0.1:3000
}
```

Aplicar y verificar la configuración:

```bash
sudo caddy fmt --overwrite /etc/caddy/Caddyfile
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl enable --now caddy
sudo systemctl reload caddy
sudo systemctl is-active caddy
```

Caddy administra el certificado TLS y la redirección de HTTP a HTTPS. El frontend Nginx gestiona internamente las rutas `/api/` (Go) y `/prolog/` (Prolog).

**Acceso a la consola:** https://gameconsole-jeank.duckdns.org

## 6. Reglas de red

En el grupo de seguridad de red de Azure (`vm-gameconsole-nsg`) se habilitan las entradas necesarias:

| Puerto TCP | Servicio | Finalidad |
|---|---|---|
| 22 | SSH | Administración y conexión de despliegue de GitHub Actions |
| 80 | HTTP | Validación y redirección a HTTPS |
| 443 | HTTPS | Acceso público a GameConsole |

El puerto 3000 no necesita una regla pública de entrada, porque Caddy se comunica con la consola dentro del propio servidor. Los puertos 8080, 9090 y 5000 tampoco se exponen públicamente en la configuración de producción.

## 7. CI/CD con GitHub Actions

El proyecto utiliza dos workflows en `.github/workflows/`:

- `ci.yml`: en `push` a `main` y solicitudes de incorporación, ejecuta verificaciones de Go, compilación del frontend, pruebas de Scala y una prueba integrada de Docker con el simulador de GameStats.
- `deploy.yml`: después de que CI termina correctamente en un `push` a `main`, conecta por SSH a Azure, descarga el commit validado y actualiza los contenedores con `docker compose up -d --build --wait`.

Los secretos configurados en el repositorio son:

| Secreto | Uso |
|---|---|
| `AZURE_HOST` | Dirección pública del servidor |
| `AZURE_USER` | Usuario de acceso SSH |
| `AZURE_SSH_KEY` | Clave privada SSH usada por GitHub Actions |

El workflow valida la huella ED25519 del servidor antes de iniciar la conexión, utiliza `StrictHostKeyChecking=yes` y no almacena claves privadas en el repositorio.

## 8. Actualización manual y recuperación

La vía habitual de actualización es el despliegue automático tras una ejecución satisfactoria de CI. También es posible actualizar desde la máquina virtual:

```bash
cd ~/Proyecto1_Lenguajes
git fetch origin main
git checkout --detach origin/main
sudo docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build --wait
sudo docker compose -f docker-compose.yml -f docker-compose.prod.yml ps
```

Reiniciar los contenedores sin reconstruir:

```bash
sudo docker compose -f docker-compose.yml -f docker-compose.prod.yml restart
```

Detener los servicios:

```bash
sudo docker compose -f docker-compose.yml -f docker-compose.prod.yml down
```

Verificar Caddy y Docker:

```bash
sudo systemctl status caddy --no-pager
sudo systemctl status docker --no-pager
```

## 9. Comprobación funcional

1. Abrir `https://gameconsole-jeank.duckdns.org` y comprobar que el navegador establece una conexión HTTPS.
2. Iniciar una partida de **Memory Match** y realizar acciones sobre las cartas.
3. Iniciar una partida de **Snake Match** y controlar la serpiente.
4. Iniciar **Gato — Tic-Tac-Toe**, efectuar una jugada y observar la respuesta del adversario calculada por Prolog.
5. Abrir varias instancias y comprobar que mantienen estados independientes.
6. Consultar el monitor de eventos y el panel de GameStats durante las partidas.
7. Comprobar que se muestran partidas, eventos, estadísticas generales, estadísticas por juego y resultados de análisis.
8. Revisar en GitHub Actions el resultado de los workflows `CI` y `CD - Deploy GameConsole to Azure`.

## 10. Organización del despliegue

```text
Internet
   |
   | HTTPS :443
   v
Azure VM (Ubuntu 24.04)
   |
   +-- Caddy :80 / :443
   |       |
   |       +-- localhost:3000
   |                 |
   |                 v
   |           game-console (Nginx)
   |                 |       |
   |                 v       v
   |                go     gato-ai
   |                 |
   |                 v
   |               scala
   |
   +-- Docker Compose (red interna)
```

La combinación de contenedores, red interna, proxy inverso y automatización proporciona una instalación reproducible y una publicación web del sistema integrado.
