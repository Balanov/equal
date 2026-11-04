#!/usr/bin/env python3
"""Compare EQUAL and Hoverfly startup and equivalent local HTTP replay.

Every measured replay batch uses fresh server state and unique paths. Server
startup/import is excluded; client keep-alive, request matching and response
transport are included. Candidates rotate order. No external Python packages.
"""

import argparse
import concurrent.futures
import datetime
import http.client
import json
import math
import os
import platform
import signal
import socket
import statistics
import subprocess
import sys
import tempfile
import threading
import time
from pathlib import Path


def percentile(values, fraction):
    return sorted(values)[math.ceil(fraction * len(values)) - 1]


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def stop(process):
    if process.poll() is None:
        if os.name == "nt":
            process.terminate()
        else:
            process.send_signal(signal.SIGINT)
        try:
            process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait(timeout=5)
    process._log.close()


def working_set(pid):
    try:
        if os.name == "nt":
            output = subprocess.check_output(["powershell", "-NoProfile", "-Command", f"(Get-Process -Id {pid}).WorkingSet64"], text=True, timeout=5)
            return int(output.strip())
        output = subprocess.check_output(["ps", "-o", "rss=", "-p", str(pid)], text=True, timeout=5)
        return int(output.strip()) * 1024
    except (OSError, subprocess.SubprocessError, ValueError):
        return None


def wait_ready(process, port):
    deadline = time.monotonic() + 30
    while time.monotonic() < deadline:
        if process.poll() is not None:
            raise RuntimeError(f"server exited early with {process.returncode}")
        try:
            with socket.create_connection(("127.0.0.1", port), timeout=0.2):
                return
        except OSError:
            time.sleep(0.03)
    raise RuntimeError("server readiness timeout")


def make_fixtures(root, count):
    cassette = root / "replay.jsonl"
    simulation = root / "simulation.json"
    pairs = []
    with cassette.open("w", encoding="utf-8", newline="\n") as file:
        file.write(json.dumps({"format": "equal/v1", "created_at": "2026-01-01T00:00:00Z", "target": "http://fixture.local"}) + "\n")
        for index in range(count):
            body = json.dumps({"item": index, "ok": True}, separators=(",", ":"))
            path = f"/v1/items/{index}"
            entry = {
                "id": index + 1,
                "request": {"method": "GET", "uri": path},
                "response": {"status": 200, "headers": {"Content-Type": ["application/json"]}, "body": body, "body_encoding": "utf8"},
            }
            file.write(json.dumps(entry, separators=(",", ":")) + "\n")
            pairs.append({
                "request": {"method": [{"matcher": "exact", "value": "GET"}], "path": [{"matcher": "exact", "value": path}]},
                "response": {"status": 200, "body": body, "headers": {"Content-Type": ["application/json"]}},
            })
    document = {"data": {"pairs": pairs, "globalActions": {"delays": [], "delaysLogNormal": []}}, "meta": {"schemaVersion": "v5.2"}}
    simulation.write_text(json.dumps(document, separators=(",", ":")), encoding="utf-8")
    return cassette, simulation


def start_server(name, executable, cassette, simulation, log_path):
    port = free_port()
    log = open(log_path, "ab")
    if name == "equal":
        arguments = [str(executable), "replay", "--listen", f"127.0.0.1:{port}", "--cassette", str(cassette)]
    else:
        arguments = [str(executable), "-webserver", "-pp", str(port), "-ap", str(free_port()), "-import", str(simulation), "-log-level", "error"]
    process = subprocess.Popen(arguments, stdin=subprocess.DEVNULL, stdout=log, stderr=subprocess.STDOUT)
    process._log = log
    wait_ready(process, port)
    return process, port


def worker(port, worker_index, count, concurrency, barrier):
    expected = []
    connection = http.client.HTTPConnection("127.0.0.1", port, timeout=20)
    barrier.wait()
    try:
        for index in range(worker_index, count, concurrency):
            path = f"/v1/items/{index}"
            connection.request("GET", path)
            response = connection.getresponse()
            body = response.read()
            wanted = json.dumps({"item": index, "ok": True}, separators=(",", ":")).encode()
            if response.status != 200 or body != wanted:
                raise AssertionError(f"{path}: status {response.status}, body {body[:100]!r}")
            expected.append(index)
    finally:
        connection.close()
    return len(expected)


def measure_batch(port, count, concurrency):
    barrier = threading.Barrier(concurrency)
    started = time.perf_counter_ns()
    with concurrent.futures.ThreadPoolExecutor(max_workers=concurrency) as executor:
        futures = [executor.submit(worker, port, index, count, concurrency, barrier) for index in range(concurrency)]
        completed = sum(future.result() for future in futures)
    elapsed_ms = (time.perf_counter_ns() - started) / 1_000_000
    if completed != count:
        raise AssertionError(f"completed {completed}/{count}")
    return elapsed_ms


def version(executable, flag):
    result = subprocess.run([str(executable), flag], capture_output=True, check=True, timeout=20)
    return result.stdout.decode("utf-8", "replace").strip()


def startup(candidates, runs, output):
    samples = {name: [] for name, _, _ in candidates}
    for name, executable, flag in candidates:
        for _ in range(5):
            subprocess.run([str(executable), flag], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True, timeout=20)
    for iteration in range(runs):
        ordered = candidates[iteration % len(candidates):] + candidates[:iteration % len(candidates)]
        for name, executable, flag in ordered:
            started = time.perf_counter_ns()
            subprocess.run([str(executable), flag], stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True, timeout=20)
            samples[name].append((time.perf_counter_ns() - started) / 1_000_000)
    results = []
    for name, executable, flag in candidates:
        values = samples[name]
        results.append({
            "candidate": name, "version": version(executable, flag), "executable_bytes": executable.stat().st_size,
            "runs": runs, "min_ms": round(min(values), 3), "median_ms": round(statistics.median(values), 3),
            "p95_ms": round(percentile(values, .95), 3), "max_ms": round(max(values), 3), "samples_ms": [round(x, 3) for x in values],
        })
    report = metadata("equal-startup-v1", "wall time of version subprocess; warm OS caches; interleaved candidates")
    report["results"] = results
    write(output, report)


def metadata(suite, method):
    return {
        "suite": suite, "measured_at_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
        "method": method, "platform": platform.platform(), "machine": platform.machine(),
        "processor": platform.processor(), "logical_cpus": os.cpu_count(), "python": platform.python_version(),
    }


def write(path, report):
    path.parent.mkdir(parents=True, exist_ok=True)
    rendered = json.dumps(report, indent=2, ensure_ascii=False) + "\n"
    path.write_text(rendered, encoding="utf-8")
    print(rendered, end="")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--equal", type=Path, required=True)
    parser.add_argument("--hoverfly", type=Path, required=True)
    parser.add_argument("--runs", type=int, default=9)
    parser.add_argument("--startup-runs", type=int, default=100)
    parser.add_argument("--requests", type=int, default=2000)
    parser.add_argument("--concurrency", type=int, default=16)
    parser.add_argument("--output-dir", type=Path, default=Path(__file__).parent / "results")
    args = parser.parse_args()
    equal = args.equal.resolve()
    hoverfly = args.hoverfly.resolve()
    if min(args.runs, args.startup_runs, args.requests, args.concurrency) < 1:
        parser.error("runs, requests and concurrency must be positive")
    if args.concurrency > args.requests:
        parser.error("concurrency cannot exceed requests")
    system = {"win32": "windows", "darwin": "darwin"}.get(sys.platform, "linux")
    prefix = f"{system}-{platform.machine().lower()}"
    startup([("equal", equal, "--version"), ("hoverfly", hoverfly, "-version")], args.startup_runs, args.output_dir / f"{prefix}-startup.json")
    samples = {"equal": [], "hoverfly": []}
    with tempfile.TemporaryDirectory(prefix="equal-bench-") as directory:
        root = Path(directory)
        cassette, simulation = make_fixtures(root, args.requests)
        candidates = [("equal", equal), ("hoverfly", hoverfly)]
        total = args.runs + 1
        for iteration in range(total):
            ordered = candidates[iteration % 2:] + candidates[:iteration % 2]
            for name, executable in ordered:
                process, port = start_server(name, executable, cassette, simulation, root / f"{name}-{iteration}.log")
                try:
                    memory = working_set(process.pid)
                    elapsed = measure_batch(port, args.requests, args.concurrency)
                    if iteration > 0:
                        samples[name].append({
                            "run": iteration, "wall_ms": round(elapsed, 3), "requests": args.requests,
                            "requests_per_second": round(args.requests * 1000 / elapsed, 3), "working_set_bytes": memory,
                        })
                finally:
                    stop(process)
    results = []
    for name, _ in candidates:
        values = samples[name]
        walls = [sample["wall_ms"] for sample in values]
        memories = [sample["working_set_bytes"] for sample in values if sample["working_set_bytes"] is not None]
        results.append({
            "candidate": name, "version": version(equal if name == "equal" else hoverfly, "--version" if name == "equal" else "-version"),
            "runs": args.runs, "median_ms": round(statistics.median(walls), 3),
            "p95_ms": round(percentile(walls, .95), 3), "min_ms": round(min(walls), 3), "max_ms": round(max(walls), 3),
            "median_requests_per_second": round(statistics.median([sample["requests_per_second"] for sample in values]), 3),
            "median_working_set_bytes": round(statistics.median(memories)) if memories else None,
            "samples": values,
        })
    report = metadata("equal-http-replay-v1", f"{args.requests} unique GET requests per fresh server; {args.concurrency} keep-alive clients; startup/import excluded; rotating candidate order")
    report.update({"requests_per_run": args.requests, "concurrency": args.concurrency, "results": results})
    write(args.output_dir / f"{prefix}-replay.json", report)


if __name__ == "__main__":
    main()
