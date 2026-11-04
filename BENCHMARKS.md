# Reproducible measurements

These are local observations, not universal rankings. Every number below comes from the raw JSON in this repository. The comparison scenario is deliberately narrow and the competitor's broader scope is part of the limitation, not a footnote to hide.

## Environment

Measured on 2026-11-04:

- Windows 11 build 28020, AMD64, AMD Family 25 Model 33, 32 logical CPUs.
- Go 1.27.3; EQUAL 0.2.0 built with `CGO_ENABLED=0`, `-trimpath`, and `-ldflags="-s -w"`.
- Hoverfly 1.12.17 official Windows amd64 release binary.
- Python 3.12.16 benchmark driver, standard library only.
- No intentional concurrent workload during the final run.

## Process launch and executable footprint

After five warmups per candidate, the driver measured 100 version subprocess launches with rotating candidate order and warm OS caches. Python subprocess overhead is included in every sample.

| Candidate | Min | Median | p95 | Max | Executable |
| --- | ---: | ---: | ---: | ---: | ---: |
| EQUAL 0.2.0 | 8.348 ms | **9.050 ms** | 10.603 ms | 12.359 ms | **7,776,768 bytes** (7.42 MiB) |
| Hoverfly 1.12.17 | 38.420 ms | **39.984 ms** | 44.161 ms | 52.390 ms | 22,234,112 bytes (21.20 MiB) |

For this version-launch workload, EQUAL's median was 4.42× lower. The complete executable was 2.86× smaller. This does not mean every server operation is 4.42× faster.

Raw data: [windows-amd64-startup.json](bench/results/windows-amd64-startup.json).

## Equivalent HTTP replay

The driver generated 2,000 unique GET interactions and two equivalent fixtures:

- an `equal/v1` JSONL cassette with exact method/path matching;
- a Hoverfly `v5.2` simulation with exact method/path matchers.

Every response was a small JSON object tied to its request path. Each fresh server processed each path exactly once through 16 keep-alive Python clients. Server startup and fixture import completed before timing began. A warmup batch preceded nine measured batches per candidate; candidate order rotated. Working set was sampled after fixture load and immediately before each measured batch.

| Candidate | Median batch | p95 | Range | Median requests/s | Median working set |
| --- | ---: | ---: | ---: | ---: | ---: |
| EQUAL 0.2.0 | **269.472 ms** | 277.140 ms | 265.355–277.140 ms | **7,421.913** | **15,282,176 bytes** (14.57 MiB) |
| Hoverfly 1.12.17 | 551.879 ms | 559.947 ms | 531.573–559.947 ms | 3,623.984 | 33,538,048 bytes (31.98 MiB) |

EQUAL's median batch time was 2.05× lower and its sampled working set was 2.19× smaller in this scenario.

EQUAL consumes a recorded response for each repeated request key; Hoverfly's ordinary simulation responses are stateless. The benchmark avoids semantic mismatch by using a fresh server and requesting every unique path exactly once. It measures exact-path replay, not complex matching, templating, recording, differential verification, proxy MITM, middleware, or fault injection. Hoverfly provides features outside EQUAL's scope and may be the right choice when those features matter.

Raw data: [windows-amd64-replay.json](bench/results/windows-amd64-replay.json). Reproduction: [bench/README.md](bench/README.md).

## End-to-end functional proof

`bench/proof.py` runs the real EQUAL executable against disposable loopback services. It needs Python 3 and no credentials or third-party packages.

The Windows run passed **22 / 22 checks**:

| Layer | Observed behavior |
| --- | --- |
| Record | Proxied 200 and 201 responses; live response bodies remained intact |
| OpenAPI export | Valid OpenAPI 3.1 document with observed endpoints and no recorded secrets |
| Binary capture | Six bytes including NUL and `0xff` arrived unchanged |
| Secret handling | Query, request JSON, authorization header, and response JSON secrets were absent from the cassette |
| Inspection | Three interactions restored from four readable JSONL lines |
| Offline replay | Canonical query order matched after upstream shutdown; replay provenance was present |
| Replay fidelity | Response secret was redacted; binary bytes were unchanged |
| Failure boundary | Unknown request returned 404 with no upstream fallback |
| Compatible candidate | 3 / 3 interactions passed under explicit volatile-ID ignore rules |
| Broken candidate | Exit code 3; JSON `body/name` and binary `body` differences were located |
| Report | Machine-readable totals and per-interaction results were written |

Raw data: [windows-amd64-proof.json](bench/results/windows-amd64-proof.json).

This proves the tested transport and control-flow behavior. It does not prove compatibility with unobserved API operations, arbitrary protocols, or production workloads.

## Local race-test boundary

The final local Windows environment had no C compiler, so `go test -race` could not be run there. Standard tests passed repeatedly, including a 100-client parallel replay test. The CI and release workflows run `go test -race` on Ubuntu. Consult the workflow result for the raced build instead of treating the local Windows run as a race-detector pass.
