# Benchmarks and proofs

All drivers use Python's standard library and disposable loopback services. No API key, real tenant, model, or external Python package is needed for the functional proof.

## Functional proof

```sh
go build -o bin/equal.exe ./cmd/equal
python bench/proof.py ./bin/equal.exe
```

On Unix, use `bin/equal`. The proof starts a synthetic upstream, records three interactions through the real CLI, shuts the upstream down, replays offline, verifies a compatible candidate, and verifies an intentionally broken candidate. It checks redaction, binary bytes, query canonicalization, provenance, fail-closed misses, JSON reports, and exit codes.

Write to a specific report:

```sh
python bench/proof.py ./bin/equal.exe --output bench/results/windows-amd64-proof.json
```

## EQUAL versus Hoverfly

Download the official Hoverfly 1.12.17 release for your OS and architecture:

<https://github.com/SpectoLabs/hoverfly/releases/tag/v1.12.17>

Then run:

```sh
go build -trimpath -ldflags="-s -w" -o bin/equal.exe ./cmd/equal
python bench/compare.py \
  --equal ./bin/equal.exe \
  --hoverfly ./bench/tools/hoverfly/hoverfly.exe \
  --runs 9 --startup-runs 100 --requests 2000 --concurrency 16 \
  --output-dir bench/results
```

On Unix, adjust executable paths and the output filename prefix is selected automatically.

The script generates both fixtures from the same interaction set. It does not use EQUAL to generate Hoverfly's fixture at runtime, and it does not call a live API. Each server starts and imports its fixture before measurement. The first batch per candidate is a warmup and is excluded from results.

For a quick smoke run:

```sh
python bench/compare.py \
  --equal ./bin/equal.exe \
  --hoverfly ./bench/tools/hoverfly/hoverfly.exe \
  --runs 1 --startup-runs 5 --requests 200 --concurrency 8 \
  --output-dir bench/tmp/smoke
```

## Reading and refuting the results

- Version-launch timing is not server throughput.
- Throughput is exact-path replay only; it is not a feature-normalized comparison.
- Working set is sampled after loading 2,000 interactions and before each batch; it is not peak RSS.
- Every run uses unique paths and fresh server state to reconcile EQUAL's consumed responses with Hoverfly's stateless simulations.
- The Python client, TCP loopback, HTTP transport, matching, and response writes are included for both candidates.
- No claim here covers model quality, complex matchers, templating, capture speed, HTTPS MITM, or production traffic.

Inspect `compare.py`, modify the workload, and publish a contradicting raw report if you get one. That is the point of including the samples.
