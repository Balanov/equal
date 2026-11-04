#!/usr/bin/env python3
"""End-to-end proof for the real EQUAL executable; standard library only."""

import argparse
import datetime
import http.client
import http.server
import json
import os
import platform
import signal
import socket
import subprocess
import sys
import tempfile
import threading
import time
from pathlib import Path

BINARY = bytes([0, 255, 1, 2, 3, 0])


class Handler(http.server.BaseHTTPRequestHandler):
    broken = False
    candidate = False

    def log_message(self, *_):
        pass

    def send(self, status, body, content_type="application/json"):
        if isinstance(body, str):
            body = body.encode()
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        if self.command != "HEAD":
            self.wfile.write(body)

    def do_GET(self):
        if self.path.startswith("/v1/items"):
            self.send(200, '{"request_id":"candidate","items":[{"name":"one","id":"candidate"}]}')
        elif self.path == "/v1/binary":
            self.send(200, BINARY if not self.broken else b"broken", "application/octet-stream")
        else:
            self.send(404, '{"error":"not found"}')

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        data = json.loads(self.rfile.read(length) or b"{}")
        if self.candidate and self.headers.get("Authorization") != "Bearer candidate":
            self.send(401, '{"error":"missing candidate credential"}')
            return
        name = "Broken" if self.broken else data.get("name", "")
        self.send(201, json.dumps({"access_token": "candidate-secret", "id": "candidate", "name": name}))


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def call(port, method, path, body=None, headers=None):
    connection = http.client.HTTPConnection("127.0.0.1", port, timeout=10)
    if isinstance(body, str):
        body = body.encode()
    connection.request(method, path, body=body, headers=headers or {})
    response = connection.getresponse()
    data = response.read()
    result = (response.status, dict(response.getheaders()), data)
    connection.close()
    return result


def wait_ready(process, port):
    deadline = time.monotonic() + 15
    while time.monotonic() < deadline:
        if process.poll() is not None:
            raise RuntimeError(f"server exited with {process.returncode}")
        try:
            with socket.create_connection(("127.0.0.1", port), timeout=0.2):
                return
        except OSError:
            time.sleep(0.03)
    raise RuntimeError("server did not start")


def start_process(arguments, log_path):
    log = open(log_path, "ab")
    process = subprocess.Popen(arguments, stdin=subprocess.DEVNULL, stdout=log, stderr=subprocess.STDOUT)
    process._equal_log = log
    return process


def stop_process(process):
    if process.poll() is None:
        if os.name == "nt":
            process.terminate()
        else:
            process.send_signal(signal.SIGINT)
        try:
            process.wait(timeout=8)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait(timeout=5)
    process._equal_log.close()


def run(arguments, expected=0):
    result = subprocess.run(arguments, capture_output=True, timeout=30)
    if result.returncode != expected:
        raise AssertionError(f"{arguments}: exit {result.returncode}, wanted {expected}\n{result.stdout!r}\n{result.stderr!r}")
    return result.stdout.decode("utf-8", "replace").strip()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("executable", type=Path)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    executable = args.executable.resolve()
    if not executable.is_file():
        parser.error(f"executable not found: {executable}")
    checks = []
    processes = []
    servers = []

    def check(name, condition, detail=""):
        if not condition:
            raise AssertionError(f"{name}: {detail}")
        checks.append({"name": name, "passed": True, "detail": detail})

    with tempfile.TemporaryDirectory(prefix="equal-proof-") as directory:
        root = Path(directory)
        cassette = root / "capture.jsonl"
        try:
            upstream = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
            servers.append(upstream)
            threading.Thread(target=upstream.serve_forever, daemon=True).start()
            upstream_port = upstream.server_address[1]
            record_port = free_port()
            record = start_process([
                str(executable), "record", "--listen", f"127.0.0.1:{record_port}",
                "--target", f"http://127.0.0.1:{upstream_port}", "--cassette", str(cassette),
            ], root / "record.log")
            processes.append(record)
            wait_ready(record, record_port)

            status, _, body = call(record_port, "GET", "/v1/items?b=2&a=1", headers={"Authorization": "Bearer original"})
            check("record proxies status", status == 200, str(status))
            check("record leaves live response intact", b"candidate" in body)
            status, _, body = call(record_port, "POST", "/v1/items?access_token=live-query-secret", '{"name":"Ada","password":"live-body-secret"}', {"Content-Type": "application/json", "Authorization": "Bearer live-header-secret"})
            check("record preserves 201", status == 201, str(status))
            check("record does not redact caller response", b"candidate-secret" in body)
            status, _, body = call(record_port, "GET", "/v1/binary")
            check("record preserves binary", status == 200 and body == BINARY)
            stop_process(record)
            processes.remove(record)
            upstream.shutdown()
            upstream.server_close()
            servers.remove(upstream)

            captured = cassette.read_text(encoding="utf-8")
            for secret in ["live-query-secret", "live-body-secret", "live-header-secret", "candidate-secret"]:
                check(f"cassette excludes {secret}", secret not in captured)
            check("cassette is readable JSONL", len(captured.splitlines()) == 4, f"{len(captured.splitlines())} lines")
            inspected = run([str(executable), "inspect", "--cassette", str(cassette)])
            check("inspect sees three interactions", "interactions: 3" in inspected)

            replay_port = free_port()
            replay = start_process([
                str(executable), "replay", "--listen", f"127.0.0.1:{replay_port}", "--cassette", str(cassette),
            ], root / "replay.log")
            processes.append(replay)
            wait_ready(replay, replay_port)
            status, headers, body = call(replay_port, "GET", "/v1/items?a=1&b=2", headers={"Authorization": "Bearer replay"})
            check("offline replay canonicalizes query order", status == 200, str(status))
            check("replay exposes provenance", headers.get("X-Equal-Replay") == "1", str(headers))
            status, _, body = call(replay_port, "POST", "/v1/items?access_token=other", '{"name":"Ada","password":"other"}', {"Content-Type": "application/json"})
            check("offline replay redacts response secret", status == 201 and b"[REDACTED]" in body and b"candidate-secret" not in body)
            status, _, body = call(replay_port, "GET", "/v1/binary")
            check("offline replay preserves binary", status == 200 and body == BINARY)
            status, _, body = call(replay_port, "GET", "/not-recorded")
            check("replay miss fails closed", status == 404 and b"no_matching_interaction" in body, str(status))
            stop_process(replay)
            processes.remove(replay)

            candidate = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
            candidate.RequestHandlerClass.candidate = True
            servers.append(candidate)
            threading.Thread(target=candidate.serve_forever, daemon=True).start()
            candidate_port = candidate.server_address[1]
            compatible_report = root / "compatible.json"
            output = run([
                str(executable), "verify", "--target", f"http://127.0.0.1:{candidate_port}", "--cassette", str(cassette),
                "--header", "Authorization: Bearer candidate", "--ignore-json", "/request_id",
                "--ignore-json", "/items/*/id", "--ignore-json", "/id", "--json", str(compatible_report),
            ])
            report = json.loads(compatible_report.read_text(encoding="utf-8"))
            check("verify accepts compatible implementation", report["passed"] == 3 and report["failed"] == 0, output)
            check("verify report is machine-readable", report["format"] == "equal/v1" and len(report["results"]) == 3)
            candidate.shutdown()
            candidate.server_close()
            servers.remove(candidate)

            broken_server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), type("BrokenHandler", (Handler,), {"broken": True, "candidate": False}))
            servers.append(broken_server)
            threading.Thread(target=broken_server.serve_forever, daemon=True).start()
            broken_report = root / "broken.json"
            output = run([
                str(executable), "verify", "--target", f"http://127.0.0.1:{broken_server.server_address[1]}", "--cassette", str(cassette),
                "--ignore-json", "/request_id", "--ignore-json", "/items/*/id", "--ignore-json", "/id", "--json", str(broken_report),
            ], expected=3)
            broken = json.loads(broken_report.read_text(encoding="utf-8"))
            fields = {difference["field"] for result in broken["results"] for difference in result.get("differences", [])}
            check("verify returns exit 3 on incompatibility", broken["failed"] == 2, output)
            check("verify locates JSON and binary drift", "body/name" in fields and "body" in fields, str(fields))
            broken_server.shutdown()
            broken_server.server_close()
            servers.remove(broken_server)
        finally:
            for process in processes:
                stop_process(process)
            for server in servers:
                server.shutdown()
                server.server_close()

    result = {
        "suite": "equal-end-to-end-proof-v1",
        "measured_at_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
        "platform": platform.platform(),
        "machine": platform.machine(),
        "python": platform.python_version(),
        "executable": str(executable),
        "executable_bytes": executable.stat().st_size,
        "checks_passed": len(checks),
        "checks": checks,
    }
    rendered = json.dumps(result, indent=2, ensure_ascii=False) + "\n"
    output = args.output
    if output is None:
        name = f"{sys.platform}-{platform.machine().lower()}-proof.json"
        output = Path(__file__).parent / "results" / name
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(rendered, encoding="utf-8")
    print(rendered, end="")


if __name__ == "__main__":
    main()
