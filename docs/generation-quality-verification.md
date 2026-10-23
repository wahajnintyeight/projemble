# Quality harness and HTTP runtime verification

Scope: Phase 0d and Phase 1 of `generation-quality-plan.md`. The HTTP runtime
applies to stdlib monolith starters in Layered, Clean, and DDD layouts. The
existing greeting slice remains; replacing it with items is Phase 2. Microservice
runtimes remain framework-owned until Phase 8.

## Resource budgets

- Generated HTTP server: 5s header, 15s read, 30s write, 60s idle timeout;
  1 MiB headers, 1 MiB default request body (configurable up to 16 MiB).
- Request context: 10s default, maximum 25s. Deadlines are cooperative; downstream
  calls must observe the context. No background goroutine is created per request
  by middleware, and request bodies are not buffered by middleware.
- Shutdown: 10s default, maximum 1 minute, followed by forced connection closure.
- Readiness checkers run sequentially within the request context. They must honor
  cancellation. No per-request fan-out or persistent request-ID map.
- Quality matrix: at most two Go commands concurrently, each with `-p=2` for
  vet/build/test, a 3-minute command deadline, and a 60-second test deadline.
  Commands inherit shared module/build caches and set `GOWORK=off`.
- Pin refresh: sequential requests, 15-second HTTP timeout, 2-minute total
  deadline, maximum 1 MiB response. Reports changes without rewriting pins.

These limits are design bounds, not evidence of production capacity. No traffic
or database-size capacity claim is made.

## Checks

`go test -short ./internal/generator -count=1` checks rendering, snapshots,
formatting, line limits and forbidden calls without compiling generated projects.
The slow tier runs tidy, tidy-diff, vet, build and tests when test files exist.
Race detection is enabled when the selected toolchain reports `CGO_ENABLED=1`.
The existing library starter exercises the tests-per-logic-package requirement;
starters without tests are not required to ship tests.

The HTTP behaviour fixtures under `internal/generator/testdata/behaviour` are
rendered into temporary projects for all three architectures and both loggers.
They check config defaults/invalid values, log levels and concurrent writes,
request IDs, context deadlines, panic recovery, known-length/chunked body limits,
JSON/problem responses, health, graceful draining, forced shutdown and listener
errors. These fixtures are embedded only in the test binary. The user-facing
test inclusion switch and test styles remain Phase 5 work.

## Reproduce on Windows

```powershell
$env:GOTOOLCHAIN = 'go1.27.3'
$env:GOCACHE = Join-Path $env:TEMP 'projemble-quality-go127-cache'
go test -short ./... -count=1
go test ./internal/generator -count=1
go vet ./internal/generator ./internal/catalog ./internal/projectstore ./tools/refreshpins
go run ./tools/refreshpins
```

The initial shared-cache run failed with misleading missing-stdlib-package errors.
The source files existed; selecting an isolated build cache resolved the failures.
The baseline comparison uses an archived copy of commit `879b83e`, with the
workspace's ignored `go.sum` copied in, and the same toolchain/module/build caches.

References: [Go server shutdown](https://pkg.go.dev/net/http#Server.Shutdown),
[Go module proxy protocol](https://go.dev/ref/mod#goproxy-protocol),
[RFC 9457 problem responses](https://www.rfc-editor.org/rfc/rfc9457.html).
