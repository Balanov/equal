# Changelog

## 0.1.0 — 2026-11-04

Initial public release.

- Record ordinary HTTP SDK traffic into portable `equal/v1` JSONL cassettes.
- Redact common credential headers, query parameters, and nested JSON fields before storage.
- Replay offline with canonical request matching, ordered response queues, optional strict global order, and no upstream fallback.
- Verify another implementation with status/header comparison, semantic JSON diffs, JSON Pointer ignores, binary comparison, credential injection, JSON reports, and exit code 3 on incompatibility.
- Inspect cassette provenance and interactions in text or JSON.
- Static Linux, macOS, and Windows binaries for amd64 and arm64.
- Reproducible functional proof and local Hoverfly comparison with raw samples.
