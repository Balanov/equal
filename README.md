<div align="center">
  <img src="assets/logo.svg" alt="EQUAL logo" width="112">
  <h1>EQUAL</h1>
  <p><strong>Different implementation. Same API.</strong></p>
  <p>One 7 MB static binary. One JSONL cassette. Both sides of the API.<br>Record any SDK · Replay offline · Verify another implementation.</p>
  <p>Linux · macOS · Windows · amd64 + arm64 · MIT</p>
  <p><a href="https://balanov.pro/equal/">Landing page: balanov.pro/equal</a></p>
</div>

EQUAL is a language-neutral HTTP compatibility laboratory. Point an existing SDK at the local recorder, and it captures real requests and responses in a reviewable JSONL cassette. The same artifact then does two jobs:

1. **Client side:** replay the recorded service offline while your SDK and tests run unchanged apart from their base URL.
2. **Server side:** send the recorded requests to another implementation and report field-level compatibility differences.

No language plugin, control plane, database, runtime, account, telemetry, or connector catalogue. HTTP is the integration boundary, so a new service does not require a new EQUAL adapter.

## Why EQUAL?

- **One cassette, two proofs.** Test the consumer against a recorded provider and the replacement provider against recorded consumer traffic.
- **Fail closed.** Replay has no upstream URL and never silently falls through to the network. Misses return 404; strict-order mismatches return 409.
- **Readable artifacts.** JSONL cassettes are diffable in Git. UTF-8 stays readable; binary bodies use base64 without changing a byte on replay.
- **Secrets redacted before storage.** Built-in rules cover common credential headers, query parameters, and nested JSON fields. The live upstream response is never redacted.
- **Semantic JSON differences.** Object key order, whitespace, and equivalent JSON numbers do not create false failures. Explicit JSON Pointers can ignore volatile IDs and timestamps.
- **Ordered scenarios.** Repeated identical requests consume responses in capture order. `--strict` also enforces global request order.
- **Small and portable.** Go standard library only, one static binary, six release targets, MIT license.

## Install

### Unix installer

```sh
curl -fsSL https://raw.githubusercontent.com/Balanov/equal/main/install.sh | sh
equal --version
```

Set `EQUAL_VERSION=0.1.0` to pin a version and `EQUAL_INSTALL_DIR` to change `$HOME/.local/bin`.

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/Balanov/equal/main/install.ps1 | iex
equal --version
```

Set `$env:EQUAL_VERSION = "0.1.0"` to pin a version and `$env:EQUAL_INSTALL_DIR` to change the install directory.

### Go install and source builds

With Go 1.26+:

```sh
go install github.com/Balanov/equal/cmd/equal@latest
```

Or:

```sh
git clone https://github.com/Balanov/equal.git
cd equal
go build -trimpath -ldflags="-s -w" -o bin/equal ./cmd/equal
```

On Windows, use `bin/equal.exe`. Every release includes SHA-256 checksums. The binaries are not code-signed yet, so verify the checksum before allowing an OS warning.

## Quick start

Record a real provider through any HTTP SDK:

```sh
equal record \
  --target https://api.example.com \
  --cassette account.jsonl
```

The recorder listens on `127.0.0.1:8080`. Set the SDK's base URL to `http://127.0.0.1:8080` and run its normal workflow. For a plain HTTP example:

```sh
curl http://127.0.0.1:8080/v1/account \
  -H "Authorization: Bearer $TEST_TOKEN"
```

Stop the recorder with Ctrl+C. Inspect the cassette and **review it for sensitive data**:

```sh
equal inspect --cassette account.jsonl
```

Replay the provider offline:

```sh
equal replay --cassette account.jsonl
```

Run the same SDK workflow against `http://127.0.0.1:8080`. The original provider is not contacted. A request that is absent from the cassette fails explicitly.

Verify a clean-room implementation, migration, or rewritten service:

```sh
equal verify \
  --target http://127.0.0.1:9000 \
  --cassette account.jsonl \
  --ignore-json '/request_id' \
  --ignore-json '/users/*/created_at' \
  --header 'Authorization: Bearer candidate-test-token' \
  --json compatibility-report.json
```

Exit code 0 means every observed interaction matched under the selected rules. Exit code 3 means at least one difference was found. The JSON report identifies status, header, JSON Pointer, or binary body differences.

## Commands

### `record`

```text
equal record --target URL --cassette FILE [options]
```

| Flag | Default | Meaning |
| --- | --- | --- |
| `--listen ADDR` | `127.0.0.1:8080` | Local reverse-proxy listener |
| `--redact NAME` | built-in list | Additional header, query, or JSON field name; repeatable |
| `--no-redact` | false | Disable built-in redaction for controlled local fixtures |
| `--overwrite` | false | Replace an existing cassette |
| `--fsync` | false | fsync every recorded interaction |
| `--max-body BYTES` | `16777216` | Per-message body limit |
| `--timeout DURATION` | `30s` | Upstream request timeout |

An existing cassette is never replaced unless `--overwrite` is explicit. Entries are written as responses complete and restored into arrival order by interaction ID.

### `replay`

```text
equal replay --cassette FILE [--strict] [--match-header NAME]
```

Matching uses method, escaped path, canonical query parameters, and the request body. JSON object key order and whitespace are normalized; array order remains significant. `--match-header` adds selected request headers. Repeated identical requests consume recorded responses in order. `--strict` requires the exact global interaction sequence.

Replay never accepts a target or upstream option. It has no live passthrough mode.

### `verify`

```text
equal verify --target URL --cassette FILE [options]
```

By default, verification compares status, `Content-Type`, `Location`, and the response body. JSON is compared semantically, including exact array order and arbitrary-precision numeric equivalence. Non-JSON bodies are compared byte for byte.

| Flag | Meaning |
| --- | --- |
| `--ignore-json POINTER` | Ignore a response JSON Pointer; `*` matches one array/object segment; repeatable |
| `--compare-header NAME` | Replace the default compared response headers; repeatable |
| `--header 'Name: value'` | Inject candidate credentials or other request headers; repeatable |
| `--json FILE` | Write a machine-readable verification report |
| `--max-body BYTES` | Per-message body limit |
| `--timeout DURATION` | Candidate request timeout |

Verification replays requests in their original order and can trigger real side effects. Use disposable test tenants, never production workloads.

### `inspect`

```text
equal inspect --cassette FILE [--json]
```

Print capture provenance, interaction count, redaction settings, methods, URIs, and statuses without contacting either implementation.

## Cassette format

The first line is an `equal/v1` header. Every following line is one interaction:

```jsonl
{"format":"equal/v1","created_at":"2026-11-04T12:00:00Z","target":"https://api.example.com","redactions":["authorization","password"]}
{"id":1,"request":{"method":"GET","uri":"/v1/account","headers":{"Authorization":["[REDACTED]"]}},"response":{"status":200,"headers":{"Content-Type":["application/json"]},"body":"{\"plan\":\"free\"}","body_encoding":"utf8"}}
```

Text uses `body_encoding=utf8`; non-UTF-8 payloads use `base64`. No compression, custom database, or schema registry is required. See [the architecture and invariants](docs/ARCHITECTURE.md).

## What compatibility means

EQUAL proves compatibility only for the captured interactions and selected comparison rules. A recording does not cover unobserved endpoints, error branches, timing behavior, concurrency semantics, or future provider changes. Ignoring a field is an explicit decision, not automatic noise detection.

Version 0.1.0 handles ordinary buffered HTTP request-response traffic. It does not record WebSocket sessions, SSE streams, or SDKs that cannot address an HTTP base URL. It is a test tool, not a production proxy. See [SECURITY.md](SECURITY.md) for capture, network, and privacy boundaries.

## Measured locally

Windows 11 AMD64, Go 1.27.3, EQUAL 0.1.0 and Hoverfly 1.12.17:

| Observation | EQUAL | Hoverfly | Scenario |
| --- | ---: | ---: | --- |
| Median process launch | **9.987 ms** | 40.730 ms | 100 version launches, warm cache |
| Executable | **7,492,608 bytes** | 22,234,112 bytes | Complete standalone binaries |
| Median replay batch | **235.152 ms** | 519.807 ms | 2,000 unique GETs, 16 keep-alive clients |
| Median requests/s | **8,505** | 3,848 | Same replay scenario |
| Median working set | **14.43 MiB** | 31.64 MiB | 2,000 loaded interactions |

The replay ratio is 2.21× in this exact scenario, not a universal performance ranking. Hoverfly provides a broader simulation and matching feature set. Startup/import was excluded; fresh server state was used for every batch. Raw samples, generated equivalent fixtures, and commands are in [BENCHMARKS.md](BENCHMARKS.md) and [bench/](bench/README.md).

## End-to-end proof

```sh
go build -o bin/equal.exe ./cmd/equal
python bench/proof.py ./bin/equal.exe
```

The proof drives the real executable through record → upstream shutdown → offline replay → compatible verification → intentional incompatibility. The recorded Windows run passed **20 checks**, including secret redaction, binary fidelity, fail-closed misses, machine-readable reports, and exit code 3 on drift. It uses synthetic loopback services and needs no credentials or third-party Python packages.

## Develop

```sh
gofmt -w cmd internal
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
```

See [CONTRIBUTING.md](CONTRIBUTING.md) and [docs/RESEARCH.md](docs/RESEARCH.md). Portable cassette tooling, public-SDK compatibility fixtures, precise bug reports, and performance reproductions are welcome.

The bilingual landing source lives in `site/`:

```sh
python bench/render_site.py
cd site
npm install
npm run dev
npm run build
```

The production site is static output in `site/dist/`; asset URLs work under `/equal/`.

## Citation

```bibtex
@misc{balanov2026equal,
  title        = {EQUAL: One Cassette, Both Sides of the API},
  author       = {Balanov, Anton},
  year         = {2026},
  howpublished = {\url{https://github.com/Balanov/equal}},
  url          = {https://balanov.pro/equal/}
}
```

## License

MIT. Fork it, modify it, ship it. See [LICENSE](LICENSE).
