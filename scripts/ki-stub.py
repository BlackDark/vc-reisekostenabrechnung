#!/usr/bin/env python3
"""OpenAI-compatible stub for receipt suggestions. Used by the e2e stack."""

import json
from datetime import date
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

HOST = "127.0.0.1"
PORT = 8091


def suggestion():
    return {
        "leistender": "Cafe Roma",
        "datum": date.today().isoformat(),
        "waehrung": "USD",
        "betrag_brutto": 4850,
        "steueranteile": [{"satz": 1900, "netto": 4076, "steuer": 774, "brutto": 4850}],
        "rechnungsart": "kleinbetragsrechnung",
        "kostenart": "verpflegung",
        "empfaenger_name": "Fremde GmbH",
        "rechnungsnummer": "R-1",
        "ust_id_leistender": "DE123",
        "trinkgeld": 0,
        "volltext": "Cafe Roma 48.50 USD",
        "konfidenz": {"leistender": 0.91, "betrag_brutto": 0.8},
    }


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path.startswith("/health"):
            self._send(200, {"ok": True})
            return
        self._send(404, {"error": "not found"})

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        self.rfile.read(length)
        if not self.path.rstrip("/").endswith("/chat/completions"):
            self._send(404, {"error": "not found"})
            return
        body = {
            "choices": [
                {"message": {"role": "assistant", "content": json.dumps(suggestion())}}
            ]
        }
        self._send(200, body)

    def _send(self, status, payload):
        raw = json.dumps(payload).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def log_message(self, fmt, *args):
        return


if __name__ == "__main__":
    ThreadingHTTPServer((HOST, PORT), Handler).serve_forever()
