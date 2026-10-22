# sample-app



Generated from Projemble template `go-monolith-layered` (Go Monolith - Layered).

## Run

```sh
go run ./cmd/server
```

## Verify

```sh
go test ./...
go vet ./...
go build ./...
```

Go module: `example.com/sample-app`.

Database: SQLite. Pass its connection URL to `storage.Open` before using the generated storage adapter.
