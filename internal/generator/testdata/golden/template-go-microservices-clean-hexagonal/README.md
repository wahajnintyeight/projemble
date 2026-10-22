# sample-app

A demo

Generated from Projemble template `go-microservices-clean-hexagonal` (Go Microservices - Clean / Hexagonal).

## Run

```sh
go mod tidy

# Terminal 1
go run ./cmd/server

# Terminal 2
go run ./cmd/catalog
```

## Verify

```sh
go test ./...
go vet ./...
go build ./...
```

Go module: `example.com/sample-app`.
