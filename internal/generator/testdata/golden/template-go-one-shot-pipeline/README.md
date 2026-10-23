# sample-app

A demo

Generated from Projemble template `go-one-shot-pipeline` (Go One-shot Job - Pipeline).

## Run

```sh
go run ./cmd/job
```

## Verify

```sh
go vet ./...
go build ./...
```

Go module: `example.com/sample-app`.

## Adapt the pipeline

Use the ordered stages in `cmd/job/main.go` for your task. For a scraper, implement fetch and parse stages; for ETL, implement extract, transform, and load. Return an error from a failed stage to stop the run. The pipeline accepts a context and exits when the work is done.
