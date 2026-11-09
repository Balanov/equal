<div align="center">
  <img src="docs/readme/hero.png" alt="EQUAL — one real API capture becomes offline replay, OpenAPI 3.1 and candidate verification" width="100%">
  <p><strong>Turn real API traffic into a local sandbox and OpenAPI.</strong></p>
  <p>One 7.4 MB static binary. Any HTTP SDK. No runtime or account.</p>
  <p>Linux · macOS · Windows · amd64 + arm64 · MIT</p>
  <p>
    <a href="#install">Install</a> ·
    <a href="#start-in-90-seconds">90-second start</a> ·
    <a href="#measured-locally">Proof</a> ·
    <a href="BENCHMARKS.md">Benchmarks</a> ·
    <a href="https://balanov.pro/equal/">Website</a> ·
    <a href="README_RU.md">Русский</a>
  </p>
  <p>
    <a href="https://github.com/Balanov/equal/actions/workflows/ci.yml"><img src="https://github.com/Balanov/equal/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
    <a href="https://github.com/Balanov/equal/releases/latest"><img src="https://img.shields.io/github/v/release/Balanov/equal" alt="Latest release"></a>
    <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-green.svg" alt="MIT license"></a>
    <a href="go.mod"><img src="https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go&logoColor=white" alt="Go 1.26+"></a>
  </p>
</div>

<div align="center">
  <img src="docs/readme/demo.gif" alt="Recorded EQUAL end-to-end proof: record, redaction, OpenAPI export, offline replay, fail-closed miss and candidate verification pass in 22 checks" width="100%">
  <p><sub>Recorded end-to-end proof: <a href="bench/results/windows-amd64-proof.json">22 checks passed</a> · <a href="bench/README.md">Run the proof yourself</a>.</sub></p>
</div>

EQUAL lets you keep building when an API is unavailable, rate-limited, paid per call, or unsafe to trigger from CI.

Point your application at the local recorder and run its normal workflow once. EQUAL turns the observed requests and responses into three useful artifacts:

1. **A local API server** that returns the recorded responses without contacting the provider.
2. **A portable JSONL cassette** that you can review, edit, share, and version in Git.
3. **An OpenAPI 3.1 document** for Postman, Prism, documentation tools, and SDK generators.

Your application only needs a configurable HTTP base URL. There is no language plugin, control plane, database, account, telemetry, or connector catalogue.

## What this is useful for

- **Offline development.** Keep working on a plane, in a restricted network, or while the provider is down.
- **Deterministic CI.** Stop depending on sandbox availability, rate limits, and changing third-party data.
- **Fewer live credentials.** Record with a test credential, then run the local sandbox without the real secret.
- **One fixture across stacks.** Share the same cassette between Python, Node, Go, mobile, and CI jobs instead of maintaining a recorder per language.
- **API documentation from reality.** Export the observed endpoints and examples instead of writing the first OpenAPI draft by hand.
- **Replacements and migrations.** Optionally replay recorded requests against another implementation and inspect field-level differences.

EQUAL is a development and testing tool, not a production proxy. It proves only the traffic it recorded.

<div align="center">
  <img src="docs/readme/architecture.png" alt="EQUAL cassette boundary: one redacted capture feeds replay, OpenAPI export and candidate verification, with 22 of 22 recorded checks passing" width="100%">
  <p><sub>Replay has no upstream option or live passthrough. An unrecorded request fails closed. <a href="docs/ARCHITECTURE.md">Architecture and invariants</a>.</sub></p>
</div>

## Install

### Unix

```sh
curl -fsSL https://raw.githubusercontent.com/Balanov/equal/main/install.sh | sh
equal --version
```

Set `EQUAL_VERSION=0.2.0` to pin a version and `EQUAL_INSTALL_DIR` to change `$HOME/.local/bin`.

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/Balanov/equal/main/install.ps1 | iex
equal --version
```

Set `$env:EQUAL_VERSION = "0.2.0"` to pin a version and `$env:EQUAL_INSTALL_DIR` to change the install directory.

### Go

With Go 1.26+:

```sh
go install github.com/Balanov/equal/cmd/equal@latest
```

Or build a checkout:

```sh
git clone https://github.com/Balanov/equal.git
cd equal
go build -trimpath -ldflags="-s -w" -o bin/equal ./cmd/equal
```

On Windows, use `bin/equal.exe`. Every release includes SHA-256 checksums. The binaries are not code-signed yet.

## Start in 90 seconds

### 1. Record your application's real traffic

```sh
equal record \
  --target https://api.example.com \
  --cassette account.jsonl
```

The recorder listens on `127.0.0.1:8080`. Set your application's existing API base URL to `http://127.0.0.1:8080` and use it normally. For example:

```sh
curl http://127.0.0.1:8080/v1/account \
  -H "Authorization: Bearer $TEST_TOKEN"
```

Stop the recorder with Ctrl+C and **review the cassette for sensitive data**:

```sh
equal inspect --cassette account.jsonl
```

### 2. Run the local copy without the provider

```sh
equal replay --cassette account.jsonl
```

Run the same application against the same local base URL. The recorded responses come back without contacting the original provider. An unrecorded request returns an explicit 404 instead of silently reaching the network.

### 3. Export OpenAPI

```sh
equal export \
  --cassette account.jsonl \
  --out openapi.json \
  --title 'Account API'
```

Import `openapi.json` into your existing API tools. See the complete synthetic [cassette](examples/cassette.jsonl) and its [generated OpenAPI document](examples/openapi.json). The generated example was independently validated against the OpenAPI 3.1 schema.

## Compare another implementation

This is an additional workflow for clean-room replacements, migrations, and rewrites:

```sh
equal verify \
  --target http://127.0.0.1:9000 \
  --cassette account.jsonl \
  --ignore-json '/request_id' \
  --ignore-json '/users/*/created_at' \
  --header 'Authorization: Bearer candidate-test-token' \
  --json compatibility-report.json
```

EQUAL sends the recorded requests to the candidate and compares status, `Content-Type`, `Location`, semantic JSON, and binary bodies. Exit code 0 means every observed interaction matched under your rules; exit code 3 identifies incompatibility. Use disposable test tenants because verification can trigger real side effects.

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

The live caller receives the original response; only the stored copy is redacted. Existing cassettes require explicit `--overwrite`.

### `replay`

```text
equal replay --cassette FILE [--strict] [--match-header NAME]
```

Matching uses method, escaped path, canonical query parameters, and the request body. JSON object key order and whitespace are normalized; array order remains significant. Repeated identical requests consume responses in capture order. `--strict` enforces the global interaction sequence.

Replay has no upstream option and no live passthrough mode.

### `export`

```text
equal export --cassette FILE [--out openapi.json] [--title NAME] [--api-version VERSION]
```

Without `--out`, the OpenAPI JSON is written to stdout. Output files are created without overwriting an existing file.

The document includes observed paths, methods, query parameters, JSON request bodies, response statuses, schemas, examples, binary content encodings, and interaction IDs. Numeric and UUID path segments become documented inferred parameters. Alternative responses are retained in a provenance extension.

This is an **example-driven, incomplete specification**, not the provider's authoritative schema. Unobserved endpoints and fields are absent. JSON array item schemas currently follow the first observed item; non-JSON request bodies are not described. Redacted examples remain redacted.

### `verify`

```text
equal verify --target URL --cassette FILE [options]
```

| Flag | Meaning |
| --- | --- |
| `--ignore-json POINTER` | Ignore a response JSON Pointer; `*` matches one segment; repeatable |
| `--compare-header NAME` | Replace the default compared response headers; repeatable |
| `--header 'Name: value'` | Inject candidate credentials or request headers; repeatable |
| `--json FILE` | Write a machine-readable report |
| `--max-body BYTES` | Per-message body limit |
| `--timeout DURATION` | Candidate request timeout |

### `inspect`

```text
equal inspect --cassette FILE [--json]
```

Print capture provenance, interaction count, redaction settings, methods, URIs, and statuses without network access.

## Cassette format

The first line is an `equal/v1` header. Every following line is one interaction:

```jsonl
{"format":"equal/v1","created_at":"2026-11-04T12:00:00Z","target":"https://api.example.com","redactions":["authorization","password"]}
{"id":1,"request":{"method":"GET","uri":"/v1/account","headers":{"Authorization":["[REDACTED]"]}},"response":{"status":200,"headers":{"Content-Type":["application/json"]},"body":"{\"plan\":\"free\"}","body_encoding":"utf8"}}
```

UTF-8 remains readable; non-UTF-8 payloads use base64. See [architecture and invariants](docs/ARCHITECTURE.md) and [SECURITY.md](SECURITY.md). Built-in redaction is a safeguard, not a privacy guarantee: inspect every cassette before sharing it.

## Measured locally

Windows 11 AMD64, Go 1.27.3, EQUAL 0.2.0 and Hoverfly 1.12.17:

| Observation | EQUAL | Hoverfly | Scenario |
| --- | ---: | ---: | --- |
| Median process launch | **9.050 ms** | 39.984 ms | 100 version launches, warm cache |
| Executable | **7,776,768 bytes** | 22,234,112 bytes | Complete standalone binaries |
| Median replay batch | **269.472 ms** | 551.879 ms | 2,000 unique GETs, 16 keep-alive clients |
| Median requests/s | **7,422** | 3,624 | Same replay scenario |
| Median working set | **14.57 MiB** | 31.98 MiB | 2,000 loaded interactions |

The replay ratio is 2.05× in this exact scenario, not a universal performance ranking. Hoverfly provides a broader simulation and matching feature set. Startup/import was excluded; fresh server state was used for every batch. Raw samples and reproduction commands are in [BENCHMARKS.md](BENCHMARKS.md) and [bench/](bench/README.md).

## End-to-end proof

```sh
go build -o bin/equal.exe ./cmd/equal
python bench/proof.py ./bin/equal.exe
```

The proof drives the real executable through record → OpenAPI export → upstream shutdown → offline replay → compatible verification → intentional incompatibility. The recorded Windows run passed **22 checks**, including OpenAPI generation, secret redaction, binary fidelity, fail-closed misses, reports, and exit code 3 on drift. It uses synthetic loopback services and needs no credentials or third-party Python packages.

## Develop

```sh
gofmt -w cmd internal
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
```

See [CONTRIBUTING.md](CONTRIBUTING.md) and [docs/RESEARCH.md](docs/RESEARCH.md). Portable cassette tooling, public-SDK fixtures, precise bug reports, and performance reproductions are welcome.

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
  title        = {EQUAL: Turn Real API Traffic into a Local Sandbox and OpenAPI},
  author       = {Balanov, Anton},
  year         = {2026},
  howpublished = {\url{https://github.com/Balanov/equal}},
  url          = {https://balanov.pro/equal/}
}
```

## License

MIT. Fork it, modify it, ship it. See [LICENSE](LICENSE).
