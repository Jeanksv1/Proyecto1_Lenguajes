"""Servidor falso del modulo Scala, solo para probar el envio desde Go.

Uso:  python fake_scala.py
Escucha en el puerto 9090 y responde a POST /analyze con {"ok": true}.
"""
import json
from http.server import BaseHTTPRequestHandler, HTTPServer


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length).decode("utf-8")

        try:
            req = json.loads(body)
            print(
                f"[fake-scala] {self.path}: matchId={req.get('matchId')} "
                f"eventos={len(req.get('events', []))} complete={req.get('complete')}"
            )
        except json.JSONDecodeError:
            print("[fake-scala] recibi un cuerpo que no es JSON valido")

        response = json.dumps({"ok": True}).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(response)))
        self.end_headers()
        self.wfile.write(response)

    def log_message(self, format, *args):
        pass  # silencia el log por defecto de http.server


if __name__ == "__main__":
    print("[fake-scala] escuchando en http://localhost:9090 (Ctrl+C para salir)")
    HTTPServer(("", 9090), Handler).serve_forever()
