# Contributing

Fork it. Change it. Ship it. MIT means MIT.

## Project contract

EQUAL stays a small, language-neutral HTTP compatibility tool. It does not need a control plane, account, database, runtime, telemetry endpoint, or connector catalogue.

Before sending a pull request:

```sh
gofmt -w cmd internal
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go build -o equal-proof.exe ./cmd/equal
python bench/proof.py ./equal-proof.exe
```

On Windows, use `equal-proof.exe`; on Unix, use `equal-proof`.

## Changes that belong here

- Better HTTP request/response fidelity and explicit mismatch diagnostics.
- Portable cassette readers and writers in other languages.
- Additional redaction controls and tests proving their behavior.
- Reproducible compatibility fixtures from public APIs and open SDKs.
- Performance improvements that preserve the format and fail-closed behavior.

## Changes that do not belong here

- Required cloud services, telemetry, logins, or seat limits.
- Hidden passthrough during replay.
- Silently ignoring differences to make verification pass.
- Large UI frameworks or a plugin ABI for a single-purpose workflow.

Do not commit real credentials, personal data, private API captures, confidential employer material, leaked source, or copied vendor assets. Use synthetic fixtures and public documentation only. Review every cassette before publishing it.

By contributing, you agree that your contribution is licensed under the MIT License.
