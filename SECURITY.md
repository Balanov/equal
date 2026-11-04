# Security

## Capture boundary

EQUAL records requests and responses, which can contain credentials, personal data, and proprietary payloads. Built-in redaction is a safeguard, not a privacy guarantee. It covers named headers, query parameters, and JSON fields; it cannot recognize secrets in arbitrary strings, XML, binary files, URLs, or vendor-specific encodings.

Use synthetic accounts and data. Add `--redact NAME` for every service-specific secret. Inspect the complete cassette before sharing or committing it. `--no-redact` is intended only for controlled local fixtures.

The recorder refuses to overwrite an existing cassette unless `--overwrite` is explicit. Cassette and report files are created with owner-only permissions where the operating system honors them. `--fsync` asks the OS to persist every interaction; without it, an OS crash can lose buffered writes.

## Network boundary

Replay has no upstream configuration and never intentionally falls back to a live service. A miss returns HTTP 404; a strict-order mismatch returns HTTP 409. Verify deliberately sends recorded requests to the candidate target and can trigger real side effects. Use disposable test tenants and never point verification at production workloads.

The default listener is loopback-only. Exposing it on another interface can expose captured data or let other users trigger upstream requests. The reverse proxy is a test tool, not a production ingress gateway. It does not provide tenant isolation, authentication, rate limiting, or sandboxing.

## Protocol limits

Version 0.2.0 buffers request and response bodies up to the configured limit (16 MiB by default). It supports ordinary HTTP/1.x request-response traffic through a reverse proxy. It does not record WebSocket sessions, server-sent event streams, HTTP/2 prioritization, multipart streaming semantics, or traffic from SDKs that cannot use an HTTP base URL. Redirects are preserved, not followed. Verification compares status, Content-Type, Location, and the body by default; additional headers require `--compare-header`.

OpenAPI export inherits the cassette's redactions and coverage limits. It includes observed examples and inferred schemas only; it is not an authoritative provider contract. Review generated documents before publishing them just as you review cassettes.

Report vulnerabilities privately through a GitHub security advisory rather than attaching sensitive captures to a public issue.
