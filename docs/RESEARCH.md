# Design context and project boundary

EQUAL is scoped around a broad development problem: applications need a usable local API when a provider is unavailable, rate-limited, paid per call, or unsafe to trigger from CI. The primary workflow produces a local server and an OpenAPI document from real application traffic. Clean-room replacements and migrations additionally benefit from differential verification, but they are not the product's only audience.

## OccupySaaS

[OccupySaaS](https://occupysaas.com/) describes a movement of laid-off engineers building clean-room, open-source, drop-in replacements from public documentation and open standards. Its stated method puts compatibility proof against public SDKs at the center of done.

[zeroauthy](https://github.com/occupysaas/zeroauthy) implements that approach with an Auth0-compatible API, storage plugins, SDK checks, and a published compatibility roadmap. [SalesCompel](https://github.com/occupysaas/salescompel) applies the movement to Salesforce.

EQUAL is independent of those projects and is not affiliated with them. It supplies a small test harness rather than another clone: record official SDK behavior, replay it offline, and verify an alternative implementation. It cannot establish clean-room provenance or legal compliance; projects still need their own policies.

## VCR and language-bound recorders

[VCR](https://github.com/vcr/vcr) popularized record-and-replay cassettes inside Ruby test suites. [go-vcr](https://github.com/dnaeon/go-vcr) and many similar libraries bring the pattern to particular languages. This is valuable when a team wants an in-process library and already shares one runtime.

EQUAL moves the same idea outside the language. Any SDK that can use a local HTTP base URL can participate, and one cassette can serve both an offline client test and a candidate-server differential test.

## Hoverfly

[Hoverfly](https://docs.hoverfly.io/) already provides capture, simulation, middleware, and service virtualization. Its [diff mode](https://docs.hoverfly.io/en/latest/pages/keyconcepts/modes/diff.html) compares live responses with stored simulations and exposes differences through an admin API.

EQUAL deliberately does not recreate that entire product. Its contribution is a single-command local development workflow: replay observed traffic on localhost, export OpenAPI 3.1, and optionally run field-level semantic verification. There is no admin API or control plane. The included benchmark compares only exact-path replay; it does not argue that EQUAL has feature parity with Hoverfly.

## WireMock and programmable stubs

[WireMock](https://wiremock.org/docs/proxying/) supports programmable stubs, proxying, record/playback, matching, and a broad JVM and standalone ecosystem. It remains appropriate for detailed stub design and fault simulation.

EQUAL starts from observed compatibility rather than a rich stub DSL. It does not attempt to replace WireMock's matcher, templating, extension, or fault-injection surface.

## API gateways and integration catalogues

Integration platforms compete partly on prebuilt connector catalogues. That model is useful for business workflows but requires continuous connector maintenance and often couples users to a hosted control plane.

EQUAL takes the opposite boundary: HTTP is the only connector. It does not know vendor-specific endpoints and cannot provide no-code integrations. Its value is proving behavior once a client already knows how to call an API.

## Resulting product boundary

1. One portable cassette represents observed requests and responses.
2. Replay provides a local API for offline development without any live fallback.
3. Export turns the same evidence into OpenAPI 3.1 for the existing API ecosystem.
4. Verify sends recorded requests to a candidate and explains differences.
5. Secret redaction and mismatches fail explicitly rather than disappearing into convenience behavior.
6. A tiny static binary and open formats make the workflow forkable and embeddable in any CI system.

This keeps EQUAL independently useful while leaving workflow orchestration, schema coverage, production virtualization, and vendor-specific integration logic to tools designed for those jobs.
