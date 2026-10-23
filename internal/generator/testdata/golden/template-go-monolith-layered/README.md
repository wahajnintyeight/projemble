# sample-app

A demo

Generated from Projemble template `go-monolith-layered` (Go Monolith - Layered).

## Run

```sh
go run ./cmd/server
```

## Verify

```sh
go vet ./...
go build ./...
```

Go module: `example.com/sample-app`.



## HTTP runtime

Logger: `fmt`. Logs include a server-generated request ID; request bodies,
query strings and credentials are not logged. Use `middleware.Logger(ctx, log)`
for request-scoped application events. `fmt` writes quoted key=value fields;
`slog` writes JSON. Levels: debug, info, warn, error.

| Environment variable | Default | Valid values |
| --- | --- | --- |
| HTTP_ADDR | :8080 | host:port (port 0 picks a free port) |
| LOG_LEVEL | info | debug, info, warn, error |
| REQUEST_TIMEOUT | 10s | positive duration, at most 25s |
| SHUTDOWN_TIMEOUT | 10s | positive duration, at most 1m |
| MAX_BODY_BYTES | 1048576 | 1?16777216 bytes |

The server sets header/read/write/idle timeouts to 5s/15s/30s/60s and caps headers
at 1 MiB. SIGINT/SIGTERM stops accepting requests and drains active requests within
the shutdown budget, then closes remaining connections. Request deadlines are
cooperative: pass `r.Context()` to dependencies and stop work when it is cancelled.
Handlers must use `respond.Decode` for JSON input so chunked oversized requests
receive a 413 problem response. Bodies are streamed through `http.MaxBytesReader`.

`GET /healthz` and `GET /readyz` return 204. Add context-aware dependency checkers
to `health.Register`; failing readiness returns 503 without exposing error details.
`respond.Problem` emits RFC 9457 `application/problem+json` errors. Panic recovery
returns 500 before headers are sent, or aborts an already-started response.

Runtime packages live in `internal/config` and `internal/platform`; `cmd/server`
loads config and wires the server. TLS termination is provided by deployment.
